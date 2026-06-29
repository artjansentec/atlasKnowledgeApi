package repository

import (
	"context"

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

func (r *AuditRepository) Recent(ctx context.Context, limit int, allowedIDs []string) ([]domain.AuditEvent, error) {
	if allowedIDs != nil && len(allowedIDs) == 0 {
		return nil, nil
	}
	sql := `
		SELECT id, project_id, actor_user_id, action, target, entity_type, entity_id, metadata, created_at
		FROM audit_events
	`
	args := []interface{}{}
	if allowedIDs != nil {
		sql += " WHERE project_id = ANY($1)"
		args = append(args, allowedIDs)
		sql += " ORDER BY created_at DESC LIMIT $2"
		args = append(args, limit)
	} else {
		sql += " ORDER BY created_at DESC LIMIT $1"
		args = append(args, limit)
	}

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
	sql := `
		SELECT id, project_id, actor_user_id, action, target, entity_type, entity_id, metadata, created_at
		FROM audit_events
		WHERE to_tsvector('portuguese', action || ' ' || target) @@ plainto_tsquery('portuguese', $1)
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
