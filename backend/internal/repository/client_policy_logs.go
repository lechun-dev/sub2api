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

// 2026-10-14 coder(lq): 复用错误日志表及 group/time 索引，固定错误类型和组 ID 精确查询，不按当前客户端规则重判历史。
func (r *opsRepository) ListClientPolicyDenials(ctx context.Context, filter service.ClientPolicyDenialFilter) (*service.ClientPolicyDenialList, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if filter.GroupID <= 0 || filter.StartTime.IsZero() || !filter.EndTime.After(filter.StartTime) || filter.EndTime.Sub(filter.StartTime) > 30*24*time.Hour || filter.Page > 100000 {
		return nil, fmt.Errorf("invalid client policy log filter")
	}
	page, size := filter.Page, filter.PageSize
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	where := `WHERE l.error_type = $1 AND l.group_id = $2 AND l.created_at >= $3 AND l.created_at < $4`
	args := []any{service.ClientPolicyDenialType, filter.GroupID, filter.StartTime.UTC(), filter.EndTime.UTC()}
	result := &service.ClientPolicyDenialList{Items: make([]*service.ClientPolicyDenialLog, 0, size), Page: page, PageSize: size}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ops_error_logs l `+where, args...).Scan(&result.Total); err != nil {
		return nil, err
	}
	args = append(args, size, (page-1)*size)
	// 2026-10-14 coder(lq): 查询时一次关联当前用户名；已删除或缺失账号仍保留日志，不回退到邮箱或用户 ID。
	rows, err := r.db.QueryContext(ctx, `SELECT l.id,l.created_at,l.group_id,l.user_id,l.api_key_id,COALESCE(host(l.client_ip),''),COALESCE(l.request_id,''),COALESCE(l.request_path,''),COALESCE(l.error_body,''),COALESCE(u.username,''),COALESCE(l.user_agent,'') FROM ops_error_logs l LEFT JOIN users u ON u.id = l.user_id AND u.deleted_at IS NULL `+where+` ORDER BY l.created_at DESC,l.id DESC LIMIT $5 OFFSET $6`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		row := &service.ClientPolicyDenialLog{}
		var user, key sql.NullInt64
		var raw string
		if err := rows.Scan(&row.ID, &row.CreatedAt, &row.GroupID, &user, &key, &row.ClientIP, &row.RequestID, &row.RequestPath, &raw, &row.Username, &row.UserAgent); err != nil {
			return nil, err
		}
		row.Username = strings.TrimSpace(row.Username)
		row.UserAgent = service.SanitizeClientPolicyUserAgent(row.UserAgent)
		if user.Valid {
			row.UserID = &user.Int64
		}
		if key.Valid {
			row.APIKeyID = &key.Int64
		}
		var snapshot service.ClientPolicySnapshot
		// 2026-10-14 coder(lq): 损坏或旧快照不把整个日志列表变成错误；只暴露强类型公开字段，不返回 raw JSON。
		if json.Unmarshal([]byte(raw), &snapshot) == nil && validClientPolicySnapshot(snapshot) {
			row.Snapshot = &snapshot
		}
		result.Items = append(result.Items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func validClientPolicySnapshot(snapshot service.ClientPolicySnapshot) bool {
	validCode := (snapshot.Mode == "allow" && (snapshot.Code == "client_not_allowed" || snapshot.Code == "client_unrecognized")) || (snapshot.Mode == "deny" && snapshot.Code == "client_forbidden")
	return validCode && snapshot.DetectedClients != nil && snapshot.SelectedClients != nil && len(snapshot.DetectedClients) <= 8 && len(snapshot.SelectedClients) > 0 && len(snapshot.SelectedClients) <= 8 && snapshot.DetectedCount >= len(snapshot.DetectedClients) && snapshot.SelectedCount >= len(snapshot.SelectedClients)
}
