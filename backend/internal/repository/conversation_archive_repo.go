package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

const conversationArchiveMetadataColumns = `id, request_id, user_id, api_key_id, group_id, protocol, endpoint, model,
 stream, status_code, session_key, jsonb_array_length(request_messages) AS message_count,
 length(response_text) AS response_chars, truncated, created_at`

// List 按筛选条件分页返回存档；IncludeContent 为 false 时只返回元数据与长度。
func (r *conversationArchiveRepository) List(ctx context.Context, filter service.ConversationArchiveFilter) ([]*service.ConversationArchive, int64, error) {
	where, args := conversationArchiveWhere(filter)

	var total int64
	if err := r.sql.QueryRowContext(ctx, `SELECT count(*) FROM conversation_archives`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*service.ConversationArchive{}, 0, nil
	}

	columns := conversationArchiveMetadataColumns
	if filter.IncludeContent {
		columns += `, request_messages, response_text`
	}
	order := ` ORDER BY created_at DESC, id DESC`
	if !filter.Descending {
		order = ` ORDER BY created_at ASC, id ASC`
	}
	queryArgs := append(append([]any(nil), args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	query := `SELECT ` + columns + ` FROM conversation_archives` + where + order +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)

	rows, err := r.sql.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	records := make([]*service.ConversationArchive, 0, filter.PageSize)
	for rows.Next() {
		record, err := scanConversationArchiveListRow(rows, filter.IncludeContent)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// GetByID 读取单条存档（含完整文本）；不存在时返回 (nil, nil)。
func (r *conversationArchiveRepository) GetByID(ctx context.Context, id int64) (*service.ConversationArchive, error) {
	row := r.sql.QueryRowContext(ctx, `SELECT `+conversationArchiveMetadataColumns+`, request_messages, response_text
 FROM conversation_archives WHERE id = $1`, id)
	record, err := scanConversationArchiveRow(row, true)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

func conversationArchiveWhere(filter service.ConversationArchiveFilter) (string, []any) {
	clauses := make([]string, 0, 12)
	args := make([]any, 0, 12)
	add := func(format string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(format, len(args)))
	}
	if filter.UserID != nil {
		add("user_id = $%d", *filter.UserID)
	}
	if filter.APIKeyID != nil {
		add("api_key_id = $%d", *filter.APIKeyID)
	}
	if filter.GroupID != nil {
		add("group_id = $%d", *filter.GroupID)
	}
	if filter.RequestID != "" {
		add("request_id = $%d", filter.RequestID)
	}
	if filter.SessionKey != "" {
		add("session_key = $%d", filter.SessionKey)
	}
	if filter.Protocol != "" {
		add("protocol = $%d", filter.Protocol)
	}
	if filter.Model != "" {
		add("model = $%d", filter.Model)
	}
	if filter.Endpoint != "" {
		add("endpoint = $%d", filter.Endpoint)
	}
	if filter.StatusCode != nil {
		add("status_code = $%d", *filter.StatusCode)
	}
	if filter.Stream != nil {
		add("stream = $%d", *filter.Stream)
	}
	if filter.Truncated != nil {
		add("truncated = $%d", *filter.Truncated)
	}
	if filter.StartTime != nil {
		add("created_at >= $%d", *filter.StartTime)
	}
	if filter.EndTime != nil {
		add("created_at <= $%d", *filter.EndTime)
	}
	if filter.Keyword != "" {
		args = append(args, "%"+escapeConversationArchiveLike(filter.Keyword)+"%")
		placeholder := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(request_messages::text ILIKE $%d ESCAPE '\\' OR response_text ILIKE $%d ESCAPE '\\')",
			placeholder, placeholder))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// escapeConversationArchiveLike 转义 LIKE 通配符，避免用户输入被当成模式。
func escapeConversationArchiveLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func scanConversationArchiveListRow(rows *sql.Rows, includeContent bool) (*service.ConversationArchive, error) {
	record := &service.ConversationArchive{}
	var (
		userID     sql.NullInt64
		apiKeyID   sql.NullInt64
		groupID    sql.NullInt64
		sessionKey sql.NullString
	)
	dest := []any{
		&record.ID, &record.RequestID, &userID, &apiKeyID, &groupID, &record.Protocol,
		&record.Endpoint, &record.Model, &record.Stream, &record.StatusCode, &sessionKey,
		&record.MessageCount, &record.ResponseChars, &record.Truncated, &record.CreatedAt,
	}
	var (
		messagesJSON []byte
		responseText string
	)
	if includeContent {
		dest = append(dest, &messagesJSON, &responseText)
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	applyConversationArchiveNullables(record, userID, apiKeyID, groupID, sessionKey)
	if includeContent {
		if err := decodeConversationArchiveContent(record, messagesJSON, responseText); err != nil {
			return nil, err
		}
	}
	return record, nil
}

func scanConversationArchiveRow(row *sql.Row, includeContent bool) (*service.ConversationArchive, error) {
	record := &service.ConversationArchive{}
	var (
		userID     sql.NullInt64
		apiKeyID   sql.NullInt64
		groupID    sql.NullInt64
		sessionKey sql.NullString
	)
	dest := []any{
		&record.ID, &record.RequestID, &userID, &apiKeyID, &groupID, &record.Protocol,
		&record.Endpoint, &record.Model, &record.Stream, &record.StatusCode, &sessionKey,
		&record.MessageCount, &record.ResponseChars, &record.Truncated, &record.CreatedAt,
	}
	var (
		messagesJSON []byte
		responseText string
	)
	if includeContent {
		dest = append(dest, &messagesJSON, &responseText)
	}
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	applyConversationArchiveNullables(record, userID, apiKeyID, groupID, sessionKey)
	if includeContent {
		if err := decodeConversationArchiveContent(record, messagesJSON, responseText); err != nil {
			return nil, err
		}
	}
	return record, nil
}

func applyConversationArchiveNullables(record *service.ConversationArchive, userID, apiKeyID, groupID sql.NullInt64, sessionKey sql.NullString) {
	if userID.Valid {
		record.UserID = userID.Int64
	}
	if apiKeyID.Valid {
		record.APIKeyID = apiKeyID.Int64
	}
	if groupID.Valid {
		value := groupID.Int64
		record.GroupID = &value
	}
	if sessionKey.Valid {
		record.SessionKey = sessionKey.String
	}
}

func decodeConversationArchiveContent(record *service.ConversationArchive, messagesJSON []byte, responseText string) error {
	record.ResponseText = responseText
	if len(messagesJSON) == 0 {
		record.Messages = []service.ConversationArchiveMessage{}
		return nil
	}
	messages := make([]service.ConversationArchiveMessage, 0, 4)
	if err := json.Unmarshal(messagesJSON, &messages); err != nil {
		return fmt.Errorf("decode conversation archive messages: %w", err)
	}
	record.Messages = messages
	return nil
}
