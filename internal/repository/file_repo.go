package repository

import (
	"context"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type FileRepository struct {
	db *db.DB
}

func NewFileRepository(database *db.DB) *FileRepository {
	return &FileRepository{db: database}
}

func (r *FileRepository) Create(ctx context.Context, tx pgx.Tx, f *domain.FileRecord) error {
	return tx.QueryRow(ctx, `
		INSERT INTO files (storage_key, original_name, mime_type, size_bytes, uploaded_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, f.StorageKey, f.OriginalName, f.MimeType, f.SizeBytes, f.UploadedBy).Scan(&f.ID, &f.CreatedAt)
}

func (r *FileRepository) GetByID(ctx context.Context, id string) (*domain.FileRecord, error) {
	var f domain.FileRecord
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, storage_key, original_name, mime_type, size_bytes, uploaded_by, created_at
		FROM files WHERE id = $1
	`, id).Scan(&f.ID, &f.StorageKey, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.UploadedBy, &f.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

type AttachmentRepository struct {
	db *db.DB
}

func NewAttachmentRepository(database *db.DB) *AttachmentRepository {
	return &AttachmentRepository{db: database}
}

func (r *AttachmentRepository) ListByProject(ctx context.Context, projectID string, kind domain.AttachmentKind) ([]domain.Attachment, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, project_id, file_id, display_name, kind, created_at
		FROM project_attachments WHERE project_id = $1 AND kind = $2 ORDER BY created_at DESC
	`, projectID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Attachment
	for rows.Next() {
		var a domain.Attachment
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.FileID, &a.DisplayName, &a.Kind, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *AttachmentRepository) GetByID(ctx context.Context, projectID, attachmentID string) (*domain.Attachment, error) {
	var a domain.Attachment
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, project_id, file_id, display_name, kind, created_at
		FROM project_attachments WHERE id = $1 AND project_id = $2
	`, attachmentID, projectID).Scan(&a.ID, &a.ProjectID, &a.FileID, &a.DisplayName, &a.Kind, &a.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AttachmentRepository) Create(ctx context.Context, tx pgx.Tx, a *domain.Attachment) error {
	kind := a.Kind
	if kind == "" {
		kind = domain.AttachmentProject
	}
	a.Kind = kind
	return tx.QueryRow(ctx, `
		INSERT INTO project_attachments (project_id, file_id, display_name, kind)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at
	`, a.ProjectID, a.FileID, a.DisplayName, kind).Scan(&a.ID, &a.CreatedAt)
}

func (r *AttachmentRepository) Delete(ctx context.Context, attachmentID string) (*domain.Attachment, error) {
	var a domain.Attachment
	err := r.db.Pool.QueryRow(ctx, `
		DELETE FROM project_attachments WHERE id = $1
		RETURNING id, project_id, file_id, display_name, kind, created_at
	`, attachmentID).Scan(&a.ID, &a.ProjectID, &a.FileID, &a.DisplayName, &a.Kind, &a.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AttachmentRepository) GetProjectAndKindByFileID(ctx context.Context, fileID string) (string, domain.AttachmentKind, error) {
	var projectID string
	var kind domain.AttachmentKind
	err := r.db.Pool.QueryRow(ctx, `
		SELECT project_id, kind FROM project_attachments WHERE file_id = $1 LIMIT 1
	`, fileID).Scan(&projectID, &kind)
	if err == pgx.ErrNoRows {
		return "", "", nil
	}
	return projectID, kind, err
}
