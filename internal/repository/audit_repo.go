package repository

import (
	"context"
	"fmt"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type AuditRepository struct {
	db *db.DB
}

func NewAuditRepository(database *db.DB) *AuditRepository {
	return &AuditRepository{db: database}
}

func (r *AuditRepository) Create(ctx context.Context, tx pgx.Tx, e *domain.AuditEvent) error {
	query := `
		INSERT INTO audit_events (project_id, actor_user_id, action, target, entity_type, entity_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at
	`
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, query, e.ProjectID, e.ActorUserID, e.Action, e.Target, e.EntityType, e.EntityID, e.Metadata).Scan(&e.ID, &e.CreatedAt)
	} else {
		err = r.db.Pool.QueryRow(ctx, query, e.ProjectID, e.ActorUserID, e.Action, e.Target, e.EntityType, e.EntityID, e.Metadata).Scan(&e.ID, &e.CreatedAt)
	}
	return err
}

func (r *AuditRepository) ListByProject(ctx context.Context, projectID string) ([]domain.AuditEvent, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, project_id, actor_user_id, action, target, entity_type, entity_id, metadata, created_at
		FROM audit_events WHERE project_id = $1 ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.ActorUserID, &e.Action, &e.Target, &e.EntityType, &e.EntityID, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *AuditRepository) CountByProjects(ctx context.Context, projectIDs []string, period *domain.DateRange) (int, error) {
	if projectIDs != nil && len(projectIDs) == 0 {
		return 0, nil
	}
	query := `SELECT COUNT(*) FROM audit_events`
	args := []interface{}{}
	idx := 1
	where := false

	if period != nil {
		clause, periodArgs, nextIdx := dateRangeSQL("created_at", *period, idx)
		query += " WHERE " + clause
		args = append(args, periodArgs...)
		idx = nextIdx
		where = true
	}

	if projectIDs != nil {
		if where {
			query += fmt.Sprintf(" AND project_id = ANY($%d)", idx)
		} else {
			query += fmt.Sprintf(" WHERE project_id = ANY($%d)", idx)
		}
		args = append(args, projectIDs)
	}

	var count int
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *AuditRepository) Recent(ctx context.Context, limit int, allowedIDs []string, period *domain.DateRange) ([]domain.AuditEvent, error) {
	if allowedIDs != nil && len(allowedIDs) == 0 {
		return nil, nil
	}
	sql := `
		SELECT id, project_id, actor_user_id, action, target, entity_type, entity_id, metadata, created_at
		FROM audit_events
	`
	args := []interface{}{}
	idx := 1
	where := false

	if period != nil {
		clause, periodArgs, nextIdx := dateRangeSQL("created_at", *period, idx)
		sql += " WHERE " + clause
		args = append(args, periodArgs...)
		idx = nextIdx
		where = true
	}

	if allowedIDs != nil {
		if where {
			sql += fmt.Sprintf(" AND project_id = ANY($%d)", idx)
		} else {
			sql += fmt.Sprintf(" WHERE project_id = ANY($%d)", idx)
		}
		args = append(args, allowedIDs)
		idx++
	}

	sql += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", idx)
	args = append(args, limit)

	rows, err := r.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.ActorUserID, &e.Action, &e.Target, &e.EntityType, &e.EntityID, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *AuditRepository) Search(ctx context.Context, query string, allowedIDs []string) ([]domain.AuditEvent, error) {
	if allowedIDs != nil && len(allowedIDs) == 0 {
		return nil, nil
	}
	limit := SearchLimitUpdates
	pattern := likePattern(query)
	sql := `
		SELECT id, project_id, actor_user_id, action, target, entity_type, entity_id, metadata, created_at
		FROM audit_events
		WHERE created_at >= NOW() - make_interval(days => $1)
		  AND (LOWER(action) LIKE $2 OR LOWER(target) LIKE $2)
	`
	args := []interface{}{SearchUpdatesMaxAgeDays, pattern}
	if allowedIDs != nil {
		sql += " AND project_id = ANY($3)"
		args = append(args, allowedIDs)
	}
	sql += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d", limit)

	rows, err := r.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.ActorUserID, &e.Action, &e.Target, &e.EntityType, &e.EntityID, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
