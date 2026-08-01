package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Feedback struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Version   int64          `json:"version"`
	Type      string         `json:"type"`
	Data      map[string]any `json:"data,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
	Snapshot  map[string]any `json:"snapshot,omitempty"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
}

type FeedbackCreateParams struct {
	UserID   string
	Type     string
	Data     map[string]any
	Meta     map[string]any
	Snapshot map[string]any
}

type FeedbackUpdateParams struct {
	Type     *string
	Data     map[string]any
	Meta     map[string]any
	Snapshot map[string]any
}

type FeedbackListOptions struct {
	OrderBy   string
	Direction string
	Skip      int
	Limit     int
}

type FeedbacksTable struct {
	db *dbinternal.Handle
}

func NewFeedbacksTable(db *dbinternal.Handle) *FeedbacksTable {
	return &FeedbacksTable{db: db}
}

func (t *FeedbacksTable) InsertNewFeedback(ctx context.Context, params FeedbackCreateParams) (*Feedback, error) {
	now := time.Now().Unix()
	dataJSON, err := marshalJSONMap(params.Data)
	if err != nil {
		return nil, err
	}
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	snapshotJSON, err := marshalJSONMap(params.Snapshot)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO feedback (id, user_id, version, type, data, meta, snapshot, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		id,
		params.UserID,
		0,
		params.Type,
		dataJSON,
		metaJSON,
		snapshotJSON,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetFeedbackByID(ctx, id)
}

func (t *FeedbacksTable) GetFeedbackByID(ctx context.Context, id string) (*Feedback, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, version, type, data, meta, snapshot, created_at, updated_at FROM feedback WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanFeedbackRow(row)
}

func (t *FeedbacksTable) GetFeedbackByIDAndUserID(ctx context.Context, id string, userID string) (*Feedback, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, version, type, data, meta, snapshot, created_at, updated_at FROM feedback WHERE id = ? AND user_id = ? LIMIT 1`, t.db.Dialect),
		id,
		userID,
	)
	return scanFeedbackRow(row)
}

func (t *FeedbacksTable) GetAllFeedbacks(ctx context.Context) ([]Feedback, error) {
	rows, err := t.db.DB.QueryContext(ctx, `SELECT id, user_id, version, type, data, meta, snapshot, created_at, updated_at FROM feedback ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeedbacks(rows)
}

func (t *FeedbacksTable) GetFeedbacksByUserID(ctx context.Context, userID string) ([]Feedback, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, version, type, data, meta, snapshot, created_at, updated_at FROM feedback WHERE user_id = ? ORDER BY created_at DESC`, t.db.Dialect),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeedbacks(rows)
}

func (t *FeedbacksTable) GetFeedbacksByChatID(ctx context.Context, chatID string) ([]Feedback, error) {
	feedbacks, err := t.GetAllFeedbacks(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Feedback, 0)
	for _, feedback := range feedbacks {
		if feedback.Meta == nil {
			continue
		}
		if metaChatID, _ := feedback.Meta["chat_id"].(string); metaChatID == chatID {
			items = append(items, feedback)
		}
	}
	return items, nil
}

func (t *FeedbacksTable) GetAllFeedbackIDs(ctx context.Context) ([]Feedback, error) {
	rows, err := t.db.DB.QueryContext(ctx, `SELECT id, user_id, version, type, data, meta, snapshot, created_at, updated_at FROM feedback ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeedbacks(rows)
}

func (t *FeedbacksTable) GetFeedbackItems(ctx context.Context, options FeedbackListOptions) ([]Feedback, int, error) {
	query := `SELECT id, user_id, version, type, data, meta, snapshot, created_at, updated_at FROM feedback`
	countQuery := `SELECT COUNT(*) FROM feedback`
	orderBy := "created_at"
	switch options.OrderBy {
	case "updated_at", "type", "user_id":
		orderBy = options.OrderBy
	}
	direction := "DESC"
	if options.Direction == "asc" {
		direction = "ASC"
	}
	query += " ORDER BY " + orderBy + " " + direction
	var total int
	if err := t.db.DB.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}
	args := []any{}
	if options.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, options.Limit)
	}
	if options.Skip > 0 {
		query += ` OFFSET ?`
		args = append(args, options.Skip)
	}
	rows, err := t.db.DB.QueryContext(ctx, rebindPlaceholders(query, t.db.Dialect), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanFeedbacks(rows)
	return items, total, err
}

func (t *FeedbacksTable) UpdateFeedbackByID(ctx context.Context, id string, params FeedbackUpdateParams) (*Feedback, error) {
	return t.updateFeedback(ctx, `UPDATE feedback SET `, ` WHERE id = ?`, []any{id}, params)
}

func (t *FeedbacksTable) UpdateFeedbackByIDAndUserID(ctx context.Context, id string, userID string, params FeedbackUpdateParams) (*Feedback, error) {
	return t.updateFeedback(ctx, `UPDATE feedback SET `, ` WHERE id = ? AND user_id = ?`, []any{id, userID}, params)
}

func (t *FeedbacksTable) updateFeedback(ctx context.Context, prefix string, suffix string, tailArgs []any, params FeedbackUpdateParams) (*Feedback, error) {
	assignments := []string{}
	args := []any{}
	if params.Type != nil {
		assignments = append(assignments, "type = ?")
		args = append(args, *params.Type)
	}
	if params.Data != nil {
		dataJSON, err := marshalJSONMap(params.Data)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "data = ?")
		args = append(args, dataJSON)
	}
	if params.Meta != nil {
		metaJSON, err := marshalJSONMap(params.Meta)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "meta = ?")
		args = append(args, metaJSON)
	}
	if params.Snapshot != nil {
		snapshotJSON, err := marshalJSONMap(params.Snapshot)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "snapshot = ?")
		args = append(args, snapshotJSON)
	}
	if len(assignments) == 0 {
		return nil, nil
	}
	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().Unix())
	args = append(args, tailArgs...)
	query := rebindPlaceholders(prefix+joinAssignments(assignments)+suffix, t.db.Dialect)
	result, err := t.db.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, nil
	}
	return t.GetFeedbackByID(ctx, tailArgs[0].(string))
}

func (t *FeedbacksTable) DeleteFeedbackByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM feedback WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (t *FeedbacksTable) DeleteFeedbackByIDAndUserID(ctx context.Context, id string, userID string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM feedback WHERE id = ? AND user_id = ?`, t.db.Dialect), id, userID)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (t *FeedbacksTable) DeleteAllFeedbacks(ctx context.Context) (bool, error) {
	_, err := t.db.DB.ExecContext(ctx, `DELETE FROM feedback`)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (t *FeedbacksTable) DeleteFeedbacksByUserID(ctx context.Context, userID string) (bool, error) {
	_, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM feedback WHERE user_id = ?`, t.db.Dialect), userID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func scanFeedbackRow(row *sql.Row) (*Feedback, error) {
	var feedback Feedback
	var dataRaw sql.NullString
	var metaRaw sql.NullString
	var snapshotRaw sql.NullString
	err := row.Scan(&feedback.ID, &feedback.UserID, &feedback.Version, &feedback.Type, &dataRaw, &metaRaw, &snapshotRaw, &feedback.CreatedAt, &feedback.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	feedback.Data, err = unmarshalJSONMap(dataRaw)
	if err != nil {
		return nil, err
	}
	feedback.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	feedback.Snapshot, err = unmarshalJSONMap(snapshotRaw)
	if err != nil {
		return nil, err
	}
	return &feedback, nil
}

func scanFeedbacks(rows *sql.Rows) ([]Feedback, error) {
	var feedbacks []Feedback
	for rows.Next() {
		var feedback Feedback
		var dataRaw sql.NullString
		var metaRaw sql.NullString
		var snapshotRaw sql.NullString
		if err := rows.Scan(&feedback.ID, &feedback.UserID, &feedback.Version, &feedback.Type, &dataRaw, &metaRaw, &snapshotRaw, &feedback.CreatedAt, &feedback.UpdatedAt); err != nil {
			return nil, err
		}
		var err error
		feedback.Data, err = unmarshalJSONMap(dataRaw)
		if err != nil {
			return nil, err
		}
		feedback.Meta, err = unmarshalJSONMap(metaRaw)
		if err != nil {
			return nil, err
		}
		feedback.Snapshot, err = unmarshalJSONMap(snapshotRaw)
		if err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, rows.Err()
}
