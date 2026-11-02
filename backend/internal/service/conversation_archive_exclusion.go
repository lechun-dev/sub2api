package service

import (
	"context"
	"encoding/json"
	"strings"
)

// 会话存档的「本机客户端排除」逻辑。
//
// DSH（DeepSeek Harness）在每次模型请求上固定发送 x-deepseek-harness-user-id，
// 取值来自客户端首次启动时生成的 ~/.dsh/.anonymous-user-id，一台机器一个且长期不变。
// 它天然就是一个「本机指纹」：把这台机器的标识写进排除名单，它的会话就不再进存档，
// 客户端侧不需要任何改造（DSH 的 deepseek-official 适配器不支持自定义请求头）。
const (
	// ConversationArchiveHarnessUserIDHeader 是 DSH 请求携带的本机标识头。
	ConversationArchiveHarnessUserIDHeader = "x-deepseek-harness-user-id"

	// ConversationArchiveClientIDHeader 是通用客户端标识头：
	// 能自定义请求头的客户端（OpenClaw、脚本、其他 CLI）用它上报机器名或设备 ID，
	// 与 DSH 的标识共用同一份排除名单，网关侧统一管理。
	ConversationArchiveClientIDHeader = "x-sub2api-archive-client-id"

	// conversationArchiveExcludedClientMaxEntries 排除名单最多保留多少条；
	// conversationArchiveExcludedClientMaxRunes 单条标识的最大字符数。
	conversationArchiveExcludedClientMaxEntries = 200
	conversationArchiveExcludedClientMaxRunes   = 128
)

// NormalizeConversationArchiveExcludedHarnessUserIDs 归一化排除名单：去空白、转小写、
// 去重、丢弃超长项，并按上限截断。返回值可能为空切片，表示「没有排除任何客户端」。
func NormalizeConversationArchiveExcludedHarnessUserIDs(raw []string) []string {
	out := make([]string, 0, len(raw))
	if len(raw) == 0 {
		return out
	}
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		id := strings.ToLower(strings.TrimSpace(item))
		if id == "" || len([]rune(id)) > conversationArchiveExcludedClientMaxRunes {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
		if len(out) >= conversationArchiveExcludedClientMaxEntries {
			break
		}
	}
	return out
}

// ParseConversationArchiveExcludedHarnessUserIDs 解析设置值。入库格式是 JSON 数组，
// 同时兼容逗号/分号/换行分隔，方便直接改库或从命令行写入。非法内容按空名单处理。
func ParseConversationArchiveExcludedHarnessUserIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var items []string
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			return []string{}
		}
	} else {
		items = strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r'
		})
	}
	return NormalizeConversationArchiveExcludedHarnessUserIDs(items)
}

// IsExcludedConversationArchiveClient 判断客户端标识是否在排除名单内；空标识一律不排除。
func IsExcludedConversationArchiveClient(excluded []string, clientID string) bool {
	clientID = strings.ToLower(strings.TrimSpace(clientID))
	if clientID == "" || len(excluded) == 0 {
		return false
	}
	for _, id := range excluded {
		if id == clientID {
			return true
		}
	}
	return false
}

// GetConversationArchiveExcludedHarnessUserIDs 读取排除名单；读取失败按空名单处理，
// 即失败时继续归档（宁可多存，也不因为设置读不到而静默漏档）。
func (s *SettingService) GetConversationArchiveExcludedHarnessUserIDs(ctx context.Context) []string {
	if s == nil || s.settingRepo == nil {
		return []string{}
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyConversationArchiveExcludedHarnessUserIDs)
	if err != nil {
		return []string{}
	}
	return ParseConversationArchiveExcludedHarnessUserIDs(value)
}
