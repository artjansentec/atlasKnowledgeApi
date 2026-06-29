package repository

import (
	"context"
	"strings"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type TagRepository struct {
	db *db.DB
}

func NewTagRepository(database *db.DB) *TagRepository {
	return &TagRepository{db: database}
}

func (r *TagRepository) Upsert(ctx context.Context, tx pgx.Tx, name string, kind domain.TagKind) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO tags (name, kind) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, name, kind).Scan(&id)
	return id, err
}

func (r *TagRepository) SetProjectTags(ctx context.Context, tx pgx.Tx, projectID string, tagIDs []string, table string) error {
	if _, err := tx.Exec(ctx, "DELETE FROM "+table+" WHERE project_id = $1", projectID); err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(ctx, "INSERT INTO "+table+" (project_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", projectID, tagID); err != nil {
			return err
		}
	}
	return nil
}

func (r *TagRepository) SetLessonTags(ctx context.Context, lessonID string, tagIDs []string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM lesson_tags WHERE lesson_id = $1`, lessonID); err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO lesson_tags (lesson_id, tag_id) VALUES ($1, $2)`, lessonID, tagID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *TagRepository) ResolveNames(ctx context.Context, tx pgx.Tx, names []string, kind domain.TagKind) ([]string, error) {
	var ids []string
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO tags (name, kind) VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, name, kind).Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *TagRepository) ListProjectTagNames(ctx context.Context, projectID string, kind domain.TagKind) ([]string, error) {
	table := "project_tags"
	if kind == domain.TagTech {
		table = "project_tech"
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT t.name FROM tags t
		JOIN `+table+` pt ON pt.tag_id = t.id
		WHERE pt.project_id = $1 ORDER BY t.name
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

func (r *TagRepository) ListLessonTagNames(ctx context.Context, lessonID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT t.name FROM tags t
		JOIN lesson_tags lt ON lt.tag_id = t.id
		WHERE lt.lesson_id = $1 ORDER BY t.name
	`, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

func (r *TagRepository) ListLessonTagsByProject(ctx context.Context, projectID string) (map[string][]string, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT pl.id, t.name FROM project_lessons pl
		JOIN lesson_tags lt ON lt.lesson_id = pl.id
		JOIN tags t ON t.id = lt.tag_id
		WHERE pl.project_id = $1 AND pl.deleted_at IS NULL
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string][]string)
	for rows.Next() {
		var lessonID, name string
		if err := rows.Scan(&lessonID, &name); err != nil {
			return nil, err
		}
		result[lessonID] = append(result[lessonID], name)
	}
	return result, rows.Err()
}
