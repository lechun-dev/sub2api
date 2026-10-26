package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type conversationArchiveSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type conversationArchiveRepository struct {
	sql conversationArchiveSQLExecutor
}

// NewConversationArchiveRepository 创建会话存档仓储。
func NewConversationArchiveRepository(db *sql.DB) service.ConversationArchiveRepository {
	return &conversationArchiveRepository{sql: db}
}

const conversationArchiveColumns = `request_id, user_id, api_key_id, group_id, protocol, endpoint, model,
 stream, status_code, session_key, request_messages, response_text, request_bytes, response_bytes, truncated, created_at`

func (r *conversationArchiveRepository) CreateBatch(ctx context.Context, records []*service.ConversationArchive) error {
	if len(records) == 0 {
		return nil
	}
	const columnsPerRow = 16
	placeholders := make([]string, 0, len(records))
	args := make([]any, 0, len(records)*columnsPerRow)
	rowIndex := 0
	for _, record := range records {
		if record == nil {
			continue
		}
		messages, err := json.Marshal(record.Messages)
		if err != nil {
			return fmt.Errorf("marshal conversation archive messages: %w", err)
		}
		base := rowIndex * columnsPerRow
		rowIndex++
		values := make([]string, 0, columnsPerRow)
		for j := 1; j <= columnsPerRow; j++ {
			values = append(values, fmt.Sprintf("$%d", base+j))
		}
		placeholders = append(placeholders, "("+strings.Join(values, ", ")+")")
		createdAt := record.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		args = append(args, record.RequestID, conversationArchiveNullableID(record.UserID),
			conversationArchiveNullableID(record.APIKeyID), conversationArchiveNullableInt64Ptr(record.GroupID),
			record.Protocol, record.Endpoint, record.Model, record.Stream, record.StatusCode,
			conversationArchiveNullableString(record.SessionKey), messages, record.ResponseText,
			record.RequestBytes, record.ResponseBytes, record.Truncated, createdAt)
	}
	if len(placeholders) == 0 {
		return nil
	}
	query := `INSERT INTO conversation_archives (` + conversationArchiveColumns + `) VALUES ` +
		strings.Join(placeholders, ", ")
	_, err := r.sql.ExecContext(ctx, query, args...)
	return err
}

func (r *conversationArchiveRepository) DeleteBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	result, err := r.sql.ExecContext(ctx, `
WITH target AS (
  SELECT id FROM conversation_archives
  WHERE created_at < $1
  ORDER BY created_at ASC, id ASC
  LIMIT $2
)
DELETE FROM conversation_archives WHERE id IN (SELECT id FROM target)`, cutoff, limit)
	if err != nil {
		return 0, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

func conversationArchiveNullableID(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func conversationArchiveNullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func conversationArchiveNullableInt64Ptr(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
