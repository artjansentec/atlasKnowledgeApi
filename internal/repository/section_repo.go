package repository

import (
	"context"
	"fmt"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type SectionRepository struct {
	db *db.DB
}

func NewSectionRepository(database *db.DB) *SectionRepository {
	return &SectionRepository{db: database}
}

func (r *SectionRepository) ListByProject(ctx context.Context, projectID string) ([]domain.Section, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, project_id, parent_id, title, content, sort_order, created_at, updated_at, deleted_at
		FROM project_sections
		WHERE project_id = $1 AND deleted_at IS NULL
		ORDER BY sort_order, title
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sections []domain.Section
	for rows.Next() {
		var s domain.Section
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ParentID, &s.Title, &s.Content, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt, &s.DeletedAt); err != nil {
			return nil, err
		}
		sections = append(sections, s)
	}
	return sections, rows.Err()
}

func (r *SectionRepository) GetByID(ctx context.Context, projectID, sectionID string) (*domain.Section, error) {
	var s domain.Section
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, project_id, parent_id, title, content, sort_order, created_at, updated_at, deleted_at
		FROM project_sections
		WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL
	`, sectionID, projectID).Scan(&s.ID, &s.ProjectID, &s.ParentID, &s.Title, &s.Content, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt, &s.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SectionRepository) Create(ctx context.Context, tx pgx.Tx, s *domain.Section) error {
	query := `
		INSERT INTO project_sections (project_id, parent_id, title, content, sort_order)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	args := []interface{}{s.ProjectID, s.ParentID, s.Title, s.Content, s.SortOrder}
	if tx != nil {
		return tx.QueryRow(ctx, query, args...).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	}
	return r.db.Pool.QueryRow(ctx, query, args...).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *SectionRepository) Update(ctx context.Context, sectionID string, title, content *string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE project_sections SET
			title = COALESCE($2, title),
			content = COALESCE($3, content)
		WHERE id = $1 AND deleted_at IS NULL
	`, sectionID, title, content)
	return err
}

func (r *SectionRepository) SoftDelete(ctx context.Context, sectionID string) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE project_sections SET deleted_at = NOW() WHERE id = $1`, sectionID)
	return err
}

func (r *SectionRepository) Reorder(ctx context.Context, projectID string, items []domain.SectionReorderItem) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			UPDATE project_sections SET parent_id = $3, sort_order = $4
			WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL
		`, item.ID, projectID, item.ParentID, item.SortOrder); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *SectionRepository) CountByProjects(ctx context.Context, projectIDs []string) (int, error) {
	if projectIDs != nil && len(projectIDs) == 0 {
		return 0, nil
	}
	var count int
	if projectIDs == nil {
		err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM project_sections WHERE deleted_at IS NULL`).Scan(&count)
		return count, err
	}
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM project_sections WHERE deleted_at IS NULL AND project_id = ANY($1)`, projectIDs).Scan(&count)
	return count, err
}

func (r *SectionRepository) Search(ctx context.Context, query string, allowedIDs []string) ([]domain.Section, error) {
	if allowedIDs != nil && len(allowedIDs) == 0 {
		return nil, nil
	}
	sql := `
		SELECT id, project_id, parent_id, title, content, sort_order, created_at, updated_at, deleted_at
		FROM project_sections
		WHERE deleted_at IS NULL
		  AND to_tsvector('portuguese', title || ' ' || content) @@ plainto_tsquery('portuguese', $1)
	`
	args := []interface{}{query}
	if allowedIDs != nil {
		sql += " AND project_id = ANY($2)"
		args = append(args, allowedIDs)
	}
	sql += " ORDER BY updated_at DESC LIMIT 50"

	rows, err := r.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("search sections: %w", err)
	}
	defer rows.Close()

	var sections []domain.Section
	for rows.Next() {
		var s domain.Section
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ParentID, &s.Title, &s.Content, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt, &s.DeletedAt); err != nil {
			return nil, err
		}
		sections = append(sections, s)
	}
	return sections, rows.Err()
}

func (r *SectionRepository) NextSortOrder(ctx context.Context, projectID string, parentID *string) (int, error) {
	var order int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(sort_order), -1) + 1 FROM project_sections
		WHERE project_id = $1 AND (($2::uuid IS NULL AND parent_id IS NULL) OR parent_id = $2) AND deleted_at IS NULL
	`, projectID, parentID).Scan(&order)
	return order, err
}
