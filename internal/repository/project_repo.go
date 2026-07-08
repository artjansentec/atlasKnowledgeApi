package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type ProjectRepository struct {
	db *db.DB
}

func NewProjectRepository(database *db.DB) *ProjectRepository {
	return &ProjectRepository{db: database}
}

func (r *ProjectRepository) GetBySlug(ctx context.Context, slug string) (*domain.Project, error) {
	var p domain.Project
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, slug, name, description, status, responsible_user_id, client, created_at, updated_at, deleted_at
		FROM projects WHERE slug = $1 AND deleted_at IS NULL
	`, slug).Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.Status, &p.ResponsibleUserID, &p.Client, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return &p, nil
}

func (r *ProjectRepository) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	var p domain.Project
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, slug, name, description, status, responsible_user_id, client, created_at, updated_at, deleted_at
		FROM projects WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.Status, &p.ResponsibleUserID, &p.Client, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepository) List(ctx context.Context, filter domain.ProjectListFilter, allowedIDs []string) ([]domain.Project, error) {
	query := `
		SELECT id, slug, name, description, status, responsible_user_id, client, created_at, updated_at, deleted_at
		FROM projects WHERE deleted_at IS NULL
	`
	args := []interface{}{}
	idx := 1

	if len(allowedIDs) > 0 {
		query += fmt.Sprintf(" AND id = ANY($%d)", idx)
		args = append(args, allowedIDs)
		idx++
	} else if allowedIDs != nil {
		return []domain.Project{}, nil
	}

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, filter.Status)
		idx++
	}

	if filter.Responsible != "" {
		query += fmt.Sprintf(` AND responsible_user_id IN (
			SELECT id FROM users WHERE LOWER(name) LIKE $%d
		)`, idx)
		args = append(args, "%"+strings.ToLower(filter.Responsible)+"%")
		idx++
	}

	if filter.Query != "" {
		query += fmt.Sprintf(` AND (
			LOWER(name) LIKE $%d OR LOWER(description) LIKE $%d OR LOWER(slug) LIKE $%d
		)`, idx, idx, idx)
		q := "%" + strings.ToLower(filter.Query) + "%"
		args = append(args, q)
		idx++
	}

	if filter.Period != nil {
		clause, periodArgs, nextIdx := dateRangeSQL("updated_at", *filter.Period, idx)
		query += " AND " + clause
		args = append(args, periodArgs...)
		idx = nextIdx
	}

	query += " ORDER BY updated_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", idx)
		args = append(args, filter.Limit)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.Status, &p.ResponsibleUserID, &p.Client, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepository) Create(ctx context.Context, tx pgx.Tx, p *domain.Project) error {
	return tx.QueryRow(ctx, `
		INSERT INTO projects (slug, name, description, status, responsible_user_id, client)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, p.Slug, p.Name, p.Description, p.Status, p.ResponsibleUserID, p.Client).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *ProjectRepository) Update(ctx context.Context, tx pgx.Tx, p *domain.Project) error {
	_, err := tx.Exec(ctx, `
		UPDATE projects SET
			name = COALESCE(NULLIF($2, ''), name),
			description = COALESCE(NULLIF($3, ''), description),
			status = COALESCE(NULLIF($4, ''), status),
			responsible_user_id = COALESCE(NULLIF($5::uuid, '00000000-0000-0000-0000-000000000000'::uuid), responsible_user_id),
			client = $6
		WHERE id = $1 AND deleted_at IS NULL
	`, p.ID, p.Name, p.Description, string(p.Status), p.ResponsibleUserID, p.Client)
	return err
}

func (r *ProjectRepository) ListStatuses(ctx context.Context) ([]domain.ProjectStatusMeta, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT code, label, color, background, sort_order
		FROM project_statuses ORDER BY sort_order, code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var statuses []domain.ProjectStatusMeta
	for rows.Next() {
		var s domain.ProjectStatusMeta
		if err := rows.Scan(&s.Code, &s.Label, &s.Color, &s.Background, &s.SortOrder); err != nil {
			return nil, err
		}
		statuses = append(statuses, s)
	}
	return statuses, rows.Err()
}

func (r *ProjectRepository) GetStatus(ctx context.Context, code string) (*domain.ProjectStatusMeta, error) {
	var s domain.ProjectStatusMeta
	err := r.db.Pool.QueryRow(ctx, `
		SELECT code, label, color, background, sort_order
		FROM project_statuses WHERE code = $1
	`, code).Scan(&s.Code, &s.Label, &s.Color, &s.Background, &s.SortOrder)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ProjectRepository) Patch(ctx context.Context, id string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	sets := []string{}
	args := []interface{}{id}
	idx := 2
	for key, val := range fields {
		sets = append(sets, fmt.Sprintf("%s = $%d", key, idx))
		args = append(args, val)
		idx++
	}
	query := fmt.Sprintf("UPDATE projects SET %s WHERE id = $1 AND deleted_at IS NULL", strings.Join(sets, ", "))
	_, err := r.db.Pool.Exec(ctx, query, args...)
	return err
}

func (r *ProjectRepository) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE projects SET deleted_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *ProjectRepository) CountActive(ctx context.Context, allowedIDs []string, period *domain.DateRange) (int, error) {
	if allowedIDs != nil && len(allowedIDs) == 0 {
		return 0, nil
	}
	query := `SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL`
	args := []interface{}{}
	idx := 1

	if period != nil {
		clause, periodArgs, nextIdx := dateRangeSQL("updated_at", *period, idx)
		query += " AND " + clause
		args = append(args, periodArgs...)
		idx = nextIdx
	}

	if allowedIDs != nil {
		query += fmt.Sprintf(" AND id = ANY($%d)", idx)
		args = append(args, allowedIDs)
	}

	var count int
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *ProjectRepository) CountWithStatus(ctx context.Context, allowedIDs []string, status domain.ProjectStatus, period *domain.DateRange) (int, error) {
	if allowedIDs != nil && len(allowedIDs) == 0 {
		return 0, nil
	}
	query := `SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL AND status = $1`
	args := []interface{}{status}
	idx := 2

	if period != nil {
		clause, periodArgs, nextIdx := dateRangeSQL("updated_at", *period, idx)
		query += " AND " + clause
		args = append(args, periodArgs...)
		idx = nextIdx
	}

	if allowedIDs != nil {
		query += fmt.Sprintf(" AND id = ANY($%d)", idx)
		args = append(args, allowedIDs)
	}

	var count int
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *ProjectRepository) ListMembers(ctx context.Context, projectID string) ([]domain.ProjectMember, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT project_id, user_id, role FROM project_members WHERE project_id = $1
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []domain.ProjectMember
	for rows.Next() {
		var m domain.ProjectMember
		if err := rows.Scan(&m.ProjectID, &m.UserID, &m.Role); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *ProjectRepository) ReplaceReaders(ctx context.Context, projectID string, userIDs []string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM project_members WHERE project_id = $1`, projectID); err != nil {
		return err
	}
	for _, uid := range userIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'reader')
			ON CONFLICT DO NOTHING
		`, projectID, uid); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *ProjectRepository) ListDevResponsibleIDs(ctx context.Context, projectID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT user_id FROM project_dev_responsibles WHERE project_id = $1
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *ProjectRepository) SetDevResponsibles(ctx context.Context, tx pgx.Tx, projectID string, userIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM project_dev_responsibles WHERE project_id = $1`, projectID); err != nil {
		return err
	}
	for _, uid := range userIDs {
		if uid == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO project_dev_responsibles (project_id, user_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, projectID, uid); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProjectRepository) AccessibleProjectIDs(ctx context.Context, userID string, isAdmin bool) ([]string, error) {
	if isAdmin {
		return nil, nil // nil means all
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id FROM projects WHERE deleted_at IS NULL AND (
			responsible_user_id = $1
			OR id IN (SELECT project_id FROM project_members WHERE user_id = $1)
			OR id IN (SELECT project_id FROM project_dev_responsibles WHERE user_id = $1)
		)
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
