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
