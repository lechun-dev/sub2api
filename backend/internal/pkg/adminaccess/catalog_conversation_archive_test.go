package adminaccess

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConversationArchivePermission(t *testing.T) {
	t.Parallel()

	policy := Policy{GlobalPermissions: []string{"conversation_archive.read"}}
	require.True(t, RouteAllowed(policy, "GET", "/api/v1/admin/conversation-archives"))
	require.True(t, RouteAllowed(policy, "GET", "/api/v1/admin/conversation-archives/:id"))

	require.False(t, RouteAllowed(Policy{}, "GET", "/api/v1/admin/conversation-archives"))
	require.False(t, RouteAllowed(Policy{GlobalPermissions: []string{"users.read"}}, "GET", "/api/v1/admin/conversation-archives"))
	require.False(t, RouteAllowed(policy, "POST", "/api/v1/admin/conversation-archives"))
	require.False(t, RouteAllowed(policy, "GET", "/api/v1/admin/conversation-archives/export"))
}

func TestConversationArchivePermissionCatalogEntry(t *testing.T) {
	t.Parallel()

	found := false
	for _, permission := range PermissionCatalog() {
		if permission.ID != "conversation_archive.read" {
			continue
		}
		found = true
		require.Equal(t, "global", permission.Scope)
		require.Equal(t, "会话", permission.Category)
	}
	require.True(t, found, "conversation_archive.read must be registered in the permission catalog")
}
