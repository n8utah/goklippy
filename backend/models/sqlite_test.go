package models

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schemaPath = "../../database/schema.sql"

func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := OpenDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, ApplySchemaFile(context.Background(), db, schemaPath))
	return db
}

func TestSchemaAppliesTwice(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, ApplySchemaFile(context.Background(), db, schemaPath), "every statement must be repeatable")
}

func TestOpenDBCreatesFile(t *testing.T) {
	path := t.TempDir() + "/nested/goklippy.db"
	db, err := OpenDB(path)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, ApplySchemaFile(context.Background(), db, schemaPath))
	var mode string
	require.NoError(t, db.Get(&mode, "PRAGMA journal_mode"))
	assert.Equal(t, "wal", mode)
}

func TestAuthStoreRoundTrip(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	seed := InitialUserSeed{Enabled: true, Email: "Admin@Example.com", Password: "pw", Role: "admin"}

	created, err := EnsureInitialUser(ctx, db, seed)
	require.NoError(t, err)
	assert.True(t, created)
	created, err = EnsureInitialUser(ctx, db, seed)
	require.NoError(t, err)
	assert.False(t, created, "same user again changes nothing")
	seed.Role = "user"
	created, err = EnsureInitialUser(ctx, db, seed)
	require.NoError(t, err)
	assert.True(t, created, "a changed role updates the user")

	store := NewAuthStore(db)
	user, err := store.GetUserByEmail(ctx, "admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, "user", user.Role)

	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, store.SaveRefreshToken(ctx, "tok", user.ID, expires))
	rec, err := store.GetRefreshToken(ctx, "tok")
	require.NoError(t, err)
	assert.Equal(t, user.ID, rec.UserID)
	assert.True(t, expires.Equal(rec.ExpiresAt), "expires %v, read back %v", expires, rec.ExpiresAt)
	assert.Nil(t, rec.RevokedAt)

	later := expires.Add(time.Hour)
	require.NoError(t, store.SaveRefreshToken(ctx, "tok", user.ID, later))
	rec, err = store.GetRefreshToken(ctx, "tok")
	require.NoError(t, err)
	assert.True(t, later.Equal(rec.ExpiresAt))

	require.NoError(t, store.RevokeRefreshToken(ctx, "tok"))
	rec, err = store.GetRefreshToken(ctx, "tok")
	require.NoError(t, err)
	require.NotNil(t, rec.RevokedAt)
	assert.WithinDuration(t, time.Now(), *rec.RevokedAt, time.Minute)
}

func TestSorterSettingsStore(t *testing.T) {
	store := NewSorterSettingsStore(openTestDB(t))
	ctx := context.Background()

	_, ok, err := store.GetSorterSettings(ctx, "machine")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, store.PutSorterSettings(ctx, "machine", json.RawMessage(`{"indexRpm": 10}`)))
	require.NoError(t, store.PutSorterSettings(ctx, "machine", json.RawMessage(`{"indexRpm": 12}`)))
	data, ok, err := store.GetSorterSettings(ctx, "machine")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.JSONEq(t, `{"indexRpm": 12}`, string(data))

	assert.Error(t, store.PutSorterSettings(ctx, "machine", json.RawMessage(`{broken`)), "the table only takes valid JSON")
}
