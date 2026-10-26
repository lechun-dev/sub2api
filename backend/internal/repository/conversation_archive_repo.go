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
 stream, status_code, session_key, request_messages, response_text, input_tokens, output_tokens,
 cached_input_tokens, request_bytes, response_bytes, truncated, created_at`

func (r *conversationArchiveRepository) CreateBatch(ctx context.Context, records []*service.ConversationArchive) error {
	if len(records) == 0 {
		return nil
	}
	const columnsPerRow = 19
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
			conversationArchiveNullableInt64Ptr(record.InputTokens),
			conversationArchiveNullableInt64Ptr(record.OutputTokens),
			conversationArchiveNullableInt64Ptr(record.CachedInputTokens),
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

const conversationArchiveFrom = `conversation_archives ca
 LEFT JOIN users u ON u.id = ca.user_id
 LEFT JOIN groups g ON g.id = ca.group_id`

const conversationArchiveMetadataColumns = `ca.id, ca.request_id, ca.user_id, COALESCE(u.username, '') AS username,
 COALESCE(u.email, '') AS email, ca.api_key_id, ca.group_id, COALESCE(g.name, '') AS group_name,
 COALESCE(g.platform, '') AS group_platform, ca.protocol, ca.endpoint, ca.model, ca.stream,
 ca.status_code, ca.session_key, jsonb_array_length(ca.request_messages) AS message_count,
 length(ca.response_text) AS response_chars, ca.input_tokens, ca.output_tokens, ca.cached_input_tokens,
 ca.truncated, ca.created_at`

// List 按筛选条件分页返回存档；IncludeContent 为 false 时只返回元数据与长度。
func (r *conversationArchiveRepository) List(ctx context.Context, filter service.ConversationArchiveFilter) ([]*service.ConversationArchive, int64, error) {
	where, args := conversationArchiveWhere(filter)

	countFrom := "conversation_archives ca"
	if filter.UserName != "" {
		countFrom = `conversation_archives ca LEFT JOIN users u ON u.id = ca.user_id`
	}
	var total int64
	if err := r.sql.QueryRowContext(ctx, `SELECT count(*) FROM `+countFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*service.ConversationArchive{}, 0, nil
	}

	columns := conversationArchiveMetadataColumns
	if filter.IncludeContent {
		columns += `, ca.request_messages, ca.response_text`
	}
	order := ` ORDER BY ca.created_at DESC, ca.id DESC`
	if !filter.Descending {
		order = ` ORDER BY ca.created_at ASC, ca.id ASC`
	}
	queryArgs := append(append([]any(nil), args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	query := `SELECT ` + columns + ` FROM ` + conversationArchiveFrom + where + order +
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
	row := r.sql.QueryRowContext(ctx, `SELECT `+conversationArchiveMetadataColumns+`, ca.request_messages, ca.response_text
 FROM `+conversationArchiveFrom+` WHERE ca.id = $1`, id)
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
		add("ca.user_id = $%d", *filter.UserID)
	}
	if filter.UserName != "" {
		args = append(args, "%"+escapeConversationArchiveLike(filter.UserName)+"%")
		placeholder := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(u.username ILIKE $%d ESCAPE '\\' OR u.email ILIKE $%d ESCAPE '\\')", placeholder, placeholder))
	}
	if filter.APIKeyID != nil {
		add("ca.api_key_id = $%d", *filter.APIKeyID)
	}
	if filter.GroupID != nil {
		add("ca.group_id = $%d", *filter.GroupID)
	}
	if filter.RequestID != "" {
		add("ca.request_id = $%d", filter.RequestID)
	}
	if filter.SessionKey != "" {
		add("ca.session_key = $%d", filter.SessionKey)
	}
	if filter.Protocol != "" {
		add("ca.protocol = $%d", filter.Protocol)
	}
	if filter.Model != "" {
		add("ca.model = $%d", filter.Model)
	}
	if filter.Endpoint != "" {
		add("ca.endpoint = $%d", filter.Endpoint)
	}
	if filter.StatusCode != nil {
		add("ca.status_code = $%d", *filter.StatusCode)
	}
	if filter.Stream != nil {
		add("ca.stream = $%d", *filter.Stream)
	}
	if filter.Truncated != nil {
		add("ca.truncated = $%d", *filter.Truncated)
	}
	if filter.StartTime != nil {
		add("ca.created_at >= $%d", *filter.StartTime)
	}
	if filter.EndTime != nil {
		add("ca.created_at <= $%d", *filter.EndTime)
	}
	if filter.Keyword != "" {
		args = append(args, "%"+escapeConversationArchiveLike(filter.Keyword)+"%")
		placeholder := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(ca.request_messages::text ILIKE $%d ESCAPE '\\' OR ca.response_text ILIKE $%d ESCAPE '\\')",
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

type conversationArchiveNullables struct {
	userID            sql.NullInt64
	apiKeyID          sql.NullInt64
	groupID           sql.NullInt64
	sessionKey        sql.NullString
	inputTokens       sql.NullInt64
	outputTokens      sql.NullInt64
	cachedInputTokens sql.NullInt64
}

func (n *conversationArchiveNullables) destinations(record *service.ConversationArchive) []any {
	return []any{
		&record.ID, &record.RequestID, &n.userID, &record.Username, &record.Email, &n.apiKeyID,
		&n.groupID, &record.GroupName, &record.GroupPlatform, &record.Protocol, &record.Endpoint,
		&record.Model, &record.Stream, &record.StatusCode,
		&n.sessionKey, &record.MessageCount, &record.ResponseChars,
		&n.inputTokens, &n.outputTokens, &n.cachedInputTokens, &record.Truncated, &record.CreatedAt,
	}
}

func (n *conversationArchiveNullables) apply(record *service.ConversationArchive) {
	if n.userID.Valid {
		record.UserID = n.userID.Int64
	}
	if n.apiKeyID.Valid {
		record.APIKeyID = n.apiKeyID.Int64
	}
	if n.groupID.Valid {
		value := n.groupID.Int64
		record.GroupID = &value
	}
	if n.sessionKey.Valid {
		record.SessionKey = n.sessionKey.String
	}
	if n.inputTokens.Valid {
		value := n.inputTokens.Int64
		record.InputTokens = &value
	}
	if n.outputTokens.Valid {
		value := n.outputTokens.Int64
		record.OutputTokens = &value
	}
	if n.cachedInputTokens.Valid {
		value := n.cachedInputTokens.Int64
		record.CachedInputTokens = &value
	}
}

func scanConversationArchiveListRow(rows *sql.Rows, includeContent bool) (*service.ConversationArchive, error) {
	record := &service.ConversationArchive{}
	nullables := &conversationArchiveNullables{}
	dest := nullables.destinations(record)
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
	nullables.apply(record)
	if includeContent {
		if err := decodeConversationArchiveContent(record, messagesJSON, responseText); err != nil {
			return nil, err
		}
	}
	return record, nil
}

func scanConversationArchiveRow(row *sql.Row, includeContent bool) (*service.ConversationArchive, error) {
	record := &service.ConversationArchive{}
	nullables := &conversationArchiveNullables{}
	dest := nullables.destinations(record)
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
	nullables.apply(record)
	if includeContent {
		if err := decodeConversationArchiveContent(record, messagesJSON, responseText); err != nil {
			return nil, err
		}
	}
	return record, nil
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

// Stats 按维度汇总 token 用量，并返回筛选条件下的总计。
func (r *conversationArchiveRepository) Stats(ctx context.Context, filter service.ConversationArchiveFilter, groupBy string, limit int, descending bool) ([]*service.ConversationArchiveAggregate, *service.ConversationArchiveTotals, error) {
	where, args := conversationArchiveWhere(filter)

	totals := &service.ConversationArchiveTotals{}
	if err := r.sql.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(ca.input_tokens),
 COALESCE(SUM(ca.input_tokens), 0)::bigint, COALESCE(SUM(ca.output_tokens), 0)::bigint,
 COALESCE(SUM(ca.cached_input_tokens), 0)::bigint
 FROM `+conversationArchiveFrom+where, args...).Scan(
		&totals.Records, &totals.RecordsWithUsage, &totals.InputTokens, &totals.OutputTokens, &totals.CachedInputTokens,
	); err != nil {
		return nil, nil, err
	}

	if groupBy == service.ConversationArchiveGroupNone {
		return []*service.ConversationArchiveAggregate{}, totals, nil
	}

	selectExpr, groupExpr, ok := conversationArchiveGroupExpressions(groupBy)
	if !ok {
		return nil, nil, fmt.Errorf("unsupported conversation archive group by %q", groupBy)
	}
	direction := "DESC"
	if !descending {
		direction = "ASC"
	}
	queryArgs := append(append([]any(nil), args...), limit)
	query := `SELECT ` + selectExpr + `,
 COUNT(*) AS records,
 COUNT(ca.input_tokens) AS records_with_usage,
 COALESCE(SUM(ca.input_tokens), 0)::bigint AS input_tokens,
 COALESCE(SUM(ca.output_tokens), 0)::bigint AS output_tokens,
 COALESCE(SUM(ca.cached_input_tokens), 0)::bigint AS cached_input_tokens
 FROM ` + conversationArchiveFrom + where + `
 GROUP BY ` + groupExpr + `
 ORDER BY (COALESCE(SUM(ca.input_tokens), 0) + COALESCE(SUM(ca.output_tokens), 0)) ` + direction +
		fmt.Sprintf(" LIMIT $%d", len(args)+1)

	rows, err := r.sql.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*service.ConversationArchiveAggregate, 0, limit)
	for rows.Next() {
		item, err := scanConversationArchiveAggregate(rows, groupBy)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, totals, nil
}

func conversationArchiveGroupExpressions(groupBy string) (string, string, bool) {
	switch groupBy {
	case service.ConversationArchiveGroupUser:
		return `ca.user_id AS group_user_id, COALESCE(u.username, '') AS group_username, COALESCE(u.email, '') AS group_email`,
			`ca.user_id, u.username, u.email`, true
	case service.ConversationArchiveGroupDate:
		return `to_char(ca.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS group_date`, `1`, true
	case service.ConversationArchiveGroupModel:
		return `ca.model AS group_model`, `ca.model`, true
	case service.ConversationArchiveGroupProtocol:
		return `ca.protocol AS group_protocol`, `ca.protocol`, true
	case service.ConversationArchiveGroupGroup:
		return `ca.group_id AS group_group_id, COALESCE(g.name, '') AS group_group_name`, `ca.group_id, g.name`, true
	case service.ConversationArchiveGroupAPIKey:
		return `ca.api_key_id AS group_api_key_id`, `ca.api_key_id`, true
	default:
		return "", "", false
	}
}

func scanConversationArchiveAggregate(rows *sql.Rows, groupBy string) (*service.ConversationArchiveAggregate, error) {
	item := &service.ConversationArchiveAggregate{}
	var (
		userID    sql.NullInt64
		groupID   sql.NullInt64
		apiKeyID  sql.NullInt64
		username  string
		email     string
		date      string
		model     string
		protocol  string
		groupName string
	)
	dest := make([]any, 0, 8)
	switch groupBy {
	case service.ConversationArchiveGroupUser:
		dest = append(dest, &userID, &username, &email)
	case service.ConversationArchiveGroupDate:
		dest = append(dest, &date)
	case service.ConversationArchiveGroupModel:
		dest = append(dest, &model)
	case service.ConversationArchiveGroupProtocol:
		dest = append(dest, &protocol)
	case service.ConversationArchiveGroupGroup:
		dest = append(dest, &groupID, &groupName)
	case service.ConversationArchiveGroupAPIKey:
		dest = append(dest, &apiKeyID)
	}
	dest = append(dest, &item.Records, &item.RecordsWithUsage, &item.InputTokens, &item.OutputTokens, &item.CachedInputTokens)
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	switch groupBy {
	case service.ConversationArchiveGroupUser:
		if userID.Valid {
			value := userID.Int64
			item.UserID = &value
			item.Key = fmt.Sprintf("%d", value)
		}
		item.Username = username
		item.Email = email
	case service.ConversationArchiveGroupDate:
		item.Date = date
		item.Key = date
	case service.ConversationArchiveGroupModel:
		item.Model = model
		item.Key = model
	case service.ConversationArchiveGroupProtocol:
		item.Protocol = protocol
		item.Key = protocol
	case service.ConversationArchiveGroupGroup:
		if groupID.Valid {
			value := groupID.Int64
			item.GroupID = &value
			item.Key = fmt.Sprintf("%d", value)
		}
		item.GroupName = groupName
	case service.ConversationArchiveGroupAPIKey:
		if apiKeyID.Valid {
			value := apiKeyID.Int64
			item.APIKeyID = &value
			item.Key = fmt.Sprintf("%d", value)
		}
	}
	return item, nil
}
