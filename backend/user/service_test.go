package user

import (
	"goaway/backend/database"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestUserService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.User{}))
	return NewService(NewRepository(db))
}

func TestUserLifecycleAndAuth(t *testing.T) {
	svc := newTestUserService(t)

	require.NoError(t, svc.CreateUser("alice", "s3cr3t!", ""))
	assert.True(t, svc.Exists("alice"))
	assert.False(t, svc.Exists("bob"))

	u, err := svc.GetUser("alice")
	require.NoError(t, err)
	assert.Equal(t, "admin", u.Role, "empty role should default to admin")

	assert.True(t, svc.Authenticate("alice", "s3cr3t!"))
	assert.False(t, svc.Authenticate("alice", "wrong"))
	assert.False(t, svc.Authenticate("nobody", "s3cr3t!"))

	require.NoError(t, svc.UpdatePassword("alice", "new-pass-42"))
	assert.False(t, svc.Authenticate("alice", "s3cr3t!"))
	assert.True(t, svc.Authenticate("alice", "new-pass-42"))

	users, err := svc.GetAllUsers()
	require.NoError(t, err)
	assert.Len(t, users, 1)

	require.NoError(t, svc.DeleteUser("alice"))
	assert.False(t, svc.Exists("alice"))
}

func TestDeleteDefaultAdminRefused(t *testing.T) {
	svc := newTestUserService(t)
	err := svc.DeleteUser("admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete default admin")
}

func TestValidateCredentials(t *testing.T) {
	svc := newTestUserService(t)

	assert.NoError(t, svc.ValidateCredentials(User{Username: "alice", Password: "password123"}))
	assert.Error(t, svc.ValidateCredentials(User{Username: "  ", Password: "password123"}))
	assert.Error(t, svc.ValidateCredentials(User{Username: "alice", Password: ""}))
	assert.Error(t, svc.ValidateCredentials(User{Username: strings.Repeat("a", 61), Password: "ok"}))
	assert.Error(t, svc.ValidateCredentials(User{Username: "alice", Password: strings.Repeat("p", 121)}))
	assert.Error(t, svc.ValidateCredentials(User{Username: "bad\x00user", Password: "ok"}))
}
