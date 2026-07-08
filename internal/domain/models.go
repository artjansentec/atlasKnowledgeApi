package domain

import "time"

type UserRole string

const (
	RoleAdmin     UserRole = "admin"
	RoleConsultor UserRole = "consultor"
	RoleDeveloper UserRole = "desenvolvedor"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Name         string
	Role         UserRole
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type ProjectStatus string

const (
	StatusActive    ProjectStatus = "active"
	StatusPaused    ProjectStatus = "paused"
	StatusDone      ProjectStatus = "done"
	StatusCancelled ProjectStatus = "cancelled"
)

// ProjectStatusMeta descreve um status disponível na tabela project_statuses.
// A tabela é a fonte de verdade: rótulo e cores (texto/fundo) alimentam o front.
type ProjectStatusMeta struct {
	Code       string
	Label      string
	Color      string
	Background string
	SortOrder  int
}

type Project struct {
	ID                string
	Slug              string
	Name              string
	Description       string
	Status            ProjectStatus
	ResponsibleUserID string
	Client            *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type MemberRole string

const (
	MemberReader MemberRole = "reader"
	MemberEditor MemberRole = "editor"
)

type ProjectMember struct {
	ProjectID string
	UserID    string
	Role      MemberRole
}

type SectionKind string

const (
	SectionDoc SectionKind = "doc"
	SectionDev SectionKind = "dev"
)

type Section struct {
	ID        string
	ProjectID string
	ParentID  *string
	Title     string
	Content   string
	Kind      SectionKind
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type LessonType string

const (
	LessonProblem   LessonType = "problem"
	LessonAttention LessonType = "attention"
	LessonFuture    LessonType = "future"
	LessonSuccess   LessonType = "success"
)

type Lesson struct {
	ID             string
	ProjectID      string
	Type           LessonType
	Title          string
	Description    string
	Recommendation string
	CreatedBy      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type FileRecord struct {
	ID           string
	StorageKey   string
	OriginalName string
	MimeType     string
	SizeBytes    int64
	UploadedBy   string
	CreatedAt    time.Time
}

type AttachmentKind string

const (
	AttachmentProject AttachmentKind = "project"
	AttachmentDev     AttachmentKind = "dev"
)

type Attachment struct {
	ID          string
	ProjectID   string
	FileID      string
	DisplayName *string
	Kind        AttachmentKind
	CreatedAt   time.Time
}

type TagKind string

const (
	TagGeneral TagKind = "general"
	TagTech    TagKind = "tech"
)

type Tag struct {
	ID   string
	Name string
	Kind TagKind
}

type AuditEvent struct {
	ID          string
	ProjectID   string
	ActorUserID *string
	Action      string
	Target      string
	EntityType  *string
	EntityID    *string
	Metadata    []byte
	CreatedAt   time.Time
}

type ProjectListFilter struct {
	Status      string
	Query       string
	Responsible string
	Limit       int // 0 = sem limite
	Period      *DateRange
}

type DateRange struct {
	From time.Time
	To   time.Time
}

func (r DateRange) Valid() bool {
	return !r.From.IsZero() && !r.To.IsZero() && !r.From.After(r.To)
}

type SectionReorderItem struct {
	ID        string
	ParentID  *string
	SortOrder int
}
