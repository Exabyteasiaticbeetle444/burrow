package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	s, err := NewSQLite(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestClientCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	c := &Client{
		ID:        "test-id",
		Name:      "Test User",
		Token:     "test-token",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	if err := s.CreateClient(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.GetClient(ctx, "test-id")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("client not found")
	}
	if got.Name != "Test User" {
		t.Errorf("name: got %q, want %q", got.Name, "Test User")
	}

	gotByToken, err := s.GetClientByToken(ctx, "test-token")
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if gotByToken == nil || gotByToken.ID != "test-id" {
		t.Error("get by token failed")
	}

	clients, err := s.ListClients(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(clients) != 1 {
		t.Errorf("list count: got %d, want 1", len(clients))
	}

	if err := s.RevokeClient(ctx, "test-id"); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	revoked, err := s.GetClientByToken(ctx, "test-token")
	if err != nil {
		t.Fatalf("get revoked: %v", err)
	}
	if revoked != nil {
		t.Error("revoked client should not be returned by GetClientByToken")
	}
}

func TestActiveTokens(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.CreateClient(ctx, &Client{ID: "a", Name: "A", Token: "token-a", CreatedAt: time.Now()})
	s.CreateClient(ctx, &Client{ID: "b", Name: "B", Token: "token-b", CreatedAt: time.Now()})
	s.RevokeClient(ctx, "b")

	tokens, err := s.ListActiveTokens(ctx)
	if err != nil {
		t.Fatalf("list active tokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "token-a" {
		t.Errorf("active tokens: got %v, want [token-a]", tokens)
	}
}

func TestStats(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.CreateClient(ctx, &Client{ID: "a", Name: "A", Token: "ta", CreatedAt: time.Now()})
	s.CreateClient(ctx, &Client{ID: "b", Name: "B", Token: "tb", CreatedAt: time.Now()})
	s.RevokeClient(ctx, "b")

	stats, err := s.GetStats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalClients != 2 {
		t.Errorf("total: got %d, want 2", stats.TotalClients)
	}
	if stats.ActiveClients != 1 {
		t.Errorf("active: got %d, want 1", stats.ActiveClients)
	}
	if stats.RevokedClients != 1 {
		t.Errorf("revoked: got %d, want 1", stats.RevokedClients)
	}
}

func TestConfig(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	val, err := s.GetConfig(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if val != "" {
		t.Errorf("missing key should return empty, got %q", val)
	}

	if err := s.SetConfig(ctx, "key1", "value1"); err != nil {
		t.Fatalf("set: %v", err)
	}

	val, err = s.GetConfig(ctx, "key1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if val != "value1" {
		t.Errorf("got %q, want %q", val, "value1")
	}

	if err := s.SetConfig(ctx, "key1", "value2"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	val, _ = s.GetConfig(ctx, "key1")
	if val != "value2" {
		t.Errorf("upsert: got %q, want %q", val, "value2")
	}
}

func TestConnections(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.CreateClient(ctx, &Client{ID: "c1", Name: "C1", Token: "t1", CreatedAt: time.Now()})

	conn := &Connection{
		ClientID:    "c1",
		ConnectedAt: time.Now(),
		Protocol:    "vless",
	}
	if err := s.CreateConnection(ctx, conn); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if conn.ID == 0 {
		t.Error("connection ID should be set")
	}

	if err := s.CloseConnection(ctx, conn.ID, 1024, 2048); err != nil {
		t.Fatalf("close connection: %v", err)
	}

	conns, err := s.GetClientConnections(ctx, "c1", 10)
	if err != nil {
		t.Fatalf("get connections: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("connections count: got %d, want 1", len(conns))
	}
	if conns[0].BytesUp != 1024 || conns[0].BytesDown != 2048 {
		t.Errorf("bytes: got %d/%d, want 1024/2048", conns[0].BytesUp, conns[0].BytesDown)
	}
}

func TestSQLiteFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file should exist after close")
	}
}
