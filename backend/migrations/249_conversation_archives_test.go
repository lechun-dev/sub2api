package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConversationArchivesMigration(t *testing.T) {
	content, err := FS.ReadFile("249_conversation_archives.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS conversation_archives")
	require.Contains(t, sql, "request_messages JSONB NOT NULL DEFAULT '[]'::jsonb")
	require.Contains(t, sql, "response_text TEXT NOT NULL DEFAULT ''")
	require.Contains(t, sql, "truncated BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_conversation_archives_created_at")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_conversation_archives_user_created")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_conversation_archives_model_created")
	require.NotContains(t, sql, "FOREIGN KEY")
}
