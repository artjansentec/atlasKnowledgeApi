package service

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"path/filepath"
	"sort"
	"strings"

	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/internal/storage"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/jackc/pgx/v5"
)

// Tipos alinhados ao DocumentResult do Mnemos (camada Atlas).

type MnemosProjectDraft struct {
	ID          string  `json:"id,omitempty"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Client      *string `json:"client"`
}

type MnemosSectionInput struct {
	TempID       string `json:"temp_id"`
	ParentTempID string `json:"parent_temp_id"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Kind         string `json:"kind"` // doc | dev
	SortOrder    int    `json:"sort_order"`
}

type MnemosAttachmentMeta struct {
	SourceFilename string `json:"source_filename"`
	DisplayName    string `json:"display_name"`
	Kind           string `json:"kind"` // project | dev
	Relevance      string `json:"relevance,omitempty"`
}

type MnemosSyncInput struct {
	Project            MnemosProjectDraft    `json:"project"`
	Sections           []MnemosSectionInput  `json:"sections"`
	Attachments        []MnemosAttachmentMeta `json:"attachments"`
	ResponsibleUserID  string                `json:"responsibleUserId"`
	ReplaceSections    *bool                 `json:"replaceSections"`
}

type MnemosSectionResult struct {
	TempID string `json:"temp_id"`
	ID     string `json:"id"`
	Title  string `json:"title"`
	Kind   string `json:"kind"`
}

type MnemosAttachmentResult struct {
	ID             string `json:"id"`
	FileID         string `json:"fileId"`
	Name           string `json:"name"`
	SourceFilename string `json:"source_filename,omitempty"`
	Kind           string `json:"kind"`
}

type MnemosSyncResponse struct {
	Created     bool                     `json:"created"`
	ProjectID   string                   `json:"projectId"`
	Slug        string                   `json:"slug"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Status      string                   `json:"status"`
	Client      *string                  `json:"client,omitempty"`
	Sections    []MnemosSectionResult    `json:"sections"`
	Attachments []MnemosAttachmentResult `json:"attachments,omitempty"`
}

type MnemosService struct {
	cfg         *config.Config
	projects    *repository.ProjectRepository
	sections    *repository.SectionRepository
	attachments *repository.AttachmentRepository
	files       *repository.FileRepository
	users       *repository.UserRepository
	audit       *repository.AuditRepository
	storage     storage.FileStorage
	pool        pgxPool
}

func NewMnemosService(
	cfg *config.Config,
	projects *repository.ProjectRepository,
	sections *repository.SectionRepository,
	attachments *repository.AttachmentRepository,
	files *repository.FileRepository,
	users *repository.UserRepository,
	audit *repository.AuditRepository,
	store storage.FileStorage,
	pool pgxPool,
) *MnemosService {
	return &MnemosService{
		cfg: cfg, projects: projects, sections: sections,
		attachments: attachments, files: files, users: users,
		audit: audit, storage: store, pool: pool,
	}
}

// MnemosCaller descreve quem chama as rotas /mnemos/*.
// ServiceAuth=true (X-Api-Key): o ator é quem pediu no front (responsibleUserId / X-Actor-User-Id).
// ServiceAuth=false (JWT admin): Actor é o admin logado.
type MnemosCaller struct {
	ServiceAuth bool
	Actor       *domain.User
}

func (s *MnemosService) resolveCaller(ctx context.Context, caller MnemosCaller, responsibleUserID string) (domain.User, error) {
	if caller.ServiceAuth {
		id := strings.TrimSpace(responsibleUserID)
		if id == "" && caller.Actor != nil {
			id = caller.Actor.ID
		}
		if id == "" {
			return domain.User{}, httperr.Validation("responsibleUserId (ou X-Actor-User-Id) é obrigatório na integração Mnemos — use o usuário logado no front")
		}
		user, err := s.users.GetByID(ctx, id)
		if err != nil || user == nil || !user.IsActive {
			return domain.User{}, httperr.Validation("responsibleUserId inválido")
		}
		return *user, nil
	}
	if caller.Actor == nil {
		return domain.User{}, httperr.Unauthorized("usuário ausente")
	}
	if !IsAdmin(*caller.Actor) {
		return domain.User{}, httperr.Forbidden("apenas admin ou integração Mnemos podem sincronizar projetos")
	}
	return *caller.Actor, nil
}

func (s *MnemosService) Sync(ctx context.Context, caller MnemosCaller, input MnemosSyncInput) (*MnemosSyncResponse, error) {
	actor, err := s.resolveCaller(ctx, caller, input.ResponsibleUserID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(input.Project.Name)
	if name == "" {
		return nil, httperr.Validation("project.name é obrigatório")
	}
	projectID := strings.TrimSpace(input.Project.ID)
	slug := strings.TrimSpace(input.Project.Slug)
	if slug == "" {
		slug = Slugify(name)
	} else {
		slug = Slugify(slug)
	}
	if projectID == "" && slug == "" {
		return nil, httperr.Validation("project.slug é obrigatório")
	}

	status := strings.TrimSpace(input.Project.Status)
	if status == "" {
		status = string(domain.StatusActive)
	}
	meta, err := s.projects.GetStatus(ctx, status)
	if err != nil {
		return nil, httperr.Internal("falha ao validar status")
	}
	if meta == nil {
		return nil, httperr.InvalidStatus("status inválido")
	}

	responsibleID := strings.TrimSpace(input.ResponsibleUserID)
	if responsibleID == "" {
		responsibleID = actor.ID
	} else if responsibleID != actor.ID {
		u, err := s.users.GetByID(ctx, responsibleID)
		if err != nil || u == nil {
			return nil, httperr.Validation("responsibleUserId inválido")
		}
	}

	if err := s.validateSections(input.Sections); err != nil {
		return nil, err
	}

	replaceSections := true
	if input.ReplaceSections != nil {
		replaceSections = *input.ReplaceSections
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao iniciar transação")
	}
	defer tx.Rollback(ctx)

	var existing *domain.Project
	if projectID != "" {
		existing, err = s.projects.GetByID(ctx, projectID)
		if err != nil {
			return nil, httperr.Internal("falha ao buscar projeto")
		}
		if existing == nil {
			return nil, httperr.NotFound("project.id não encontrado — não será criado um projeto duplicado")
		}
		// Keep the Atlas slug when matching by id so AI slug drift cannot fork the project.
		slug = existing.Slug
	} else {
		existing, err = s.projects.GetBySlug(ctx, slug)
		if err != nil {
			return nil, httperr.Internal("falha ao buscar projeto")
		}
	}

	created := existing == nil
	var project *domain.Project

	if created {
		project = &domain.Project{
			Slug:              slug,
			Name:              name,
			Description:       input.Project.Description,
			Status:            domain.ProjectStatus(meta.Code),
			ResponsibleUserID: responsibleID,
			Client:            input.Project.Client,
		}
		if err := s.projects.Create(ctx, tx, project); err != nil {
			return nil, httperr.Validation("não foi possível criar o projeto (slug pode estar em uso)")
		}
		actorID := actor.ID
		_ = s.audit.Create(ctx, tx, &domain.AuditEvent{
			ProjectID: project.ID, ActorUserID: &actorID,
			Action: "Criou o projeto (Mnemos)", Target: name,
			EntityType: strPtr("project"), EntityID: strPtr(project.ID),
		})
	} else {
		project = existing
		project.Name = name
		if input.Project.Description != "" {
			project.Description = input.Project.Description
		}
		project.Status = domain.ProjectStatus(meta.Code)
		project.ResponsibleUserID = responsibleID
		if input.Project.Client != nil {
			project.Client = input.Project.Client
		}
		if err := s.projects.Update(ctx, tx, project); err != nil {
			return nil, httperr.Internal("falha ao atualizar projeto")
		}
		actorID := actor.ID
		_ = s.audit.Create(ctx, tx, &domain.AuditEvent{
			ProjectID: project.ID, ActorUserID: &actorID,
			Action: "Atualizou o projeto (Mnemos)", Target: name,
			EntityType: strPtr("project"), EntityID: strPtr(project.ID),
		})
	}

	sectionResults := []MnemosSectionResult{}
	if replaceSections && len(input.Sections) > 0 {
		if err := s.sections.SoftDeleteByProject(ctx, tx, project.ID); err != nil {
			return nil, httperr.Internal("falha ao substituir seções")
		}
		sectionResults, err = s.createSections(ctx, tx, project.ID, input.Sections)
		if err != nil {
			return nil, err
		}
		actorID := actor.ID
		_ = s.audit.Create(ctx, tx, &domain.AuditEvent{
			ProjectID: project.ID, ActorUserID: &actorID,
			Action: "Aplicou estrutura (Mnemos)", Target: "seções",
			EntityType: strPtr("project"), EntityID: strPtr(project.ID),
		})
	} else if created && len(input.Sections) == 0 {
		// Projeto novo sem seções Mnemos: cria visão geral padrão (mesmo padrão do Create admin).
		section := &domain.Section{
			ProjectID: project.ID,
			Title:     "Visão geral",
			Content:   input.Project.Description,
			Kind:      domain.SectionDoc,
			SortOrder: 0,
		}
		if err := s.sections.Create(ctx, tx, section); err != nil {
			return nil, httperr.Internal("falha ao criar seção inicial")
		}
		sectionResults = append(sectionResults, MnemosSectionResult{
			TempID: "", ID: section.ID, Title: section.Title, Kind: string(section.Kind),
		})
	} else if !replaceSections && len(input.Sections) > 0 {
		sectionResults, err = s.createSections(ctx, tx, project.ID, input.Sections)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, httperr.Internal("falha ao confirmar sincronização")
	}

	return &MnemosSyncResponse{
		Created:     created,
		ProjectID:   project.ID,
		Slug:        project.Slug,
		Name:        project.Name,
		Description: project.Description,
		Status:      string(project.Status),
		Client:      project.Client,
		Sections:    sectionResults,
	}, nil
}

func (s *MnemosService) PatchProject(ctx context.Context, caller MnemosCaller, slug string, draft MnemosProjectDraft, responsibleUserID string) (*MnemosSyncResponse, error) {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}
	name := strings.TrimSpace(draft.Name)
	if name == "" {
		name = project.Name
	}
	desc := draft.Description
	if strings.TrimSpace(desc) == "" {
		desc = project.Description
	}
	status := strings.TrimSpace(draft.Status)
	if status == "" {
		status = string(project.Status)
	}
	client := draft.Client
	if client == nil {
		client = project.Client
	}
	replace := false
	return s.Sync(ctx, caller, MnemosSyncInput{
		Project: MnemosProjectDraft{
			Name:        name,
			Slug:        slug,
			Description: desc,
			Status:      status,
			Client:      client,
		},
		ResponsibleUserID: firstNonEmpty(responsibleUserID, project.ResponsibleUserID),
		ReplaceSections:   &replace,
	})
}

func (s *MnemosService) ApplyStructure(ctx context.Context, caller MnemosCaller, slug string, sections []MnemosSectionInput, replace bool, responsibleUserID string) (*MnemosSyncResponse, error) {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}
	client := project.Client
	return s.Sync(ctx, caller, MnemosSyncInput{
		Project: MnemosProjectDraft{
			Name:        project.Name,
			Slug:        project.Slug,
			Description: project.Description,
			Status:      string(project.Status),
			Client:      client,
		},
		Sections:          sections,
		ResponsibleUserID: firstNonEmpty(responsibleUserID, project.ResponsibleUserID),
		ReplaceSections:   &replace,
	})
}

func (s *MnemosService) UploadAttachments(
	ctx context.Context,
	caller MnemosCaller,
	slug string,
	files []*multipart.FileHeader,
	meta []MnemosAttachmentMeta,
	actorUserID string,
) ([]MnemosAttachmentResult, error) {
	actor, err := s.resolveCaller(ctx, caller, actorUserID)
	if err != nil {
		return nil, err
	}
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}
	if len(files) == 0 {
		return nil, httperr.Validation("pelo menos um arquivo deve ser enviado")
	}

	metaByName := make(map[string]MnemosAttachmentMeta, len(meta))
	for _, m := range meta {
		key := strings.ToLower(filepath.Base(strings.TrimSpace(m.SourceFilename)))
		if key != "" {
			metaByName[key] = m
		}
	}

	results := make([]MnemosAttachmentResult, 0, len(files))
	for _, fh := range files {
		if fh.Size > s.cfg.MaxUploadBytes {
			return nil, httperr.Validation("arquivo excede o tamanho máximo permitido: " + fh.Filename)
		}
		src, err := fh.Open()
		if err != nil {
			return nil, httperr.Internal("falha ao ler arquivo")
		}

		kind := domain.AttachmentProject
		displayName := fh.Filename
		if m, ok := metaByName[strings.ToLower(filepath.Base(fh.Filename))]; ok {
			if k := normalizeAttachmentKind(m.Kind); k != "" {
				kind = k
			}
			if strings.TrimSpace(m.DisplayName) != "" {
				displayName = strings.TrimSpace(m.DisplayName)
			}
		}

		att, file, err := s.saveAttachment(ctx, actor, *project, kind, fh.Filename, displayName, fh.Size, src)
		src.Close()
		if err != nil {
			return nil, err
		}
		results = append(results, MnemosAttachmentResult{
			ID:             att.ID,
			FileID:         file.ID,
			Name:           file.OriginalName,
			SourceFilename: fh.Filename,
			Kind:           string(kind),
		})
	}
	return results, nil
}

func (s *MnemosService) saveAttachment(
	ctx context.Context,
	actor domain.User,
	project domain.Project,
	kind domain.AttachmentKind,
	originalName, displayName string,
	size int64,
	reader io.Reader,
) (*domain.Attachment, *domain.FileRecord, error) {
	ext := strings.ToLower(filepath.Ext(originalName))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		switch ext {
		case ".md", ".markdown":
			mimeType = "text/markdown"
		case ".txt":
			mimeType = "text/plain"
		default:
			mimeType = "application/octet-stream"
		}
	}
	// Integração Mnemos aceita os mesmos tipos da geração de documentação + anexos comuns.
	if !allowedMimes[mimeType] && !docAllowedMimes[mimeType] && mimeType != "application/octet-stream" {
		// permite octet-stream só para extensões conhecidas de texto/doc
		if !docAllowedExt[ext] {
			return nil, nil, httperr.Validation("tipo de arquivo não permitido: " + originalName)
		}
	}

	key, err := s.storage.Save(ctx, originalName, reader)
	if err != nil {
		return nil, nil, httperr.Internal("falha ao salvar arquivo")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, httperr.Internal("falha ao iniciar transação")
	}
	defer tx.Rollback(ctx)

	file := &domain.FileRecord{
		StorageKey:   key,
		OriginalName: originalName,
		MimeType:     mimeType,
		SizeBytes:    size,
		UploadedBy:   actor.ID,
	}
	if err := s.files.Create(ctx, tx, file); err != nil {
		return nil, nil, httperr.Internal("falha ao registrar arquivo")
	}

	dn := displayName
	attachment := &domain.Attachment{
		ProjectID: project.ID, FileID: file.ID, DisplayName: &dn, Kind: kind,
	}
	if err := s.attachments.Create(ctx, tx, attachment); err != nil {
		return nil, nil, httperr.Internal("falha ao vincular anexo")
	}

	actorID := actor.ID
	_ = s.audit.Create(ctx, tx, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Adicionou (Mnemos)", Target: originalName,
		EntityType: strPtr("attachment"), EntityID: strPtr(attachment.ID),
	})

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, httperr.Internal("falha ao confirmar anexo")
	}
	return attachment, file, nil
}

func (s *MnemosService) validateSections(sections []MnemosSectionInput) error {
	if len(sections) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(sections))
	for _, sec := range sections {
		tempID := strings.TrimSpace(sec.TempID)
		if tempID == "" {
			return httperr.Validation("sections[].temp_id é obrigatório")
		}
		if seen[tempID] {
			return httperr.Validation("sections[].temp_id duplicado: " + tempID)
		}
		seen[tempID] = true
		if strings.TrimSpace(sec.Title) == "" {
			return httperr.Validation("sections[].title é obrigatório")
		}
		kind := normalizeSectionKind(sec.Kind)
		if kind == "" {
			return httperr.Validation("sections[].kind deve ser doc ou dev")
		}
	}
	for _, sec := range sections {
		parent := strings.TrimSpace(sec.ParentTempID)
		if parent == "" {
			continue
		}
		if !seen[parent] {
			return httperr.Validation("sections[].parent_temp_id inexistente: " + parent)
		}
		if parent == strings.TrimSpace(sec.TempID) {
			return httperr.Validation("seção não pode ser pai de si mesma")
		}
	}
	return nil
}

func (s *MnemosService) createSections(ctx context.Context, tx pgx.Tx, projectID string, sections []MnemosSectionInput) ([]MnemosSectionResult, error) {
	ordered := orderSections(sections)
	idByTemp := make(map[string]string, len(ordered))
	results := make([]MnemosSectionResult, 0, len(ordered))

	for _, sec := range ordered {
		kind := normalizeSectionKind(sec.Kind)
		var parentID *string
		if parent := strings.TrimSpace(sec.ParentTempID); parent != "" {
			id, ok := idByTemp[parent]
			if !ok {
				return nil, httperr.Validation("parent_temp_id sem resolução: " + parent)
			}
			parentID = &id
		}
		section := &domain.Section{
			ProjectID: projectID,
			ParentID:  parentID,
			Title:     strings.TrimSpace(sec.Title),
			Content:   sec.Content,
			Kind:      kind,
			SortOrder: sec.SortOrder,
		}
		if err := s.sections.Create(ctx, tx, section); err != nil {
			return nil, httperr.Internal("falha ao criar seção: " + sec.Title)
		}
		tempID := strings.TrimSpace(sec.TempID)
		idByTemp[tempID] = section.ID
		results = append(results, MnemosSectionResult{
			TempID: tempID, ID: section.ID, Title: section.Title, Kind: string(section.Kind),
		})
	}
	return results, nil
}

func orderSections(sections []MnemosSectionInput) []MnemosSectionInput {
	byTemp := make(map[string]MnemosSectionInput, len(sections))
	children := make(map[string][]string)
	roots := make([]string, 0)

	for _, sec := range sections {
		temp := strings.TrimSpace(sec.TempID)
		byTemp[temp] = sec
		parent := strings.TrimSpace(sec.ParentTempID)
		if parent == "" {
			roots = append(roots, temp)
		} else {
			children[parent] = append(children[parent], temp)
		}
	}

	sort.SliceStable(roots, func(i, j int) bool {
		return byTemp[roots[i]].SortOrder < byTemp[roots[j]].SortOrder
	})
	for p := range children {
		sort.SliceStable(children[p], func(i, j int) bool {
			return byTemp[children[p][i]].SortOrder < byTemp[children[p][j]].SortOrder
		})
	}

	out := make([]MnemosSectionInput, 0, len(sections))
	var walk func(id string)
	walk = func(id string) {
		out = append(out, byTemp[id])
		for _, child := range children[id] {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	// seções órfãs (ciclo) — já validadas, mas garanta inclusão
	if len(out) < len(sections) {
		seen := make(map[string]bool, len(out))
		for _, s := range out {
			seen[strings.TrimSpace(s.TempID)] = true
		}
		for _, sec := range sections {
			if !seen[strings.TrimSpace(sec.TempID)] {
				out = append(out, sec)
			}
		}
	}
	return out
}

func normalizeSectionKind(kind string) domain.SectionKind {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "doc":
		return domain.SectionDoc
	case "dev":
		return domain.SectionDev
	default:
		return ""
	}
}

func normalizeAttachmentKind(kind string) domain.AttachmentKind {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "project":
		return domain.AttachmentProject
	case "dev":
		return domain.AttachmentDev
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
