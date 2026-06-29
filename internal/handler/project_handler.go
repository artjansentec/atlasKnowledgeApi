package handler

import (
	"context"
	"net/http"

	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/mapper"
	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type ProjectHandler struct {
	cfg      *config.Config
	projects *service.ProjectService
	users    *repository.UserRepository
	tags     *repository.TagRepository
	files    *repository.FileRepository
}

func NewProjectHandler(
	cfg *config.Config,
	projects *service.ProjectService,
	users *repository.UserRepository,
	tags *repository.TagRepository,
	files *repository.FileRepository,
) *ProjectHandler {
	return &ProjectHandler{cfg: cfg, projects: projects, users: users, tags: tags, files: files}
}

func (h *ProjectHandler) List(c echo.Context) error {
	user := middleware.GetUser(c)
	filter := domain.ProjectListFilter{
		Status:      c.QueryParam("status"),
		Query:       c.QueryParam("q"),
		Responsible: c.QueryParam("responsible"),
	}
	projects, err := h.projects.List(c.Request().Context(), user, filter)
	if err != nil {
		return Error(c, err)
	}

	items := make([]mapper.ProjectListItem, 0, len(projects))
	for _, p := range projects {
		item, err := h.buildListItem(c.Request().Context(), p)
		if err != nil {
			return Error(c, err)
		}
		items = append(items, item)
	}
	return JSON(c, http.StatusOK, items)
}

func (h *ProjectHandler) Get(c echo.Context) error {
	user := middleware.GetUser(c)
	project, _, err := h.projects.GetBySlug(c.Request().Context(), user, c.Param("slug"))
	if err != nil {
		return Error(c, err)
	}
	resp, err := h.buildDetail(c.Request().Context(), *project)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, resp)
}

type createProjectRequest struct {
	Name              string   `json:"name"`
	Slug              string   `json:"slug"`
	Description       string   `json:"description"`
	Status            string   `json:"status"`
	ResponsibleUserID string   `json:"responsibleUserId"`
	Client            *string  `json:"client"`
	Tags              []string `json:"tags"`
	Tech              []string `json:"tech"`
	SectionTitle      string   `json:"sectionTitle"`
	SectionContent    string   `json:"sectionContent"`
}

func (h *ProjectHandler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req createProjectRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	project, err := h.projects.Create(c.Request().Context(), user, service.CreateProjectInput{
		Name: req.Name, Slug: req.Slug, Description: req.Description, Status: req.Status,
		ResponsibleUserID: req.ResponsibleUserID, Client: req.Client,
		Tags: req.Tags, Tech: req.Tech, SectionTitle: req.SectionTitle, SectionContent: req.SectionContent,
	})
	if err != nil {
		return Error(c, err)
	}
	resp, err := h.buildDetail(c.Request().Context(), *project)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusCreated, resp)
}

type patchProjectRequest struct {
	Name              *string  `json:"name"`
	Description       *string  `json:"description"`
	Status            *string  `json:"status"`
	ResponsibleUserID *string  `json:"responsibleUserId"`
	Client            *string  `json:"client"`
	Tags              []string `json:"tags"`
	Tech              []string `json:"tech"`
}

func (h *ProjectHandler) Patch(c echo.Context) error {
	user := middleware.GetUser(c)
	var req patchProjectRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	body := map[string]interface{}{}
	_ = c.Bind(&body)
	_, hasTags := body["tags"]
	_, hasTech := body["tech"]
	_, hasClient := body["client"]

	project, err := h.projects.Patch(c.Request().Context(), user, c.Param("slug"), service.PatchProjectInput{
		Name: req.Name, Description: req.Description, Status: req.Status,
		ResponsibleUserID: req.ResponsibleUserID, Client: req.Client, HasClient: hasClient,
		Tags: req.Tags, Tech: req.Tech, HasTags: hasTags, HasTech: hasTech,
	})
	if err != nil {
		return Error(c, err)
	}
	resp, err := h.buildDetail(c.Request().Context(), *project)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, resp)
}

func (h *ProjectHandler) Delete(c echo.Context) error {
	user := middleware.GetUser(c)
	if err := h.projects.Delete(c.Request().Context(), user, c.Param("slug")); err != nil {
		return Error(c, err)
	}
	return NoContent(c)
}

type setReadersRequest struct {
	UserIDs []string `json:"userIds"`
}

func (h *ProjectHandler) SetReaders(c echo.Context) error {
	user := middleware.GetUser(c)
	var req setReadersRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	if err := h.projects.SetReaders(c.Request().Context(), user, c.Param("slug"), req.UserIDs); err != nil {
		return Error(c, err)
	}
	return NoContent(c)
}

func (h *ProjectHandler) buildListItem(ctx context.Context, p domain.Project) (mapper.ProjectListItem, error) {
	responsible, tags, tech, readers, err := h.loadMeta(ctx, p)
	if err != nil {
		return mapper.ProjectListItem{}, err
	}
	return mapper.ToProjectListItem(p, responsible, readers, tags, tech), nil
}

func (h *ProjectHandler) buildDetail(ctx context.Context, p domain.Project) (mapper.ProjectResponse, error) {
	responsible, tags, tech, readerNames, err := h.loadMeta(ctx, p)
	if err != nil {
		return mapper.ProjectResponse{}, err
	}

	sections, lessons, attachments, history, err := h.projects.LoadProjectData(ctx, p.ID)
	if err != nil {
		return mapper.ProjectResponse{}, httperr.Internal("falha ao carregar projeto")
	}

	lessonTags, _ := h.tags.ListLessonTagsByProject(ctx, p.ID)
	fileMap := make(map[string]domain.FileRecord)
	for _, a := range attachments {
		f, _ := h.files.GetByID(ctx, a.FileID)
		if f != nil {
			fileMap[f.ID] = *f
		}
	}

	authorIDs := make([]string, 0)
	for _, ev := range history {
		if ev.ActorUserID != nil {
			authorIDs = append(authorIDs, *ev.ActorUserID)
		}
	}
	authorNames, _ := h.users.GetNamesByIDs(ctx, authorIDs)

	return mapper.ToProjectResponse(mapper.ProjectBuildInput{
		Project: p, Responsible: responsible, ReaderNames: readerNames,
		Tags: tags, Tech: tech, Sections: sections, Lessons: lessons,
		LessonTags: lessonTags, Attachments: attachments, Files: fileMap,
		History: history, AuthorNames: authorNames, APIBaseURL: h.cfg.APIBaseURL,
	}), nil
}

func (h *ProjectHandler) loadMeta(ctx context.Context, p domain.Project) (responsible string, tags, tech, readers []string, err error) {
	u, _ := h.users.GetByID(ctx, p.ResponsibleUserID)
	if u != nil {
		responsible = u.Name
	}
	tags, err = h.tags.ListProjectTagNames(ctx, p.ID, domain.TagGeneral)
	if err != nil {
		return
	}
	tech, err = h.tags.ListProjectTagNames(ctx, p.ID, domain.TagTech)
	if err != nil {
		return
	}
	members, err := h.projects.ListMembers(ctx, p.ID)
	if err != nil {
		return
	}
	if len(members) == 0 {
		return responsible, tags, tech, nil, nil
	}
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	names, err := h.users.GetNamesByIDs(ctx, ids)
	if err != nil {
		return
	}
	for _, id := range ids {
		if name, ok := names[id]; ok {
			readers = append(readers, name)
		}
	}
	return responsible, tags, tech, readers, nil
}
