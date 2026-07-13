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

type DocumentationJobStatus string

const (
	DocJobPending         DocumentationJobStatus = "PENDING"
	DocJobValidating      DocumentationJobStatus = "VALIDATING"
	DocJobUploadingFiles  DocumentationJobStatus = "UPLOADING_FILES"
	DocJobWaitingAI       DocumentationJobStatus = "WAITING_AI"
	DocJobProcessing      DocumentationJobStatus = "PROCESSING"
	DocJobCompleted       DocumentationJobStatus = "COMPLETED"
	DocJobFailed          DocumentationJobStatus = "FAILED"
	DocJobCancelled       DocumentationJobStatus = "CANCELLED"
)

func (s DocumentationJobStatus) IsTerminal() bool {
	return s == DocJobCompleted || s == DocJobFailed || s == DocJobCancelled
}

func (s DocumentationJobStatus) IsActive() bool {
	switch s {
	case DocJobPending, DocJobValidating, DocJobUploadingFiles, DocJobWaitingAI, DocJobProcessing:
		return true
	default:
		return false
	}
}

// DocumentationJob representa uma solicitação de geração de documentação.
type DocumentationJob struct {
	ID                 string
	ProjectID          string
	CreatedBy          string
	Status             DocumentationJobStatus
	Progress           int
	CurrentStep        string
	ProjectName        string
	Description        string
	GenerationOptions  []byte
	ErrorMessage       *string
	VersionID          *string
	FileCount          int
	TotalSizeBytes     int64
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// DocumentationVersion é uma geração persistida (conteúdo integral da IA).
type DocumentationVersion struct {
	ID                string
	ProjectID         string
	JobID             string
	CreatedBy         string
	VersionNumber     int
	Content           []byte
	ModelUsed         string
	Language          string
	ProcessingMs      int64
	FileCount         int
	TotalSizeBytes    int64
	GenerationOptions []byte
	CreatedAt         time.Time
	DeletedAt         *time.Time
}

// DocumentationFile liga um arquivo de entrada a um job/versão.
type DocumentationFile struct {
	ID          string
	JobID       string
	VersionID   *string
	FileID      string
	ContentHash *string
	CreatedAt   time.Time
}
