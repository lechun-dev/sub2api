package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiKeyLookupUserRepo struct {
	UserRepository
	user      *User
	err       error
	seenEmail string
}

func (r *apiKeyLookupUserRepo) GetByEmail(_ context.Context, email string) (*User, error) {
	r.seenEmail = email
	return r.user, r.err
}

type apiKeyLookupKeyRepo struct {
	APIKeyRepository
	keys       []APIKey
	err        error
	seenUserID int64
	seenStatus string
}

func (r *apiKeyLookupKeyRepo) ListAllByUserID(_ context.Context, userID int64, filters APIKeyListFilters) ([]APIKey, error) {
	r.seenUserID = userID
	r.seenStatus = filters.Status
	return r.keys, r.err
}

func TestAdminFindAPIKeysByEmail(t *testing.T) {
	t.Parallel()

	userRepo := &apiKeyLookupUserRepo{user: &User{ID: 7, Email: "zhang@example.com", Username: "张三"}}
	keyRepo := &apiKeyLookupKeyRepo{keys: []APIKey{{ID: 11, UserID: 7, Name: "default"}}}
	svc := &adminServiceImpl{userRepo: userRepo, apiKeyRepo: keyRepo}

	user, keys, err := svc.FindAPIKeysByEmail(context.Background(), "  zhang@example.com  ", "active")
	require.NoError(t, err)
	require.Equal(t, int64(7), user.ID)
	require.Len(t, keys, 1)
	require.Equal(t, "zhang@example.com", userRepo.seenEmail)
	require.Equal(t, int64(7), keyRepo.seenUserID)
	require.Equal(t, "active", keyRepo.seenStatus)
}

func TestAdminFindAPIKeysByEmailRequiresEmail(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{userRepo: &apiKeyLookupUserRepo{}, apiKeyRepo: &apiKeyLookupKeyRepo{}}
	_, _, err := svc.FindAPIKeysByEmail(context.Background(), "   ", "")
	require.Error(t, err)
}

func TestAdminFindAPIKeysByEmailUserNotFound(t *testing.T) {
	t.Parallel()

	userRepo := &apiKeyLookupUserRepo{err: ErrUserNotFound}
	svc := &adminServiceImpl{userRepo: userRepo, apiKeyRepo: &apiKeyLookupKeyRepo{}}
	_, _, err := svc.FindAPIKeysByEmail(context.Background(), "missing@example.com", "")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestAdminFindAPIKeysByEmailRepoError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	userRepo := &apiKeyLookupUserRepo{user: &User{ID: 7}}
	keyRepo := &apiKeyLookupKeyRepo{err: boom}
	svc := &adminServiceImpl{userRepo: userRepo, apiKeyRepo: keyRepo}
	_, _, err := svc.FindAPIKeysByEmail(context.Background(), "zhang@example.com", "")
	require.ErrorIs(t, err, boom)
}

func TestAdminFindAPIKeysByEmail(t *testing.T) {
	t.Parallel()

	userRepo := &apiKeyLookupUserRepo{user: &User{ID: 7, Email: "zhang@example.com", Username: "张三"}}
	keyRepo := &apiKeyLookupKeyRepo{keys: []APIKey{{ID: 11, UserID: 7, Name: "default"}}}
	svc := &adminServiceImpl{userRepo: userRepo, apiKeyRepo: keyRepo}

	user, keys, err := svc.FindAPIKeysByEmail(context.Background(), "  zhang@example.com  ", "active")
	require.NoError(t, err)
	require.Equal(t, int64(7), user.ID)
	require.Len(t, keys, 1)
	require.Equal(t, "zhang@example.com", userRepo.seenEmail)
	require.Equal(t, int64(7), keyRepo.seenUserID)
	require.Equal(t, "active", keyRepo.seenStatus)
}

func TestAdminFindAPIKeysByEmailRequiresEmail(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{userRepo: &apiKeyLookupUserRepo{}, apiKeyRepo: &apiKeyLookupKeyRepo{}}
	_, _, err := svc.FindAPIKeysByEmail(context.Background(), "   ", "")
	require.Error(t, err)
}

func TestAdminFindAPIKeysByEmailUserNotFound(t *testing.T) {
	t.Parallel()

	userRepo := &apiKeyLookupUserRepo{err: ErrUserNotFound}
	svc := &adminServiceImpl{userRepo: userRepo, apiKeyRepo: &apiKeyLookupKeyRepo{}}
	_, _, err := svc.FindAPIKeysByEmail(context.Background(), "missing@example.com", "")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestAdminFindAPIKeysByEmailRepoError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	userRepo := &apiKeyLookupUserRepo{user: &User{ID: 7}}
	keyRepo := &apiKeyLookupKeyRepo{err: boom}
	svc := &adminServiceImpl{userRepo: userRepo, apiKeyRepo: keyRepo}
	_, _, err := svc.FindAPIKeysByEmail(context.Background(), "zhang@example.com", "")
	require.ErrorIs(t, err, boom)
}

func TestAdminFindUserByEmail(t *testing.T) {
	t.Parallel()

	userRepo := &apiKeyLookupUserRepo{user: &User{ID: 7, Email: "zhang@example.com", Role: RoleAdmin}}
	svc := &adminServiceImpl{userRepo: userRepo}

	user, err := svc.FindUserByEmail(context.Background(), "  zhang@example.com  ")
	require.NoError(t, err)
	require.Equal(t, int64(7), user.ID)
	require.True(t, user.IsAdmin())
	require.Equal(t, "zhang@example.com", userRepo.seenEmail)
}

func TestAdminFindUserByEmailRequiresEmail(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{userRepo: &apiKeyLookupUserRepo{}}
	_, err := svc.FindUserByEmail(context.Background(), "  ")
	require.Error(t, err)
}

func TestAdminFindUserByEmailNotFound(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{userRepo: &apiKeyLookupUserRepo{err: ErrUserNotFound}}
	_, err := svc.FindUserByEmail(context.Background(), "missing@example.com")
	require.ErrorIs(t, err, ErrUserNotFound)
}
