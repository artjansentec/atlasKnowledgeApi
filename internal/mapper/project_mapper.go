package mapper

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/atlas/knowledge-api/internal/domain"
)

type ProjectResponse struct {
	ID          string               `json:"id"`
	Slug        string               `json:"slug"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Status      string               `json:"status"`
	Responsible string               `json:"responsible"`
	Readers     []string             `json:"readers,omitempty"`
	Client      *string              `json:"client,omitempty"`
	CreatedAt   string               `json:"createdAt"`
	UpdatedAt   string               `json:"updatedAt"`
	Tags        []string             `json:"tags"`
	Tech        []string             `json:"tech,omitempty"`
	Attachments []AttachmentResponse `json:"attachments"`
	Lessons     []LessonResponse     `json:"lessons"`
	Sections    []SectionResponse    `json:"sections"`
	History     []HistoryResponse    `json:"history"`
}

type ProjectListItem struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Responsible string   `json:"responsible"`
	Readers     []string `json:"readers,omitempty"`
	Client      *string  `json:"client,omitempty"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
	Tags        []string `json:"tags"`
	Tech        []string `json:"tech,omitempty"`
}

type AttachmentResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	MimeType   *string `json:"mimeType,omitempty"`
	Size       string  `json:"size"`
	UploadedAt string  `json:"uploadedAt"`
	URL        *string `json:"url,omitempty"`
}

type LessonResponse struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Recommendation string   `json:"recommendation"`
	CreatedAt      string   `json:"createdAt"`
	Tags           []string `json:"tags,omitempty"`
}

type HistoryResponse struct {
	ID     string `json:"id"`
	At     string `json:"at"`
	Author string `json:"author"`
	Action string `json:"action"`
	Target string `json:"target"`
}

type ProjectBuildInput struct {
	Project     domain.Project
	Responsible string
	ReaderNames []string
	Tags        []string
	Tech        []string
	Sections    []domain.Section
	Lessons     []domain.Lesson
	LessonTags  map[string][]string
	Attachments []domain.Attachment
	Files       map[string]domain.FileRecord
	History     []domain.AuditEvent
	AuthorNames map[string]string
	APIBaseURL  string
}

func ToProjectResponse(in ProjectBuildInput) ProjectResponse {
	attachments := make([]AttachmentResponse, 0, len(in.Attachments))
	for _, a := range in.Attachments {
		file := in.Files[a.FileID]
		name := file.OriginalName
		if a.DisplayName != nil && *a.DisplayName != "" {
			name = *a.DisplayName
		}
		url := fmt.Sprintf("%s/api/v1/files/%s/download", strings.TrimRight(in.APIBaseURL, "/"), file.ID)
		attachments = append(attachments, AttachmentResponse{
			ID: a.ID, Name: name, Type: strings.TrimPrefix(filepath.Ext(name), "."),
			MimeType: &file.MimeType, Size: HumanSize(file.SizeBytes),
			UploadedAt: FormatDate(file.CreatedAt), URL: &url,
		})
	}

	lessons := make([]LessonResponse, 0, len(in.Lessons))
	for _, l := range in.Lessons {
		lessons = append(lessons, LessonResponse{
			ID: l.ID, Type: string(l.Type), Title: l.Title,
			Description: l.Description, Recommendation: l.Recommendation,
			CreatedAt: FormatDate(l.CreatedAt), Tags: in.LessonTags[l.ID],
		})
	}

	history := make([]HistoryResponse, 0, len(in.History))
	for _, h := range in.History {
		author := "Sistema"
		if h.ActorUserID != nil {
			if name, ok := in.AuthorNames[*h.ActorUserID]; ok {
				author = name
			}
		}
		history = append(history, HistoryResponse{
			ID: h.ID, At: FormatDate(h.CreatedAt), Author: author,
			Action: h.Action, Target: h.Target,
		})
	}

	return ProjectResponse{
		ID: in.Project.ID, Slug: in.Project.Slug, Name: in.Project.Name,
		Description: in.Project.Description, Status: string(in.Project.Status),
		Responsible: in.Responsible, Readers: in.ReaderNames, Client: in.Project.Client,
		CreatedAt: FormatDate(in.Project.CreatedAt), UpdatedAt: FormatDate(in.Project.UpdatedAt),
		Tags: in.Tags, Tech: in.Tech,
		Attachments: attachments, Lessons: lessons,
		Sections: BuildSectionTree(in.Sections), History: history,
	}
}

func ToProjectListItem(p domain.Project, responsible string, readers, tags, tech []string) ProjectListItem {
	return ProjectListItem{
		ID: p.ID, Slug: p.Slug, Name: p.Name, Description: p.Description,
		Status: string(p.Status), Responsible: responsible, Readers: readers,
		Client: p.Client, CreatedAt: FormatDate(p.CreatedAt), UpdatedAt: FormatDate(p.UpdatedAt),
		Tags: tags, Tech: tech,
	}
}

func FormatDate(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func HumanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
