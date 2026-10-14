package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const SettingKeyClientAccess = "group_client_access_v1"

var ErrClientAccessConflict = errors.New("客户端配置已被其他管理员修改，请重新加载后再保存")

type ClientHeaderRule struct {
	Header   string `json:"header"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type ClientDefinition struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Enabled  bool               `json:"enabled"`
	Detector string             `json:"detector"`
	Match    string             `json:"match"`
	Rules    []ClientHeaderRule `json:"rules"`
}

type GroupClientPolicy struct {
	Mode      string     `json:"mode"`
	ClientIDs []string   `json:"client_ids"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type ClientAccessConfig struct {
	Revision string                       `json:"revision"`
	Clients  []ClientDefinition           `json:"clients"`
	Groups   map[string]GroupClientPolicy `json:"groups"`
}

type ClientAccessDecision struct {
	Allowed bool
	Code    string
	Message string
	Denial  *ClientPolicySnapshot
}

type clientAccessWriter interface {
	CompareAndSetClientAccess(context.Context, string, string, []int64) error
}

type cachedClientAccess struct {
	Config  *ClientAccessConfig
	Err     error
	Expires time.Time
}

func DefaultClientAccessConfig() ClientAccessConfig {
	return ClientAccessConfig{
		Clients: []ClientDefinition{
			{ID: "codex", Name: "Codex 官方客户端", Enabled: true, Detector: "codex", Match: "all", Rules: []ClientHeaderRule{}},
			{ID: "deepseek_harness", Name: "DeepSeek Harness", Enabled: true, Detector: "headers", Match: "all", Rules: []ClientHeaderRule{{Header: "User-Agent", Operator: "prefix", Value: "deepseek-harness/"}}},
		}, Groups: map[string]GroupClientPolicy{},
	}
}

var clientAccessIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var clientAccessHeaderPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)

func ValidateClientAccessConfig(cfg *ClientAccessConfig) error {
	if cfg == nil || cfg.Groups == nil || len(cfg.Clients) > 100 || len(cfg.Groups) > 10000 {
		return errors.New("客户端配置不完整或超过数量限制")
	}
	clients := map[string]ClientDefinition{}
	for _, client := range cfg.Clients {
		if !clientAccessIDPattern.MatchString(client.ID) || strings.TrimSpace(client.Name) == "" || utf8.RuneCountInString(client.Name) > 128 || strings.ContainsAny(client.Name, "\r\n\x00") {
			return errors.New("客户端标识或名称无效")
		}
		if _, exists := clients[client.ID]; exists {
			return fmt.Errorf("客户端标识重复：%s", client.ID)
		}
		clients[client.ID] = client
		if client.Match != "all" && client.Match != "any" {
			return errors.New("客户端条件关系必须为 all 或 any")
		}
		if client.Detector == "codex" {
			if client.ID != "codex" || len(client.Rules) != 0 {
				return errors.New("Codex 内置识别器不可更换或配置请求头规则")
			}
			continue
		}
		if client.ID == "codex" || client.Detector != "headers" || len(client.Rules) == 0 || len(client.Rules) > 12 {
			return errors.New("自定义客户端需要 1 至 12 条请求头条件")
		}
		for _, rule := range client.Rules {
			header := strings.ToLower(rule.Header)
			// 2026-10-13 coder(lq): 规则只维护公开产品标识，禁止把 Key、令牌或 Cookie 作为客户端指纹。
			if !clientAccessHeaderPattern.MatchString(rule.Header) || (header != "user-agent" && header != "originator" && !strings.HasPrefix(header, "x-")) || strings.Contains(header, "auth") || strings.Contains(header, "key") || strings.Contains(header, "token") || strings.Contains(header, "cookie") {
				return errors.New("仅支持公开 User-Agent、Originator 或非凭据 X- 请求头")
			}
			if strings.TrimSpace(rule.Value) == "" || len(rule.Value) > 256 || strings.ContainsAny(rule.Value, "\r\n\x00") {
				return errors.New("请求头匹配值不能为空或包含控制字符")
			}
			if rule.Operator != "exact" && rule.Operator != "prefix" && rule.Operator != "contains" {
				return errors.New("不支持的请求头匹配方式")
			}
		}
	}
	for groupID, policy := range cfg.Groups {
		id, err := strconv.ParseInt(groupID, 10, 64)
		if err != nil || id <= 0 || strconv.FormatInt(id, 10) != groupID {
			return errors.New("分组 ID 无效")
		}
		if policy.Mode != "off" && policy.Mode != "allow" && policy.Mode != "deny" {
			return errors.New("分组限制方式无效")
		}
		if policy.Mode == "off" {
			if len(policy.ClientIDs) != 0 {
				return errors.New("不限制模式不能保留客户端选择")
			}
			continue
		}
		if len(policy.ClientIDs) == 0 || len(policy.ClientIDs) > 100 {
			return errors.New("限制模式至少选择一个客户端")
		}
		seen := map[string]bool{}
		for _, id := range policy.ClientIDs {
			client, exists := clients[id]
			if !exists || !client.Enabled {
				return fmt.Errorf("分组 %s 引用了不存在或停用的客户端：%s", groupID, id)
			}
			if seen[id] {
				return errors.New("分组客户端选择重复")
			}
			seen[id] = true
		}
	}
	return nil
}

func ClientDefinitionMatches(client ClientDefinition, headers http.Header) bool {
	if !client.Enabled {
		return false
	}
	if client.Detector == "codex" {
		return openai.IsCodexOfficialClientRequestStrict(headers.Get("User-Agent"))
	}
	if client.Detector != "headers" || len(client.Rules) == 0 {
		return false
	}
	for _, rule := range client.Rules {
		value, expected := headers.Get(rule.Header), rule.Value
		if strings.EqualFold(rule.Header, "User-Agent") {
			value, expected = strings.ToLower(value), strings.ToLower(expected)
		}
		matched := false
		if value != "" {
			switch rule.Operator {
			case "exact":
				matched = value == expected
			case "prefix":
				matched = strings.HasPrefix(value, expected)
			case "contains":
				matched = strings.Contains(value, expected)
			}
		}
		if client.Match == "any" && matched {
			return true
		}
		if client.Match == "all" && !matched {
			return false
		}
	}
	return client.Match == "all"
}

func EvaluateClientAccess(cfg *ClientAccessConfig, groupID int64, headers http.Header) ClientAccessDecision {
	policy, exists := cfg.Groups[strconv.FormatInt(groupID, 10)]
	if !exists || policy.Mode == "off" {
		return ClientAccessDecision{Allowed: true}
	}
	selected := map[string]bool{}
	for _, id := range policy.ClientIDs {
		selected[id] = true
	}
	names := []string{}
	selectedClients, detectedClients := []ClientPolicyClient{}, []ClientPolicyClient{}
	matchedSelected, recognized := false, false
	for _, client := range cfg.Clients {
		if selected[client.ID] {
			names = append(names, client.Name)
			selectedClients = append(selectedClients, ClientPolicyClient{ID: client.ID, Name: client.Name})
		}
		matched := ClientDefinitionMatches(client, headers)
		if matched {
			detectedClients = append(detectedClients, ClientPolicyClient{ID: client.ID, Name: client.Name})
		}
		recognized = recognized || matched
		matchedSelected = matchedSelected || (matched && selected[client.ID])
	}
	denied := func(code, message string) ClientAccessDecision {
		userAgentPresent := strings.TrimSpace(headers.Get("User-Agent")) != ""
		recognitionReason := "matched_registered_client"
		if !recognized {
			recognitionReason = "no_matching_rule"
			if !userAgentPresent {
				recognitionReason = "user_agent_missing"
			}
		}
		return ClientAccessDecision{Code: code, Message: message, Denial: &ClientPolicySnapshot{
			UserAgentPresent: &userAgentPresent, RecognitionReason: recognitionReason, Code: code, Mode: policy.Mode, DetectedClients: boundedPolicyClients(detectedClients), SelectedClients: boundedPolicyClients(selectedClients),
			DetectedCount: len(detectedClients), SelectedCount: len(selectedClients), PolicyUpdatedAt: policy.UpdatedAt,
		}}
	}
	// 2026-10-13 coder(lq): 不把客户端识别降成“取第一个命中”，避免重叠规则绕过黑名单。
	if policy.Mode == "deny" {
		if !matchedSelected {
			return ClientAccessDecision{Allowed: true}
		}
		return denied("client_forbidden", "当前 API Key 所属分组禁止使用："+strings.Join(names, "、")+"。请更换客户端后重试。")
	}
	if matchedSelected {
		return ClientAccessDecision{Allowed: true}
	}
	code, prefix := "client_not_allowed", ""
	if !recognized {
		code, prefix = "client_unrecognized", "未能识别当前客户端。"
	}
	return denied(code, prefix+"当前 API Key 所属分组仅允许使用："+strings.Join(names, "、")+"。请切换到允许的客户端后重试。")
}

func (s *SettingService) readClientAccess(ctx context.Context) (*ClientAccessConfig, string, error) {
	if s == nil || s.settingRepo == nil {
		return nil, "", errors.New("客户端配置存储不可用")
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyClientAccess)
	if errors.Is(err, ErrSettingNotFound) {
		cfg := DefaultClientAccessConfig()
		return &cfg, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	var cfg ClientAccessConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, "", errors.New("客户端配置损坏")
	}
	if err := ValidateClientAccessConfig(&cfg); err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256([]byte(raw))
	cfg.Revision = hex.EncodeToString(digest[:])
	return &cfg, raw, nil
}

func (s *SettingService) GetClientAccessConfig(ctx context.Context) (*ClientAccessConfig, error) {
	cfg, _, err := s.readClientAccess(ctx)
	return cfg, err
}

func (s *SettingService) RuntimeClientAccessConfig(ctx context.Context) (*ClientAccessConfig, error) {
	if s == nil {
		return nil, errors.New("客户端配置服务不可用")
	}
	s.clientAccessMu.Lock()
	defer s.clientAccessMu.Unlock()
	if s.clientAccessCache != nil && time.Now().Before(s.clientAccessCache.Expires) {
		return s.clientAccessCache.Config, s.clientAccessCache.Err
	}
	// 2026-10-13 coder(lq): 到期后同步读取，不用陈旧权限无限放行；失败也短暂缓存，防止故障时打爆数据库。
	loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	cfg, err := s.GetClientAccessConfig(loadCtx)
	s.clientAccessCache = &cachedClientAccess{Config: cfg, Err: err, Expires: time.Now().Add(2 * time.Second)}
	return cfg, err
}

func (s *SettingService) SaveClientAccessConfig(ctx context.Context, cfg ClientAccessConfig) error {
	if err := ValidateClientAccessConfig(&cfg); err != nil {
		return err
	}
	current, raw, err := s.readClientAccess(ctx)
	if err != nil {
		return err
	}
	if cfg.Revision != current.Revision {
		return ErrClientAccessConflict
	}
	writer, ok := s.settingRepo.(clientAccessWriter)
	if !ok {
		return errors.New("存储不支持客户端配置并发保护")
	}
	groupIDs := []int64{}
	for key := range cfg.Groups {
		id, _ := strconv.ParseInt(key, 10, 64)
		groupIDs = append(groupIDs, id)
	}
	stampClientPolicyDates(&cfg, current, time.Now().UTC())
	cfg.Revision = ""
	value, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := writer.CompareAndSetClientAccess(ctx, raw, string(value), groupIDs); err != nil {
		return err
	}
	s.clientAccessMu.Lock()
	s.clientAccessCache = nil
	s.clientAccessMu.Unlock()
	return nil
}
