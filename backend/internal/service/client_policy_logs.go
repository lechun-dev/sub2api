package service

import (
	"context"
	"errors"
	"time"
)

const ClientPolicyDenialType = "client_policy_denied"
const ClientPolicyDenialContextKey = "client_policy_denial_event"

type ClientPolicyClient struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type ClientPolicySnapshot struct {
	Code            string               `json:"code"`
	Mode            string               `json:"mode"`
	DetectedClients []ClientPolicyClient `json:"detected_clients"`
	SelectedClients []ClientPolicyClient `json:"selected_clients"`
	DetectedCount   int                  `json:"detected_count"`
	SelectedCount   int                  `json:"selected_count"`
	PolicyUpdatedAt *time.Time           `json:"policy_updated_at,omitempty"`
}
type ClientPolicyDenialEvent struct {
	CreatedAt time.Time
	GroupID   int64
	Snapshot  ClientPolicySnapshot
}
type ClientPolicyDenialLog struct {
	ID          int64                 `json:"id"`
	CreatedAt   time.Time             `json:"created_at"`
	GroupID     int64                 `json:"group_id"`
	Username    string                `json:"username"`
	UserID      *int64                `json:"user_id,omitempty"`
	APIKeyID    *int64                `json:"api_key_id,omitempty"`
	ClientIP    string                `json:"client_ip"`
	RequestID   string                `json:"request_id"`
	RequestPath string                `json:"request_path"`
	Snapshot    *ClientPolicySnapshot `json:"snapshot,omitempty"`
}
type ClientPolicyDenialFilter struct {
	GroupID   int64
	StartTime time.Time
	EndTime   time.Time
	Page      int
	PageSize  int
}
type ClientPolicyDenialList struct {
	Items    []*ClientPolicyDenialLog `json:"items"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}
type clientPolicyDenialRepository interface {
	ListClientPolicyDenials(context.Context, ClientPolicyDenialFilter) (*ClientPolicyDenialList, error)
}

func (s *OpsService) ListClientPolicyDenials(ctx context.Context, filter ClientPolicyDenialFilter) (*ClientPolicyDenialList, error) {
	if s == nil {
		return nil, errors.New("日志服务不可用")
	}
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if filter.GroupID <= 0 || filter.StartTime.IsZero() || !filter.EndTime.After(filter.StartTime) || filter.EndTime.Sub(filter.StartTime) > 30*24*time.Hour {
		return nil, errors.New("拦截日志查询范围无效")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Page > 100000 {
		return nil, errors.New("日志分页超出范围")
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	repo, ok := s.opsRepo.(clientPolicyDenialRepository)
	if !ok {
		return nil, errors.New("日志存储不可用")
	}
	return repo.ListClientPolicyDenials(ctx, filter)
}

// 2026-10-14 coder(lq): 快照只保留公开客户端名称/ID；限制数量与名字长度，确保完整 JSON 不超过既有队列 8KB 限制。
func boundedPolicyClients(clients []ClientPolicyClient) []ClientPolicyClient {
	if len(clients) > 8 {
		clients = clients[:8]
	}
	result := make([]ClientPolicyClient, 0, len(clients))
	for _, client := range clients {
		name := []rune(client.Name)
		if len(name) > 48 {
			client.Name = string(name[:48]) + "…"
		}
		result = append(result, client)
	}
	return result
}
