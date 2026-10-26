package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// AggregateUsageStats 按用户/日期/模型等维度汇总 usage_logs，口径与计费一致。
func (r *usageLogRepository) AggregateUsageStats(ctx context.Context, filters usagestats.UsageLogFilters, groupBy string, limit int, descending bool) ([]*service.UsageStatsAggregate, *service.UsageStatsTotals, error) {
	if r == nil || r.db == nil {
		return nil, nil, errors.New("usage statistics database is unavailable")
	}
	where, args := usageStatsWhere(filters)

	totals, err := r.usageStatsTotals(ctx, where, args)
	if err != nil {
		return nil, nil, err
	}
	if groupBy == service.UsageStatsGroupNone {
		return []*service.UsageStatsAggregate{}, totals, nil
	}

	selectExpr, groupExpr, ok := usageStatsGroupExpressions(groupBy)
	if !ok {
		return nil, nil, fmt.Errorf("unsupported usage stats group by %q", groupBy)
	}
	direction := "DESC"
	if !descending {
		direction = "ASC"
	}
	queryArgs := append(append([]any(nil), args...), limit)
	query := `SELECT ` + selectExpr + `,
 COUNT(*) AS requests,
 COALESCE(SUM(ul.input_tokens), 0)::bigint AS input_tokens,
 COALESCE(SUM(ul.output_tokens), 0)::bigint AS output_tokens,
 COALESCE(SUM(ul.cache_creation_tokens), 0)::bigint AS cache_creation_tokens,
 COALESCE(SUM(ul.cache_read_tokens), 0)::bigint AS cache_read_tokens,
 COALESCE(SUM(ul.total_cost), 0)::float8 AS total_cost
 FROM usage_logs ul
 LEFT JOIN users u ON u.id = ul.user_id
 LEFT JOIN groups g ON g.id = ul.group_id` + where + `
 GROUP BY ` + groupExpr + `
 ORDER BY (COALESCE(SUM(ul.input_tokens), 0) + COALESCE(SUM(ul.output_tokens), 0)) ` + direction +
		fmt.Sprintf(" LIMIT $%d", len(args)+1)

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*service.UsageStatsAggregate, 0, limit)
	for rows.Next() {
		item, err := scanUsageStatsAggregate(rows, groupBy)
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

func (r *usageLogRepository) usageStatsTotals(ctx context.Context, where string, args []any) (*service.UsageStatsTotals, error) {
	totals := &service.UsageStatsTotals{}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*),
 COALESCE(SUM(ul.input_tokens), 0)::bigint,
 COALESCE(SUM(ul.output_tokens), 0)::bigint,
 COALESCE(SUM(ul.cache_creation_tokens), 0)::bigint,
 COALESCE(SUM(ul.cache_read_tokens), 0)::bigint,
 COALESCE(SUM(ul.total_cost), 0)::float8
 FROM usage_logs ul
 LEFT JOIN users u ON u.id = ul.user_id`+where, args...).Scan(
		&totals.Requests, &totals.InputTokens, &totals.OutputTokens,
		&totals.CacheCreationTokens, &totals.CacheReadTokens, &totals.TotalCost,
	); err != nil {
		return nil, err
	}
	totals.TotalTokens = totals.InputTokens + totals.OutputTokens
	return totals, nil
}

func usageStatsWhere(filters usagestats.UsageLogFilters) (string, []any) {
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	add := func(format string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(format, len(args)))
	}
	if filters.UserID > 0 {
		add("ul.user_id = $%d", filters.UserID)
	}
	if filters.UserName != "" {
		args = append(args, "%"+escapeUsageStatsLike(filters.UserName)+"%")
		placeholder := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(u.username ILIKE $%d ESCAPE '\\' OR u.email ILIKE $%d ESCAPE '\\')", placeholder, placeholder))
	}
	if filters.APIKeyID > 0 {
		add("ul.api_key_id = $%d", filters.APIKeyID)
	}
	if filters.GroupID > 0 {
		add("ul.group_id = $%d", filters.GroupID)
	}
	if filters.Model != "" {
		add("ul.model = $%d", filters.Model)
	}
	if filters.StartTime != nil {
		add("ul.created_at >= $%d", *filters.StartTime)
	}
	if filters.EndTime != nil {
		add("ul.created_at < $%d", *filters.EndTime)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func escapeUsageStatsLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func usageStatsGroupExpressions(groupBy string) (string, string, bool) {
	switch groupBy {
	case service.UsageStatsGroupUser:
		return `ul.user_id AS group_user_id, COALESCE(u.username, '') AS group_username, COALESCE(u.email, '') AS group_email`,
			`ul.user_id, u.username, u.email`, true
	case service.UsageStatsGroupDate:
		return `to_char(ul.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS group_date`, `1`, true
	case service.UsageStatsGroupModel:
		return `ul.model AS group_model`, `ul.model`, true
	case service.UsageStatsGroupGroup:
		return `ul.group_id AS group_group_id, COALESCE(g.name, '') AS group_group_name`, `ul.group_id, g.name`, true
	case service.UsageStatsGroupAPIKey:
		return `ul.api_key_id AS group_api_key_id`, `ul.api_key_id`, true
	default:
		return "", "", false
	}
}

func scanUsageStatsAggregate(rows *sql.Rows, groupBy string) (*service.UsageStatsAggregate, error) {
	item := &service.UsageStatsAggregate{}
	var (
		userID    sql.NullInt64
		groupID   sql.NullInt64
		apiKeyID  sql.NullInt64
		username  string
		email     string
		date      string
		model     string
		groupName string
	)
	dest := make([]any, 0, 9)
	switch groupBy {
	case service.UsageStatsGroupUser:
		dest = append(dest, &userID, &username, &email)
	case service.UsageStatsGroupDate:
		dest = append(dest, &date)
	case service.UsageStatsGroupModel:
		dest = append(dest, &model)
	case service.UsageStatsGroupGroup:
		dest = append(dest, &groupID, &groupName)
	case service.UsageStatsGroupAPIKey:
		dest = append(dest, &apiKeyID)
	}
	dest = append(dest, &item.Requests, &item.InputTokens, &item.OutputTokens,
		&item.CacheCreationTokens, &item.CacheReadTokens, &item.TotalCost)
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	item.TotalTokens = item.InputTokens + item.OutputTokens
	switch groupBy {
	case service.UsageStatsGroupUser:
		if userID.Valid {
			value := userID.Int64
			item.UserID = &value
			item.Key = fmt.Sprintf("%d", value)
		}
		item.Username = username
		item.Email = email
	case service.UsageStatsGroupDate:
		item.Date = date
		item.Key = date
	case service.UsageStatsGroupModel:
		item.Model = model
		item.Key = model
	case service.UsageStatsGroupGroup:
		if groupID.Valid {
			value := groupID.Int64
			item.GroupID = &value
			item.Key = fmt.Sprintf("%d", value)
		}
		item.GroupName = groupName
	case service.UsageStatsGroupAPIKey:
		if apiKeyID.Valid {
			value := apiKeyID.Int64
			item.APIKeyID = &value
			item.Key = fmt.Sprintf("%d", value)
		}
	}
	return item, nil
}
