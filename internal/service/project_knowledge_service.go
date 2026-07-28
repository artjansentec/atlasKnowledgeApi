package service

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/mapper"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/internal/storage"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

// ProjectKnowledgeService assembles the full textual corpus of a project for Mnemos.
type ProjectKnowledgeService struct {
	projects *repository.ProjectRepository
	sections *repository.SectionRepository
	lessons  *repository.LessonRepository
	atts     *repository.AttachmentRepository
	files    *repository.FileRepository
	tags     *repository.TagRepository
	users    *repository.UserRepository
	docs     *repository.DocumentationRepository
	store    storage.FileStorage
}

func NewProjectKnowledgeService(
	projects *repository.ProjectRepository,
	sections *repository.SectionRepository,
	lessons *repository.LessonRepository,
	atts *repository.AttachmentRepository,
	files *repository.FileRepository,
	tags *repository.TagRepository,
	users *repository.UserRepository,
	docs *repository.DocumentationRepository,
	store storage.FileStorage,
) *ProjectKnowledgeService {
	return &ProjectKnowledgeService{
		projects: projects, sections: sections, lessons: lessons,
		atts: atts, files: files, tags: tags, users: users, docs: docs, store: store,
	}
}

func (s *ProjectKnowledgeService) ListProjectIDs(ctx context.Context, page, pageSize int) (*mapper.ProjectIDPage, error) {
	ids, total, err := s.projects.ListIDs(ctx, page, pageSize)
	if err != nil {
		return nil, httperr.Internal("falha ao listar IDs de projetos")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return &mapper.ProjectIDPage{
		Items:    ids,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *ProjectKnowledgeService) GetByID(ctx context.Context, projectID string) (*mapper.ProjectKnowledge, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httperr.Internal("falha ao carregar projeto")
	}
	if project == nil || project.DeletedAt != nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}

	out := &mapper.ProjectKnowledge{
		ID:                project.ID,
		Slug:              project.Slug,
		Name:              project.Name,
		Description:       project.Description,
		Status:            string(project.Status),
		Client:            project.Client,
		ResponsibleUserID: project.ResponsibleUserID,
		Tags:              []string{},
		Technologies:      []string{},
		WikiSections:      []mapper.KnowledgeSection{},
		DevSections:       []mapper.KnowledgeSection{},
		Lessons:           []mapper.KnowledgeLesson{},
		Attachments:       []mapper.KnowledgeAttachment{},
		UpdatedAt:         project.UpdatedAt,
	}

	if u, err := s.users.GetByID(ctx, project.ResponsibleUserID); err == nil && u != nil {
		out.ResponsibleName = u.Name
	}
	if tags, err := s.tags.ListProjectTagNames(ctx, project.ID, domain.TagGeneral); err == nil && tags != nil {
		out.Tags = tags
	}
	if tech, err := s.tags.ListProjectTagNames(ctx, project.ID, domain.TagTech); err == nil && tech != nil {
		out.Technologies = tech
	}

	if wiki, err := s.sections.ListByProject(ctx, project.ID, domain.SectionDoc); err == nil {
		out.WikiSections = mapKnowledgeSections(wiki)
	}
	if dev, err := s.sections.ListByProject(ctx, project.ID, domain.SectionDev); err == nil {
		out.DevSections = mapKnowledgeSections(dev)
	}

	lessonTags, _ := s.tags.ListLessonTagsByProject(ctx, project.ID)
	if lessons, err := s.lessons.ListByProject(ctx, project.ID); err == nil {
		for _, l := range lessons {
			tags := lessonTags[l.ID]
			if tags == nil {
				tags = []string{}
			}
			out.Lessons = append(out.Lessons, mapper.KnowledgeLesson{
				ID:             l.ID,
				Type:           string(l.Type),
				Title:          l.Title,
				Description:    l.Description,
				Recommendation: l.Recommendation,
				Tags:           tags,
			})
		}
	}

	out.Attachments = append(out.Attachments, s.loadAttachments(ctx, project.ID, domain.AttachmentProject)...)
	out.Attachments = append(out.Attachments, s.loadAttachments(ctx, project.ID, domain.AttachmentDev)...)

	if ver, err := s.docs.GetLatestVersion(ctx, project.ID); err == nil && ver != nil {
		out.LatestDocumentation = &mapper.KnowledgeDocumentation{
			ID:            ver.ID,
			VersionNumber: ver.VersionNumber,
			ModelUsed:     ver.ModelUsed,
			Language:      ver.Language,
			ContentText:   stringifyDocContent(ver.Content),
		}
	}

	return out, nil
}

func mapKnowledgeSections(sections []domain.Section) []mapper.KnowledgeSection {
	out := make([]mapper.KnowledgeSection, 0, len(sections))
	for _, sec := range sections {
		out = append(out, mapper.KnowledgeSection{
			ID:       sec.ID,
			ParentID: sec.ParentID,
			Title:    sec.Title,
			Content:  sec.Content,
			Kind:     string(sec.Kind),
		})
	}
	return out
}

func (s *ProjectKnowledgeService) loadAttachments(ctx context.Context, projectID string, kind domain.AttachmentKind) []mapper.KnowledgeAttachment {
	atts, err := s.atts.ListByProject(ctx, projectID, kind)
	if err != nil {
		return nil
	}
	out := make([]mapper.KnowledgeAttachment, 0, len(atts))
	for _, a := range atts {
		item := mapper.KnowledgeAttachment{
			ID:   a.ID,
			Kind: string(a.Kind),
		}
		if a.DisplayName != nil {
			item.DisplayName = *a.DisplayName
		}
		file, err := s.files.GetByID(ctx, a.FileID)
		if err == nil && file != nil {
			item.OriginalName = file.OriginalName
			item.MimeType = file.MimeType
			if item.DisplayName == "" {
				item.DisplayName = file.OriginalName
			}
			if text := extractPlainText(ctx, s.store, file); text != "" {
				item.ExtractedText = &text
			}
		}
		out = append(out, item)
	}
	return out
}

func extractPlainText(ctx context.Context, store storage.FileStorage, file *domain.FileRecord) string {
	if store == nil || file == nil {
		return ""
	}
	mime := strings.ToLower(file.MimeType)
	name := strings.ToLower(file.OriginalName)
	ok := strings.HasPrefix(mime, "text/") ||
		strings.HasSuffix(name, ".txt") ||
		strings.HasSuffix(name, ".md") ||
		strings.HasSuffix(name, ".markdown") ||
		strings.HasSuffix(name, ".csv") ||
		mime == "application/json"
	if !ok {
		return ""
	}
	rc, err := store.Open(ctx, file.StorageKey)
	if err != nil {
		return ""
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, 2<<20))
	if err != nil {
		return ""
	}
	if !utf8.Valid(raw) {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func stringifyDocContent(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	if json.Valid(content) {
		var v any
		if err := json.Unmarshal(content, &v); err == nil {
			if b, err := json.MarshalIndent(v, "", "  "); err == nil {
				return string(b)
			}
		}
	}
	return string(content)
}
