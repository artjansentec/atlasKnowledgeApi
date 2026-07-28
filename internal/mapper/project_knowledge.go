package mapper

import "time"

// ProjectKnowledge is the full textual corpus of a project for Mnemos indexing.
type ProjectKnowledge struct {
	ID                  string                    `json:"id"`
	Slug                string                    `json:"slug"`
	Name                string                    `json:"name"`
	Description         string                    `json:"description"`
	Status              string                    `json:"status"`
	Client              *string                   `json:"client,omitempty"`
	ResponsibleUserID   string                    `json:"responsible_user_id"`
	ResponsibleName     string                    `json:"responsible_name"`
	Tags                []string                  `json:"tags"`
	Technologies        []string                  `json:"technologies"`
	WikiSections        []KnowledgeSection        `json:"wiki_sections"`
	DevSections         []KnowledgeSection        `json:"dev_sections"`
	Lessons             []KnowledgeLesson         `json:"lessons"`
	Attachments         []KnowledgeAttachment     `json:"attachments"`
	LatestDocumentation *KnowledgeDocumentation   `json:"latest_documentation,omitempty"`
	UpdatedAt           time.Time                 `json:"updated_at"`
}

type KnowledgeSection struct {
	ID       string              `json:"id"`
	ParentID *string             `json:"parent_id,omitempty"`
	Title    string              `json:"title"`
	Content  string              `json:"content"`
	Kind     string              `json:"kind"`
	Children []KnowledgeSection  `json:"children,omitempty"`
}

type KnowledgeLesson struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Recommendation string   `json:"recommendation"`
	Tags           []string `json:"tags"`
}

type KnowledgeAttachment struct {
	ID            string  `json:"id"`
	Kind          string  `json:"kind"`
	DisplayName   string  `json:"display_name"`
	OriginalName  string  `json:"original_name"`
	MimeType      string  `json:"mime_type"`
	ExtractedText *string `json:"extracted_text,omitempty"`
}

type KnowledgeDocumentation struct {
	ID            string `json:"id"`
	VersionNumber int    `json:"version_number"`
	ModelUsed     string `json:"model_used,omitempty"`
	Language      string `json:"language,omitempty"`
	ContentText   string `json:"content_text"`
}

// ProjectIDPage is a paginated list of project IDs for Mnemos bootstrap.
type ProjectIDPage struct {
	Items    []string `json:"items"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
	Total    int      `json:"total"`
}
