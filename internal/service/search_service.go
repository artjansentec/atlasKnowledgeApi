package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

type SearchService struct {
	projects *repository.ProjectRepository
	sections *repository.SectionRepository
	lessons  *repository.LessonRepository
	audit    *repository.AuditRepository
	users    *repository.UserRepository
}

func NewSearchService(
	projects *repository.ProjectRepository,
	sections *repository.SectionRepository,
	lessons *repository.LessonRepository,
	audit *repository.AuditRepository,
	users *repository.UserRepository,
) *SearchService {
	return &SearchService{projects: projects, sections: sections, lessons: lessons, audit: audit, users: users}
}

type SearchResultItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Snippet     string `json:"snippet"`
	Meta        string `json:"meta"`
	Href        string `json:"href"`
	ProjectSlug string `json:"projectSlug"`
	ProjectName string `json:"projectName"`
}

type SearchResponse struct {
	Projects []SearchResultItem `json:"projects"`
	Sections []SearchResultItem `json:"sections"`
	Lessons  []SearchResultItem `json:"lessons"`
	Updates  []SearchResultItem `json:"updates"`
}

func (s *SearchService) Search(ctx context.Context, user domain.User, query string) (*SearchResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return &SearchResponse{}, nil
	}

	allowed, err := s.projects.AccessibleProjectIDs(ctx, user.ID, IsAdmin(user))
	if err != nil {
		return nil, httperr.Internal("falha na busca")
	}

	projects, err := s.projects.List(ctx, domain.ProjectListFilter{Query: query}, allowed)
	if err != nil {
		return nil, httperr.Internal("falha na busca")
	}

	sections, err := s.sections.Search(ctx, query, allowed)
	if err != nil {
		return nil, httperr.Internal("falha na busca")
	}

	lessons, err := s.lessons.Search(ctx, query, allowed)
	if err != nil {
		return nil, httperr.Internal("falha na busca")
	}

	updates, err := s.audit.Search(ctx, query, allowed)
	if err != nil {
		return nil, httperr.Internal("falha na busca")
	}

	projectMap := make(map[string]domain.Project)
	for _, p := range projects {
		projectMap[p.ID] = p
	}

	resp := &SearchResponse{}
	for _, p := range projects {
		responsible, _ := s.users.GetByID(ctx, p.ResponsibleUserID)
		responsibleName := ""
		if responsible != nil {
			responsibleName = responsible.Name
		}
		resp.Projects = append(resp.Projects, SearchResultItem{
			ID: p.ID + "-project", Type: "project", Title: p.Name, Snippet: p.Description,
			Meta: fmt.Sprintf("Responsável: %s · Atualizado em %s", responsibleName, formatDate(p.UpdatedAt)),
			Href: "/projects/" + p.Slug, ProjectSlug: p.Slug, ProjectName: p.Name,
		})
	}

	for _, sec := range sections {
		p, err := s.projects.GetByID(ctx, sec.ProjectID)
		if err != nil || p == nil {
			continue
		}
		resp.Sections = append(resp.Sections, SearchResultItem{
			ID: p.ID + "-section-" + sec.ID, Type: "section", Title: sec.Title,
			Snippet: truncate(sec.Content, 200), Meta: "Seção de documentação",
			Href: fmt.Sprintf("/projects/%s?section=%s", p.Slug, sec.ID),
			ProjectSlug: p.Slug, ProjectName: p.Name,
		})
	}

	for _, lesson := range lessons {
		p, err := s.projects.GetByID(ctx, lesson.ProjectID)
		if err != nil || p == nil {
			continue
		}
		resp.Lessons = append(resp.Lessons, SearchResultItem{
			ID: p.ID + "-lesson-" + lesson.ID, Type: "lesson", Title: lesson.Title,
			Snippet: truncate(lesson.Description+" "+lesson.Recommendation, 200),
			Meta: "Lição aprendida",
			Href: "/projects/" + p.Slug + "?tab=lessons",
			ProjectSlug: p.Slug, ProjectName: p.Name,
		})
	}

	for _, ev := range updates {
		p, err := s.projects.GetByID(ctx, ev.ProjectID)
		if err != nil || p == nil {
			continue
		}
		resp.Updates = append(resp.Updates, SearchResultItem{
			ID: p.ID + "-update-" + ev.ID, Type: "update", Title: ev.Action,
			Snippet: "Atualização em " + ev.Target + ".",
			Meta: formatDate(ev.CreatedAt),
			Href: "/projects/" + p.Slug + "?tab=history",
			ProjectSlug: p.Slug, ProjectName: p.Name,
		})
	}

	return resp, nil
}

type DashboardService struct {
	projects *repository.ProjectRepository
	sections *repository.SectionRepository
	lessons  *repository.LessonRepository
	audit    *repository.AuditRepository
	users    *repository.UserRepository
}

func NewDashboardService(
	projects *repository.ProjectRepository,
	sections *repository.SectionRepository,
	lessons *repository.LessonRepository,
	audit *repository.AuditRepository,
	users *repository.UserRepository,
) *DashboardService {
	return &DashboardService{projects: projects, sections: sections, lessons: lessons, audit: audit, users: users}
}

type DashboardSummary struct {
	ProjectCount    int                    `json:"projectCount"`
	DocumentCount   int                    `json:"documentCount"`
	LessonCount     int                    `json:"lessonCount"`
	RecentUpdates   []DashboardUpdateItem  `json:"recentUpdates"`
}

type DashboardUpdateItem struct {
	ID          string `json:"id"`
	At          string `json:"at"`
	Author      string `json:"author"`
	Action      string `json:"action"`
	Target      string `json:"target"`
	ProjectSlug string `json:"projectSlug"`
	ProjectName string `json:"projectName"`
}

func (s *DashboardService) Summary(ctx context.Context, user domain.User) (*DashboardSummary, error) {
	allowed, err := s.projects.AccessibleProjectIDs(ctx, user.ID, IsAdmin(user))
	if err != nil {
		return nil, httperr.Internal("falha ao carregar dashboard")
	}

	projectCount, err := s.projects.CountActive(ctx, allowed)
	if err != nil {
		return nil, httperr.Internal("falha ao carregar dashboard")
	}
	docCount, err := s.sections.CountByProjects(ctx, allowed)
	if err != nil {
		return nil, httperr.Internal("falha ao carregar dashboard")
	}
	lessonCount, err := s.lessons.CountByProjects(ctx, allowed)
	if err != nil {
		return nil, httperr.Internal("falha ao carregar dashboard")
	}

	events, err := s.audit.Recent(ctx, 6, allowed)
	if err != nil {
		return nil, httperr.Internal("falha ao carregar dashboard")
	}

	updates := make([]DashboardUpdateItem, 0, len(events))
	for _, ev := range events {
		p, _ := s.projects.GetByID(ctx, ev.ProjectID)
		author := "Sistema"
		if ev.ActorUserID != nil {
			u, _ := s.users.GetByID(ctx, *ev.ActorUserID)
			if u != nil {
				author = u.Name
			}
		}
		item := DashboardUpdateItem{
			ID: ev.ID, At: formatDate(ev.CreatedAt),
			Author: author, Action: ev.Action, Target: ev.Target,
		}
		if p != nil {
			item.ProjectSlug = p.Slug
			item.ProjectName = p.Name
		}
		updates = append(updates, item)
	}

	return &DashboardSummary{
		ProjectCount: projectCount, DocumentCount: docCount,
		LessonCount: lessonCount, RecentUpdates: updates,
	}, nil
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func formatDate(t interface{ Format(string) string }) string {
	return t.Format("2006-01-02")
}
