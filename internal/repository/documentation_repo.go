package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DocumentationRepository struct {
	db *db.DB
}

func NewDocumentationRepository(database *db.DB) *DocumentationRepository {
	return &DocumentationRepository{db: database}
}

func (r *DocumentationRepository) pool() *pgxpool.Pool {
	return r.db.Pool
}

func (r *DocumentationRepository) CreateJob(ctx context.Context, tx pgx.Tx, job *domain.DocumentationJob) error {
	opts := job.GenerationOptions
	if len(opts) == 0 {
		opts = []byte("{}")
	}
	return tx.QueryRow(ctx, `
		INSERT INTO documentation_jobs (
			project_id, created_by, status, progress, current_step,
			project_name, description, generation_options,
			file_count, total_size_bytes, started_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at, updated_at
	`,
		job.ProjectID, job.CreatedBy, job.Status, job.Progress, job.CurrentStep,
		job.ProjectName, job.Description, opts,
		job.FileCount, job.TotalSizeBytes, job.StartedAt,
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)
}

func (r *DocumentationRepository) UpdateJobStatus(
	ctx context.Context,
	jobID string,
	status domain.DocumentationJobStatus,
	progress int,
	currentStep string,
	errMsg *string,
) error {
	now := time.Now().UTC()
	var finishedAt *time.Time
	if status.IsTerminal() {
		finishedAt = &now
	}
	tag, err := r.pool().Exec(ctx, `
		UPDATE documentation_jobs
		SET status = $2,
		    progress = $3,
		    current_step = $4,
		    error_message = $5,
		    finished_at = COALESCE($6, finished_at),
		    started_at = COALESCE(started_at, CASE WHEN $2::documentation_job_status <> 'PENDING' THEN NOW() ELSE NULL END)
		WHERE id = $1
		  AND status NOT IN ('CANCELLED')
	`, jobID, status, progress, currentStep, errMsg, finishedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 && status == domain.DocJobCompleted {
		return nil
	}
	return nil
}

func (r *DocumentationRepository) SetJobVersion(ctx context.Context, jobID, versionID string) error {
	_, err := r.pool().Exec(ctx, `
		UPDATE documentation_jobs SET version_id = $2 WHERE id = $1
	`, jobID, versionID)
	return err
}

func (r *DocumentationRepository) GetJobByID(ctx context.Context, jobID string) (*domain.DocumentationJob, error) {
	return r.scanJob(r.pool().QueryRow(ctx, `
		SELECT id, project_id, created_by, status, progress, current_step,
		       project_name, description, generation_options, error_message, version_id,
		       file_count, total_size_bytes, started_at, finished_at, created_at, updated_at
		FROM documentation_jobs WHERE id = $1
	`, jobID))
}

func (r *DocumentationRepository) HasActiveJob(ctx context.Context, projectID string) (bool, error) {
	var exists bool
	err := r.pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM documentation_jobs
			WHERE project_id = $1
			  AND status IN ('PENDING','VALIDATING','UPLOADING_FILES','WAITING_AI','PROCESSING')
		)
	`, projectID).Scan(&exists)
	return exists, err
}

// ActiveJobRow é um job em andamento com dados do projeto.
type ActiveJobRow struct {
	Job         domain.DocumentationJob
	ProjectSlug string
	ProjectName string
}

func (r *DocumentationRepository) ListActiveJobs(ctx context.Context, projectID *string) ([]ActiveJobRow, error) {
	query := `
		SELECT j.id, j.project_id, j.created_by, j.status, j.progress, j.current_step,
		       j.project_name, j.description, j.generation_options, j.error_message, j.version_id,
		       j.file_count, j.total_size_bytes, j.started_at, j.finished_at, j.created_at, j.updated_at,
		       p.slug, p.name
		FROM documentation_jobs j
		JOIN projects p ON p.id = j.project_id
		WHERE j.status IN ('PENDING','VALIDATING','UPLOADING_FILES','WAITING_AI','PROCESSING')
	`
	args := []any{}
	if projectID != nil && *projectID != "" {
		query += ` AND j.project_id = $1`
		args = append(args, *projectID)
	}
	query += ` ORDER BY j.created_at DESC`

	rows, err := r.pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ActiveJobRow
	for rows.Next() {
		var row ActiveJobRow
		if err := rows.Scan(
			&row.Job.ID, &row.Job.ProjectID, &row.Job.CreatedBy, &row.Job.Status, &row.Job.Progress, &row.Job.CurrentStep,
			&row.Job.ProjectName, &row.Job.Description, &row.Job.GenerationOptions, &row.Job.ErrorMessage, &row.Job.VersionID,
			&row.Job.FileCount, &row.Job.TotalSizeBytes, &row.Job.StartedAt, &row.Job.FinishedAt, &row.Job.CreatedAt, &row.Job.UpdatedAt,
			&row.ProjectSlug, &row.ProjectName,
		); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (r *DocumentationRepository) CancelJob(ctx context.Context, jobID string) (*domain.DocumentationJob, error) {
	row := r.pool().QueryRow(ctx, `
		UPDATE documentation_jobs
		SET status = 'CANCELLED',
		    progress = progress,
		    current_step = 'Cancelado',
		    finished_at = NOW()
		WHERE id = $1
		  AND status IN ('PENDING','VALIDATING','UPLOADING_FILES','WAITING_AI','PROCESSING')
		RETURNING id, project_id, created_by, status, progress, current_step,
		          project_name, description, generation_options, error_message, version_id,
		          file_count, total_size_bytes, started_at, finished_at, created_at, updated_at
	`, jobID)
	return r.scanJob(row)
}

func (r *DocumentationRepository) AddFile(ctx context.Context, tx pgx.Tx, f *domain.DocumentationFile) error {
	return tx.QueryRow(ctx, `
		INSERT INTO documentation_files (job_id, version_id, file_id, content_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, f.JobID, f.VersionID, f.FileID, f.ContentHash).Scan(&f.ID, &f.CreatedAt)
}

func (r *DocumentationRepository) ListFilesByJob(ctx context.Context, jobID string) ([]domain.DocumentationFile, error) {
	rows, err := r.pool().Query(ctx, `
		SELECT id, job_id, version_id, file_id, content_hash, created_at
		FROM documentation_files WHERE job_id = $1 ORDER BY created_at
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.DocumentationFile
	for rows.Next() {
		var f domain.DocumentationFile
		if err := rows.Scan(&f.ID, &f.JobID, &f.VersionID, &f.FileID, &f.ContentHash, &f.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, f)
	}
	return items, rows.Err()
}

func (r *DocumentationRepository) LinkFilesToVersion(ctx context.Context, jobID, versionID string) error {
	_, err := r.pool().Exec(ctx, `
		UPDATE documentation_files SET version_id = $2 WHERE job_id = $1
	`, jobID, versionID)
	return err
}

func (r *DocumentationRepository) NextVersionNumber(ctx context.Context, tx pgx.Tx, projectID string) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version_number), 0) + 1
		FROM documentation_versions WHERE project_id = $1
	`, projectID).Scan(&n)
	return n, err
}

func (r *DocumentationRepository) CreateVersion(ctx context.Context, tx pgx.Tx, v *domain.DocumentationVersion) error {
	opts := v.GenerationOptions
	if len(opts) == 0 {
		opts = []byte("{}")
	}
	return tx.QueryRow(ctx, `
		INSERT INTO documentation_versions (
			project_id, job_id, created_by, version_number, content,
			model_used, language, processing_ms, file_count, total_size_bytes, generation_options
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at
	`,
		v.ProjectID, v.JobID, v.CreatedBy, v.VersionNumber, v.Content,
		v.ModelUsed, v.Language, v.ProcessingMs, v.FileCount, v.TotalSizeBytes, opts,
	).Scan(&v.ID, &v.CreatedAt)
}

func (r *DocumentationRepository) GetLatestVersion(ctx context.Context, projectID string) (*domain.DocumentationVersion, error) {
	return r.scanVersion(r.pool().QueryRow(ctx, `
		SELECT id, project_id, job_id, created_by, version_number, content,
		       model_used, language, processing_ms, file_count, total_size_bytes,
		       generation_options, created_at, deleted_at
		FROM documentation_versions
		WHERE project_id = $1 AND deleted_at IS NULL
		ORDER BY version_number DESC
		LIMIT 1
	`, projectID))
}

func (r *DocumentationRepository) GetVersionByID(ctx context.Context, projectID, versionID string) (*domain.DocumentationVersion, error) {
	return r.scanVersion(r.pool().QueryRow(ctx, `
		SELECT id, project_id, job_id, created_by, version_number, content,
		       model_used, language, processing_ms, file_count, total_size_bytes,
		       generation_options, created_at, deleted_at
		FROM documentation_versions
		WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL
	`, versionID, projectID))
}

func (r *DocumentationRepository) ListVersions(ctx context.Context, projectID string) ([]domain.DocumentationVersion, error) {
	rows, err := r.pool().Query(ctx, `
		SELECT id, project_id, job_id, created_by, version_number, content,
		       model_used, language, processing_ms, file_count, total_size_bytes,
		       generation_options, created_at, deleted_at
		FROM documentation_versions
		WHERE project_id = $1 AND deleted_at IS NULL
		ORDER BY version_number DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.DocumentationVersion
	for rows.Next() {
		var v domain.DocumentationVersion
		if err := rows.Scan(
			&v.ID, &v.ProjectID, &v.JobID, &v.CreatedBy, &v.VersionNumber, &v.Content,
			&v.ModelUsed, &v.Language, &v.ProcessingMs, &v.FileCount, &v.TotalSizeBytes,
			&v.GenerationOptions, &v.CreatedAt, &v.DeletedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *DocumentationRepository) SoftDeleteVersion(ctx context.Context, projectID, versionID string) (*domain.DocumentationVersion, error) {
	return r.scanVersion(r.pool().QueryRow(ctx, `
		UPDATE documentation_versions
		SET deleted_at = NOW()
		WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL
		RETURNING id, project_id, job_id, created_by, version_number, content,
		          model_used, language, processing_ms, file_count, total_size_bytes,
		          generation_options, created_at, deleted_at
	`, versionID, projectID))
}

func (r *DocumentationRepository) ListFilesByVersion(ctx context.Context, versionID string) ([]domain.DocumentationFile, error) {
	rows, err := r.pool().Query(ctx, `
		SELECT id, job_id, version_id, file_id, content_hash, created_at
		FROM documentation_files WHERE version_id = $1 ORDER BY created_at
	`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.DocumentationFile
	for rows.Next() {
		var f domain.DocumentationFile
		if err := rows.Scan(&f.ID, &f.JobID, &f.VersionID, &f.FileID, &f.ContentHash, &f.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, f)
	}
	return items, rows.Err()
}

func (r *DocumentationRepository) scanJob(row pgx.Row) (*domain.DocumentationJob, error) {
	var j domain.DocumentationJob
	err := row.Scan(
		&j.ID, &j.ProjectID, &j.CreatedBy, &j.Status, &j.Progress, &j.CurrentStep,
		&j.ProjectName, &j.Description, &j.GenerationOptions, &j.ErrorMessage, &j.VersionID,
		&j.FileCount, &j.TotalSizeBytes, &j.StartedAt, &j.FinishedAt, &j.CreatedAt, &j.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan documentation job: %w", err)
	}
	return &j, nil
}

func (r *DocumentationRepository) scanVersion(row pgx.Row) (*domain.DocumentationVersion, error) {
	var v domain.DocumentationVersion
	err := row.Scan(
		&v.ID, &v.ProjectID, &v.JobID, &v.CreatedBy, &v.VersionNumber, &v.Content,
		&v.ModelUsed, &v.Language, &v.ProcessingMs, &v.FileCount, &v.TotalSizeBytes,
		&v.GenerationOptions, &v.CreatedAt, &v.DeletedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan documentation version: %w", err)
	}
	return &v, nil
}
