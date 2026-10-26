package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestConversationArchiveRepositoryCreateBatch(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := NewConversationArchiveRepository(db)

	mock.ExpectExec("INSERT INTO conversation_archives").
		WillReturnResult(sqlmock.NewResult(1, 2))

	err := repo.CreateBatch(context.Background(), []*service.ConversationArchive{
		{
			RequestID: "req-1",
			UserID:    7,
			APIKeyID:  9,
			Protocol:  service.ConversationArchiveProtocolAnthropicMessages,
			Endpoint:  "/v1/messages",
			Model:     "claude",
			Messages: []service.ConversationArchiveMessage{
				{Role: "user", Content: "hi"},
			},
			ResponseText: "hello",
			CreatedAt:    time.Now().UTC(),
		},
		{
			RequestID: "req-2",
			Protocol:  service.ConversationArchiveProtocolOpenAIChat,
			Endpoint:  "/v1/chat/completions",
			Model:     "gpt-5",
			Stream:    true,
		},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationArchiveRepositoryCreateBatchEmpty(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := NewConversationArchiveRepository(db)
	require.NoError(t, repo.CreateBatch(context.Background(), nil))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationArchiveRepositoryDeleteBefore(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := NewConversationArchiveRepository(db)

	cutoff := time.Now().UTC().AddDate(0, 0, -90)
	mock.ExpectExec("DELETE FROM conversation_archives").
		WithArgs(cutoff, 5000).
		WillReturnResult(sqlmock.NewResult(0, 3))

	deleted, err := repo.DeleteBefore(context.Background(), cutoff, 5000)
	require.NoError(t, err)
	require.Equal(t, int64(3), deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func conversationArchiveListRowColumns(includeContent bool) []string {
	columns := []string{
		"id", "request_id", "user_id", "username", "email", "api_key_id", "group_id", "protocol", "endpoint", "model",
		"stream", "status_code", "session_key", "message_count", "response_chars", "truncated", "created_at",
	}
	if includeContent {
		columns = append(columns, "request_messages", "response_text")
	}
	return columns
}

func TestConversationArchiveRepositoryListFilters(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := NewConversationArchiveRepository(db)

	userID := int64(7)
	filter := service.ConversationArchiveFilter{
		Page:       1,
		PageSize:   20,
		UserID:     &userID,
		Model:      "claude",
		Keyword:    "hello",
		Descending: true,
	}

	mock.ExpectQuery(`SELECT count\(\*\) FROM conversation_archives ca WHERE ca.user_id = \$1 AND ca.model = \$2`).
		WithArgs(int64(7), "claude", "%hello%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	createdAt := time.Now().UTC()
	rows := sqlmock.NewRows(conversationArchiveListRowColumns(false)).
		AddRow(1, "req-1", int64(7), "张三", "zhang@example.com", int64(9), nil, "anthropic_messages", "/v1/messages", "claude",
			false, 200, "session-1", 3, 120, false, createdAt)
	mock.ExpectQuery(`SELECT ca.id, ca.request_id, ca.user_id, COALESCE\(u.username`).
		WithArgs(int64(7), "claude", "%hello%", 20, 0).
		WillReturnRows(rows)

	records, total, err := repo.List(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	require.Equal(t, int64(1), records[0].ID)
	require.Equal(t, int64(7), records[0].UserID)
	require.Equal(t, "张三", records[0].Username)
	require.Equal(t, "zhang@example.com", records[0].Email)
	require.Equal(t, int64(9), records[0].APIKeyID)
	require.Nil(t, records[0].GroupID)
	require.Equal(t, "session-1", records[0].SessionKey)
	require.Equal(t, 3, records[0].MessageCount)
	require.Equal(t, 120, records[0].ResponseChars)
	require.Empty(t, records[0].Messages)
	require.Empty(t, records[0].ResponseText)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationArchiveRepositoryListWithContent(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := NewConversationArchiveRepository(db)

	filter := service.ConversationArchiveFilter{
		Page:           1,
		PageSize:       10,
		IncludeContent: true,
	}
	mock.ExpectQuery(`SELECT count\(\*\) FROM conversation_archives ca$`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	rows := sqlmock.NewRows(conversationArchiveListRowColumns(true)).
		AddRow(2, "req-2", nil, "", "", nil, nil, "openai_chat", "/v1/chat/completions", "gpt-5",
			true, 200, nil, 1, 12, false, time.Now().UTC(),
			[]byte(`[{"role":"user","content":"hi"}]`), "hello")
	mock.ExpectQuery(`SELECT ca.id, ca.request_id`).
		WithArgs(10, 0).
		WillReturnRows(rows)

	records, total, err := repo.List(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	require.True(t, records[0].Stream)
	require.Len(t, records[0].Messages, 1)
	require.Equal(t, "hi", records[0].Messages[0].Content)
	require.Equal(t, "hello", records[0].ResponseText)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationArchiveRepositoryGetByID(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := NewConversationArchiveRepository(db)

	rows := sqlmock.NewRows(conversationArchiveListRowColumns(true)).
		AddRow(3, "req-3", int64(7), "李四", "li@example.com", int64(9), int64(2), "gemini", "/v1beta/models/gemini-2.5-pro:generateContent",
			"gemini-2.5-pro", false, 200, nil, 2, 8, false, time.Now().UTC(),
			[]byte(`[{"role":"user","content":"hi"}]`), "hello")
	mock.ExpectQuery(`SELECT ca.id, ca.request_id`).
		WithArgs(int64(3)).
		WillReturnRows(rows)

	record, err := repo.GetByID(context.Background(), 3)
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, int64(3), record.ID)
	require.NotNil(t, record.GroupID)
	require.Equal(t, int64(2), *record.GroupID)
	require.Equal(t, "hello", record.ResponseText)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationArchiveRepositoryGetByIDNotFound(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := NewConversationArchiveRepository(db)

	mock.ExpectQuery(`SELECT ca.id, ca.request_id`).WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows(conversationArchiveListRowColumns(true)))

	record, err := repo.GetByID(context.Background(), 99)
	require.NoError(t, err)
	require.Nil(t, record)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEscapeConversationArchiveLike(t *testing.T) {
	t.Parallel()

	require.Equal(t, `100\%\_test\\`, escapeConversationArchiveLike(`100%_test\`))
	require.Equal(t, "plain", escapeConversationArchiveLike("plain"))
}

func TestConversationArchiveRepositoryListByUsername(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := NewConversationArchiveRepository(db)

	filter := service.ConversationArchiveFilter{
		Page:     1,
		PageSize: 10,
		UserName: "张三",
	}

	mock.ExpectQuery(`SELECT count\(\*\) FROM conversation_archives ca LEFT JOIN users u ON u.id = ca.user_id WHERE \(u.username ILIKE \$1`).
		WithArgs("%张三%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	rows := sqlmock.NewRows(conversationArchiveListRowColumns(false)).
		AddRow(4, "req-4", int64(7), "张三", "zhang@example.com", int64(9), nil, "anthropic_messages", "/v1/messages", "claude",
			false, 200, nil, 1, 5, false, time.Now().UTC())
	mock.ExpectQuery(`SELECT ca.id, ca.request_id`).
		WithArgs("%张三%", 10, 0).
		WillReturnRows(rows)

	records, total, err := repo.List(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	require.Equal(t, "张三", records[0].Username)
	require.NoError(t, mock.ExpectationsWereMet())
}
