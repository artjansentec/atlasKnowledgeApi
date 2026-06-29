package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type LessonRepository struct {
	db *db.DB
}

func NewLessonRepository(database *db.DB) *LessonRepository {
	return &LessonRepository{db: database}
}

func (r *LessonRepository) ListByProject(ctx context.Context, projectID string) ([]domain.Lesson, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, project_id, type, title, description, recommendation, created_by, created_at, updated_at, deleted_at
		FROM project_lessons WHERE project_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []domain.Lesson
	for rows.Next() {
		var l domain.Lesson
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Type, &l.Title, &l.Description, &l.Recommendation, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}

func (r *LessonRepository) GetByID(ctx context.Context, projectID, lessonID string) (*domain.Lesson, error) {
	var l domain.Lesson
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, project_id, type, title, description, recommendation, created_by, created_at, updated_at, deleted_at
		FROM project_lessons WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL
	`, lessonID, projectID).Scan(&l.ID, &l.ProjectID, &l.Type, &l.Title, &l.Description, &l.Recommendation, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *LessonRepository) Create(ctx context.Context, tx pgx.Tx, l *domain.Lesson) error {
	return tx.QueryRow(ctx, `
		INSERT INTO project_lessons (project_id, type, title, description, recommendation, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, l.ProjectID, l.Type, l.Title, l.Description, l.Recommendation, l.CreatedBy).Scan(&l.ID, &l.CreatedAt, &l.UpdatedAt)
}

func (r *LessonRepository) Update(ctx context.Context, lessonID string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	sets := []string{}
	args := []interface{}{lessonID}
	i := 2
	for key, val := range fields {
		if key == "type" {
			sets = append(sets, fmt.Sprintf("type = $%d::lesson_type", i))
		} else {
			sets = append(sets, fmt.Sprintf("%s = $%d", key, i))
		}
		args = append(args, val)
		i++
	}
	query := fmt.Sprintf("UPDATE project_lessons SET %s WHERE id = $1 AND deleted_at IS NULL", strings.Join(sets, ", "))
	_, err := r.db.Pool.Exec(ctx, query, args...)
	return err
}

func (r *LessonRepository) SoftDelete(ctx context.Context, lessonID string) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE project_lessons SET deleted_at = NOW() WHERE id = $1`, lessonID)
	return err
}

func (r *LessonRepository) CountByProjects(ctx context.Context, projectIDs []string) (int, error) {
	if projectIDs != nil && len(projectIDs) == 0 {
		return 0, nil
	}
	var count int
	if projectIDs == nil {
		err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM project_lessons WHERE deleted_at IS NULL`).Scan(&count)
		return count, err
	}
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM project_lessons WHERE deleted_at IS NULL AND project_id = ANY($1)`, projectIDs).Scan(&count)
	return count, err
}

func (r *LessonRepository) Search(ctx context.Context, query string, allowedIDs []string) ([]domain.Lesson, error) {
	if allowedIDs != nil && len(allowedIDs) == 0 {
		return nil, nil
	}
	sql := `
		SELECT id, project_id, type, title, description, recommendation, created_by, created_at, updated_at, deleted_at
		FROM project_lessons
		WHERE deleted_at IS NULL
		  AND to_tsvector('portuguese', title || ' ' || description || ' ' || recommendation) @@ plainto_tsquery('portuguese', $1)
	`
	args := []interface{}{query}
	if allowedIDs != nil {
		sql += " AND project_id = ANY($2)"
		args = append(args, allowedIDs)
	}
	sql += " ORDER BY created_at DESC LIMIT 50"

	rows, err := r.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []domain.Lesson
	for rows.Next() {
		var l domain.Lesson
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Type, &l.Title, &l.Description, &l.Recommendation, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}
