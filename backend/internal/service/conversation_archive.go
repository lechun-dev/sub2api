package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	// ConversationArchiveProtocolAnthropicMessages Anthropic Messages 协议（客户端视角）。
	ConversationArchiveProtocolAnthropicMessages = "anthropic_messages"
	// ConversationArchiveProtocolOpenAIChat OpenAI Chat Completions 协议。
	ConversationArchiveProtocolOpenAIChat = "openai_chat"
	// ConversationArchiveProtocolOpenAIResponses OpenAI Responses 协议。
	ConversationArchiveProtocolOpenAIResponses = "openai_responses"
	// ConversationArchiveProtocolGemini Gemini 原生 generateContent 协议。
	ConversationArchiveProtocolGemini = "gemini"

	// ConversationArchiveRequestLimitBytes 单个请求体最多归档多少字节，超出则不存档该请求。
	ConversationArchiveRequestLimitBytes = 8 << 20 // 8 MiB，覆盖带图片/长上下文的请求
	// ConversationArchiveResponseLimitBytes 单个响应最多捕获多少字节，超出后标记 truncated。
	ConversationArchiveResponseLimitBytes = 4 << 20 // 4 MiB

	conversationArchiveMaxMessageRunes  = 32768
	conversationArchiveMaxMessagesRunes = 131072
	conversationArchiveMaxResponseRunes = 65536
	conversationArchiveSessionKeyRunes  = 128

	conversationArchiveQueueCapacity = 8192
	conversationArchiveBatchSize     = 100
	conversationArchiveFlushInterval = 500 * time.Millisecond

	conversationArchiveRetentionInterval     = 24 * time.Hour
	conversationArchiveRetentionStartupDelay = 5 * time.Minute
	conversationArchiveRetentionBatchSize    = 5000
)

// ConversationArchiveMessage 是归档中的一条文本消息；图片/工具/思考内容不落库。
type ConversationArchiveMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ConversationArchive 是一轮用户与 AI 对话的归档记录。
type ConversationArchive struct {
	ID                int64                        `json:"id"`
	RequestID         string                       `json:"request_id"`
	UserID            int64                        `json:"user_id"`
	Username          string                       `json:"username,omitempty"`
	Email             string                       `json:"email,omitempty"`
	APIKeyID          int64                        `json:"api_key_id"`
	GroupID           *int64                       `json:"group_id,omitempty"`
	GroupName         string                       `json:"group_name,omitempty"`
	GroupPlatform     string                       `json:"group_platform,omitempty"`
	Protocol          string                       `json:"protocol"`
	Endpoint          string                       `json:"endpoint"`
	Model             string                       `json:"model"`
	Stream            bool                         `json:"stream"`
	StatusCode        int                          `json:"status_code"`
	SessionKey        string                       `json:"session_key,omitempty"`
	Messages          []ConversationArchiveMessage `json:"request_messages"`
	ResponseText      string                       `json:"response_text"`
	RequestBytes      int                          `json:"request_bytes"`
	ResponseBytes     int                          `json:"response_bytes"`
	InputTokens       *int64                       `json:"input_tokens,omitempty"`
	OutputTokens      *int64                       `json:"output_tokens,omitempty"`
	CachedInputTokens *int64                       `json:"cached_input_tokens,omitempty"`
	Truncated         bool                         `json:"truncated"`
	MessageCount      int                          `json:"message_count"`
	ResponseChars     int                          `json:"response_chars"`
	CreatedAt         time.Time                    `json:"created_at"`
}

// 会话存档统计的分组维度。
const (
	ConversationArchiveGroupNone     = "none"
	ConversationArchiveGroupUser     = "user"
	ConversationArchiveGroupDate     = "date"
	ConversationArchiveGroupModel    = "model"
	ConversationArchiveGroupProtocol = "protocol"
	ConversationArchiveGroupGroup    = "group"
	ConversationArchiveGroupAPIKey   = "api_key"
)

// ConversationArchiveTotals 是一组存档的 token 汇总；样本数用于区分「无数据」与「上游未返回」。
type ConversationArchiveTotals struct {
	Records           int64 `json:"records"`
	RecordsWithUsage  int64 `json:"records_with_usage"`
	InputTokens       int64 `json:"input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	CachedInputTokens int64 `json:"cached_input_tokens"`
}

// ConversationArchiveAggregate 是按维度聚合后的一行；具体分组字段由 GroupBy 决定。
type ConversationArchiveAggregate struct {
	Key       string `json:"key"`
	UserID    *int64 `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`
	Email     string `json:"email,omitempty"`
	GroupID   *int64 `json:"group_id,omitempty"`
	GroupName string `json:"group_name,omitempty"`
	APIKeyID  *int64 `json:"api_key_id,omitempty"`
	Model     string `json:"model,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	Date      string `json:"date,omitempty"`
	ConversationArchiveTotals
}

// ConversationArchiveFilter 是管理端查询存档的过筛条件。
type ConversationArchiveFilter struct {
	Page     int
	PageSize int
	UserID   *int64
	APIKeyID *int64
	GroupID  *int64
	// UserName 按用户姓名（用户名或邮箱）模糊匹配。
	UserName   string
	RequestID  string
	SessionKey string
	Protocol   string
	Model      string
	Endpoint   string
	Keyword    string
	StatusCode *int
	Stream     *bool
	Truncated  *bool
	StartTime  *time.Time
	EndTime    *time.Time
	// IncludeContent 为 false 时只返回元数据与长度，避免管理端列表把大段原文一次拉回。
	IncludeContent bool
	// Descending 控制 created_at 排序方向，默认倒序（最新在前）。
	Descending bool
}

// ConversationArchiveRepository 持久化会话存档。
type ConversationArchiveRepository interface {
	CreateBatch(ctx context.Context, records []*ConversationArchive) error
	DeleteBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	List(ctx context.Context, filter ConversationArchiveFilter) ([]*ConversationArchive, int64, error)
	GetByID(ctx context.Context, id int64) (*ConversationArchive, error)
	Stats(ctx context.Context, filter ConversationArchiveFilter, groupBy string, limit int, descending bool) ([]*ConversationArchiveAggregate, *ConversationArchiveTotals, error)
}

// ErrConversationArchiveNotFound 表示查询的存档不存在。
var ErrConversationArchiveNotFound = infraerrors.NotFound("CONVERSATION_ARCHIVE_NOT_FOUND", "会话存档不存在")

const (
	conversationArchiveListDefaultPageSize = 20
	conversationArchiveListMaxPageSize     = 100
	conversationArchiveKeywordMaxRunes     = 200
)

// List 分页查询会话存档；PageSize 缺省或超限时会被收敛到安全范围。
func (s *ConversationArchiveService) List(ctx context.Context, filter ConversationArchiveFilter) ([]*ConversationArchive, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, errors.New("会话存档服务不可用")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = conversationArchiveListDefaultPageSize
	}
	if filter.PageSize > conversationArchiveListMaxPageSize {
		filter.PageSize = conversationArchiveListMaxPageSize
	}
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	if len([]rune(filter.Keyword)) > conversationArchiveKeywordMaxRunes {
		filter.Keyword = string([]rune(filter.Keyword)[:conversationArchiveKeywordMaxRunes])
	}
	return s.repo.List(ctx, filter)
}

const (
	conversationArchiveStatsDefaultLimit = 50
	conversationArchiveStatsMaxLimit     = 200
)

// UsageStats 按用户/日期/模型等维度汇总 token 用量；filter 复用列表接口的筛选条件。
func (s *ConversationArchiveService) UsageStats(ctx context.Context, filter ConversationArchiveFilter, groupBy string, limit int, descending bool) ([]*ConversationArchiveAggregate, *ConversationArchiveTotals, error) {
	if s == nil || s.repo == nil {
		return nil, nil, errors.New("会话存档服务不可用")
	}
	groupBy = strings.ToLower(strings.TrimSpace(groupBy))
	switch groupBy {
	case "", ConversationArchiveGroupNone:
		groupBy = ConversationArchiveGroupNone
	case ConversationArchiveGroupUser, ConversationArchiveGroupDate, ConversationArchiveGroupModel,
		ConversationArchiveGroupProtocol, ConversationArchiveGroupGroup, ConversationArchiveGroupAPIKey:
	default:
		return nil, nil, infraerrors.BadRequest("INVALID_GROUP_BY", "不支持的分组维度")
	}
	if limit <= 0 {
		limit = conversationArchiveStatsDefaultLimit
	}
	if limit > conversationArchiveStatsMaxLimit {
		limit = conversationArchiveStatsMaxLimit
	}
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	return s.repo.Stats(ctx, filter, groupBy, limit, descending)
}

// GetByID 读取单条存档（含完整文本内容）。
func (s *ConversationArchiveService) GetByID(ctx context.Context, id int64) (*ConversationArchive, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("会话存档服务不可用")
	}
	if id <= 0 {
		return nil, ErrConversationArchiveNotFound
	}
	record, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrConversationArchiveNotFound
	}
	return record, nil
}

// ConversationArchiveCaptureInput 是中间件交给服务组装归档记录的原始数据。
type ConversationArchiveCaptureInput struct {
	RequestID     string
	UserID        int64
	APIKeyID      int64
	GroupID       *int64
	Protocol      string
	Endpoint      string
	PathModel     string
	StatusCode    int
	ContentType   string
	RequestBytes  []byte
	ResponseBytes []byte
	RequestLimit  int
	ResponseLimit int
	Headers       map[string]string
}

// ConversationArchiveService 负责会话存档的开关判定、异步批量落库与保留期清理。
type ConversationArchiveService struct {
	repo           ConversationArchiveRepository
	settingService *SettingService

	queue chan *ConversationArchive

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	enabledMu     sync.Mutex
	enabledCache  bool
	enabledExpiry time.Time

	droppedCount uint64
	writeFailed  uint64
	writtenCount uint64
}

// NewConversationArchiveService 创建会话存档服务（不启动后台协程）。
func NewConversationArchiveService(repo ConversationArchiveRepository, settingService *SettingService) *ConversationArchiveService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConversationArchiveService{
		repo:           repo,
		settingService: settingService,
		queue:          make(chan *ConversationArchive, conversationArchiveQueueCapacity),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动异步写入与保留期清理协程。
func (s *ConversationArchiveService) Start() {
	if s == nil || s.repo == nil {
		return
	}
	s.wg.Add(2)
	go s.runWriter()
	go s.runRetentionLoop()
}

// Stop 停止服务并尽量落盘队列中剩余记录。
func (s *ConversationArchiveService) Stop() {
	if s == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
}

// Enabled 返回当前是否开启会话存档；使用 2 秒进程内缓存，避免每请求读库。
func (s *ConversationArchiveService) Enabled(ctx context.Context) bool {
	if s == nil || s.repo == nil || s.settingService == nil {
		return false
	}
	s.enabledMu.Lock()
	defer s.enabledMu.Unlock()
	if time.Now().Before(s.enabledExpiry) {
		return s.enabledCache
	}
	loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	enabled := s.settingService.IsConversationArchiveEnabled(loadCtx)
	s.enabledCache = enabled
	s.enabledExpiry = time.Now().Add(2 * time.Second)
	return enabled
}

// Record 非阻塞入队一条归档记录；队列打满时丢弃并计数，绝不阻塞客户端请求。
func (s *ConversationArchiveService) Record(record *ConversationArchive) {
	if s == nil || record == nil || record.ID != 0 {
		return
	}
	select {
	case <-s.ctx.Done():
		return
	default:
	}
	select {
	case s.queue <- record:
	default:
		atomic.AddUint64(&s.droppedCount, 1)
	}
}

// Stats 返回运行时计数，供测试与运维观测使用。
func (s *ConversationArchiveService) Stats() (written, dropped, failed uint64) {
	if s == nil {
		return 0, 0, 0
	}
	return atomic.LoadUint64(&s.writtenCount), atomic.LoadUint64(&s.droppedCount), atomic.LoadUint64(&s.writeFailed)
}

func (s *ConversationArchiveService) runWriter() {
	defer s.wg.Done()

	batch := make([]*ConversationArchive, 0, conversationArchiveBatchSize)
	ticker := time.NewTicker(conversationArchiveFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), 10*time.Second)
		err := s.repo.CreateBatch(ctx, batch)
		cancel()
		if err != nil {
			atomic.AddUint64(&s.writeFailed, uint64(len(batch)))
			logger.L().Warn("conversation archive batch write failed",
				zap.Int("count", len(batch)), zap.Error(err))
		} else {
			atomic.AddUint64(&s.writtenCount, uint64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-s.ctx.Done():
			// 关停时先把已入队记录排空再落盘，避免进程退出丢数据。
			for {
				select {
				case record := <-s.queue:
					batch = append(batch, record)
					if len(batch) >= conversationArchiveBatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		case record := <-s.queue:
			batch = append(batch, record)
			if len(batch) >= conversationArchiveBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// runRetentionLoop 按保留期定期删除过期存档；多实例并发删除幂等。
func (s *ConversationArchiveService) runRetentionLoop() {
	defer s.wg.Done()

	startupTimer := time.NewTimer(conversationArchiveRetentionStartupDelay)
	defer startupTimer.Stop()
	select {
	case <-s.ctx.Done():
		return
	case <-startupTimer.C:
	}

	ticker := time.NewTicker(conversationArchiveRetentionInterval)
	defer ticker.Stop()

	s.runRetentionOnce()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runRetentionOnce()
		}
	}
}

func (s *ConversationArchiveService) runRetentionOnce() {
	if s.settingService == nil {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute)
	defer cancel()

	days := s.settingService.GetConversationArchiveRetentionDays(ctx)
	if days <= 0 {
		return // <=0 表示永久保留
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	for {
		deleted, err := s.repo.DeleteBefore(ctx, cutoff, conversationArchiveRetentionBatchSize)
		if err != nil {
			logger.L().Warn("conversation archive retention cleanup failed", zap.Error(err))
			return
		}
		if deleted == 0 {
			return
		}
	}
}

// BoundedBuffer 是带上限的写入缓冲；超过上限后继续接受写入但不再保存，并标记 truncated。
type BoundedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

// NewBoundedBuffer 创建带上限的缓冲。
func NewBoundedBuffer(limit int) *BoundedBuffer {
	if limit <= 0 {
		limit = 1 << 20
	}
	return &BoundedBuffer{limit: limit}
}

// Write 实现 io.Writer。
func (b *BoundedBuffer) Write(p []byte) {
	if b == nil || len(p) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.truncated = true
		return
	}
	_, _ = b.buf.Write(p)
}

// Bytes 返回缓冲中的字节副本。
func (b *BoundedBuffer) Bytes() []byte {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

// Len 返回当前缓冲字节数。
func (b *BoundedBuffer) Len() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

// Truncated 报告是否发生过截断。
func (b *BoundedBuffer) Truncated() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}

// TeeReadCloser 在读取请求体的同时把内容复制到 BoundedBuffer。
type TeeReadCloser struct {
	source io.ReadCloser
	sink   *BoundedBuffer
}

// NewTeeReadCloser 包装请求体读取。
func NewTeeReadCloser(source io.ReadCloser, sink *BoundedBuffer) *TeeReadCloser {
	return &TeeReadCloser{source: source, sink: sink}
}

// Read 实现 io.Reader。
func (t *TeeReadCloser) Read(p []byte) (int, error) {
	n, err := t.source.Read(p)
	if n > 0 && t.sink != nil {
		t.sink.Write(p[:n])
	}
	return n, err
}

// Close 实现 io.Closer。
func (t *TeeReadCloser) Close() error {
	if t.source == nil {
		return nil
	}
	return t.source.Close()
}

// ConversationArchiveTarget 判断请求是否属于需要存档的文本会话端点，并返回客户端协议与路径中的模型名。
func ConversationArchiveTarget(method, path string) (protocol string, model string, ok bool) {
	if !strings.EqualFold(method, "POST") {
		return "", "", false
	}
	p := strings.ToLower(strings.TrimSpace(path))
	if p == "" {
		return "", "", false
	}
	if idx := strings.Index(p, "/v1beta/models/"); idx >= 0 {
		rest := p[idx+len("/v1beta/models/"):]
		colon := strings.Index(rest, ":")
		if colon <= 0 {
			return "", "", false
		}
		action := rest[colon+1:]
		if strings.HasPrefix(action, "generatecontent") || strings.HasPrefix(action, "streamgeneratecontent") {
			return ConversationArchiveProtocolGemini, rest[:colon], true
		}
		return "", "", false
	}
	switch {
	case strings.HasSuffix(p, "/chat/completions"):
		return ConversationArchiveProtocolOpenAIChat, "", true
	case strings.HasSuffix(p, "/messages"):
		return ConversationArchiveProtocolAnthropicMessages, "", true
	case strings.HasSuffix(p, "/responses"):
		return ConversationArchiveProtocolOpenAIResponses, "", true
	default:
		return "", "", false
	}
}

// BuildConversationArchive 根据捕获到的请求/响应字节组装归档记录；不满足条件时返回 nil。
func BuildConversationArchive(input ConversationArchiveCaptureInput) *ConversationArchive {
	protocol := strings.TrimSpace(input.Protocol)
	if protocol == "" || len(input.RequestBytes) == 0 {
		return nil
	}
	if input.RequestLimit > 0 && len(input.RequestBytes) >= input.RequestLimit {
		return nil // 请求体超限，放弃存档，避免保存不完整的对话
	}
	messages, bodyModel, ok := ExtractConversationRequest(protocol, input.RequestBytes)
	if !ok || len(messages) == 0 {
		return nil
	}
	stream := strings.Contains(strings.ToLower(input.ContentType), "text/event-stream")
	responseText, respOK := ExtractConversationResponse(protocol, stream, input.ResponseBytes)
	if !respOK {
		responseText = ""
	}
	model := strings.TrimSpace(bodyModel)
	if model == "" {
		model = strings.TrimSpace(input.PathModel)
	}
	if model == "" {
		return nil
	}
	sessionKey := conversationArchiveSessionKey(input.Headers, protocol, input.RequestBytes)
	messages, messagesTruncated := limitConversationArchiveMessages(messages)
	responseText, responseTruncated := truncateRunes(responseText, conversationArchiveMaxResponseRunes)
	tokenUsage := ExtractConversationUsage(protocol, stream, input.ResponseBytes)

	createdAt := time.Now().UTC()
	return &ConversationArchive{
		RequestID:         truncateRunesPlain(input.RequestID, 128),
		UserID:            input.UserID,
		APIKeyID:          input.APIKeyID,
		GroupID:           input.GroupID,
		Protocol:          protocol,
		Endpoint:          truncateRunesPlain(input.Endpoint, 255),
		Model:             truncateRunesPlain(model, 128),
		Stream:            stream,
		StatusCode:        input.StatusCode,
		SessionKey:        sessionKey,
		Messages:          messages,
		ResponseText:      responseText,
		InputTokens:       tokenUsage.InputTokens,
		OutputTokens:      tokenUsage.OutputTokens,
		CachedInputTokens: tokenUsage.CachedInputTokens,
		RequestBytes:      len(input.RequestBytes),
		ResponseBytes:     len(input.ResponseBytes),
		Truncated:         messagesTruncated || responseTruncated || (input.ResponseLimit > 0 && len(input.ResponseBytes) >= input.ResponseLimit),
		CreatedAt:         createdAt,
	}
}

func conversationArchiveSessionKey(headers map[string]string, protocol string, body []byte) string {
	for _, name := range []string{"session_id", "x-session-id", "conversation_id", "x-conversation-id"} {
		if value := strings.TrimSpace(headers[name]); value != "" {
			return truncateRunesPlain(value, conversationArchiveSessionKeyRunes)
		}
	}
	if protocol == ConversationArchiveProtocolOpenAIResponses {
		var payload struct {
			PreviousResponseID string `json:"previous_response_id"`
			Conversation       any    `json:"conversation"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			if payload.PreviousResponseID != "" {
				return truncateRunesPlain(payload.PreviousResponseID, conversationArchiveSessionKeyRunes)
			}
			if conversation, ok := payload.Conversation.(string); ok && conversation != "" {
				return truncateRunesPlain(conversation, conversationArchiveSessionKeyRunes)
			}
		}
	}
	return ""
}

func limitConversationArchiveMessages(messages []ConversationArchiveMessage) ([]ConversationArchiveMessage, bool) {
	truncated := false
	out := make([]ConversationArchiveMessage, 0, len(messages))
	total := 0
	for _, message := range messages {
		content, cut := truncateRunes(message.Content, conversationArchiveMaxMessageRunes)
		if cut {
			truncated = true
		}
		if total+len([]rune(content)) > conversationArchiveMaxMessagesRunes {
			truncated = true
			break
		}
		total += len([]rune(content))
		out = append(out, ConversationArchiveMessage{Role: message.Role, Content: content})
	}
	return out, truncated
}

// truncateRunes 按字符数截断。
func truncateRunes(value string, limit int) (string, bool) {
	if limit <= 0 || value == "" {
		return value, false
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value, false
	}
	return string(runes[:limit]), true
}

func truncateRunesPlain(value string, limit int) string {
	out, _ := truncateRunes(value, limit)
	return out
}
