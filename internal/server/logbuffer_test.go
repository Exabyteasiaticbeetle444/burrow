package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestLogBufferAddAndRetrieve(t *testing.T) {
	buf := NewLogBuffer(10)

	buf.Add(LogEntry{Time: time.Now(), Level: "INFO", Message: "first"})
	buf.Add(LogEntry{Time: time.Now(), Level: "ERROR", Message: "second"})

	entries := buf.Entries(0)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Message != "first" {
		t.Errorf("expected first entry message 'first', got %q", entries[0].Message)
	}
	if entries[1].Message != "second" {
		t.Errorf("expected second entry message 'second', got %q", entries[1].Message)
	}
}

func TestLogBufferLimit(t *testing.T) {
	buf := NewLogBuffer(10)

	for i := 0; i < 5; i++ {
		buf.Add(LogEntry{Time: time.Now(), Level: "INFO", Message: "msg"})
	}

	entries := buf.Entries(3)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestLogBufferOverflow(t *testing.T) {
	buf := NewLogBuffer(3)

	buf.Add(LogEntry{Time: time.Now(), Level: "INFO", Message: "a"})
	buf.Add(LogEntry{Time: time.Now(), Level: "INFO", Message: "b"})
	buf.Add(LogEntry{Time: time.Now(), Level: "INFO", Message: "c"})
	buf.Add(LogEntry{Time: time.Now(), Level: "INFO", Message: "d"})
	buf.Add(LogEntry{Time: time.Now(), Level: "INFO", Message: "e"})

	entries := buf.Entries(0)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Message != "c" {
		t.Errorf("expected oldest entry 'c', got %q", entries[0].Message)
	}
	if entries[1].Message != "d" {
		t.Errorf("expected middle entry 'd', got %q", entries[1].Message)
	}
	if entries[2].Message != "e" {
		t.Errorf("expected newest entry 'e', got %q", entries[2].Message)
	}
}

func TestLogBufferThreadSafety(t *testing.T) {
	buf := NewLogBuffer(100)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				buf.Add(LogEntry{Time: time.Now(), Level: "INFO", Message: "concurrent"})
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				buf.Entries(10)
			}
		}()
	}

	wg.Wait()

	entries := buf.Entries(0)
	if len(entries) != 100 {
		t.Errorf("expected 100 entries (buffer full), got %d", len(entries))
	}
}

func TestLogBufferSlogHandler(t *testing.T) {
	buf := NewLogBuffer(10)
	logger := slog.New(buf.Handler())

	logger.Info("hello", "key", "value")
	logger.Error("failure", "code", "500")

	entries := buf.Entries(0)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Level != "INFO" {
		t.Errorf("expected level INFO, got %q", entries[0].Level)
	}
	if entries[0].Message != "hello" {
		t.Errorf("expected message 'hello', got %q", entries[0].Message)
	}
	if entries[0].Attrs["key"] != "value" {
		t.Errorf("expected attr key=value, got %q", entries[0].Attrs["key"])
	}

	if entries[1].Level != "ERROR" {
		t.Errorf("expected level ERROR, got %q", entries[1].Level)
	}
	if entries[1].Attrs["code"] != "500" {
		t.Errorf("expected attr code=500, got %q", entries[1].Attrs["code"])
	}
}

func TestLogBufferSlogHandlerWithGroup(t *testing.T) {
	buf := NewLogBuffer(10)
	logger := slog.New(buf.Handler()).WithGroup("server")

	logger.Info("started", "port", "8080")

	entries := buf.Entries(0)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Attrs["server.port"] != "8080" {
		t.Errorf("expected grouped attr server.port=8080, got attrs: %v", entries[0].Attrs)
	}
}

func TestLogBufferEmpty(t *testing.T) {
	buf := NewLogBuffer(10)
	entries := buf.Entries(0)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries from empty buffer, got %d", len(entries))
	}
}

func TestLogBufferDefaultSize(t *testing.T) {
	buf := NewLogBuffer(0)
	if buf.size != 500 {
		t.Errorf("expected default size 500, got %d", buf.size)
	}
}

func TestMultiHandler(t *testing.T) {
	buf1 := NewLogBuffer(10)
	buf2 := NewLogBuffer(10)
	multi := NewMultiHandler(buf1.Handler(), buf2.Handler())
	logger := slog.New(multi)

	logger.Info("test message")

	e1 := buf1.Entries(0)
	e2 := buf2.Entries(0)
	if len(e1) != 1 {
		t.Errorf("buf1: expected 1 entry, got %d", len(e1))
	}
	if len(e2) != 1 {
		t.Errorf("buf2: expected 1 entry, got %d", len(e2))
	}
}

func TestHandleLogsEndpoint(t *testing.T) {
	api, auth, _ := setupTestAPI(t)
	logBuf := NewLogBuffer(500)
	api.logBuffer = logBuf
	router := api.Router()
	token := authToken(t, auth)

	logBuf.Add(LogEntry{
		Time:    time.Now(),
		Level:   "INFO",
		Message: "test log entry",
		Attrs:   map[string]string{"key": "value"},
	})
	logBuf.Add(LogEntry{
		Time:    time.Now(),
		Level:   "ERROR",
		Message: "error log entry",
	})

	rec := doRequest(t, router, "GET", "/api/logs", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var entries []LogEntry
	decodeJSON(t, rec, &entries)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Message != "test log entry" {
		t.Errorf("expected 'test log entry', got %q", entries[0].Message)
	}
}

func TestHandleLogsEndpointWithLimit(t *testing.T) {
	api, auth, _ := setupTestAPI(t)
	logBuf := NewLogBuffer(500)
	api.logBuffer = logBuf
	router := api.Router()
	token := authToken(t, auth)

	for i := 0; i < 10; i++ {
		logBuf.Add(LogEntry{Time: time.Now(), Level: "INFO", Message: "msg"})
	}

	rec := doRequest(t, router, "GET", "/api/logs?limit=5", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var entries []LogEntry
	decodeJSON(t, rec, &entries)
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
}

func TestHandleLogsEndpointMaxLimit(t *testing.T) {
	api, auth, _ := setupTestAPI(t)
	logBuf := NewLogBuffer(500)
	api.logBuffer = logBuf
	router := api.Router()
	token := authToken(t, auth)

	rec := doRequest(t, router, "GET", "/api/logs?limit=9999", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var entries []LogEntry
	decodeJSON(t, rec, &entries)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (empty buffer), got %d", len(entries))
	}
}

func TestHandleLogsEndpointRequiresAuth(t *testing.T) {
	api, _, _ := setupTestAPI(t)
	api.logBuffer = NewLogBuffer(500)
	router := api.Router()

	rec := doRequest(t, router, "GET", "/api/logs", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleLogsEndpointInvalidLimit(t *testing.T) {
	api, auth, _ := setupTestAPI(t)
	logBuf := NewLogBuffer(500)
	api.logBuffer = logBuf
	router := api.Router()
	token := authToken(t, auth)

	rec := doRequest(t, router, "GET", "/api/logs?limit=notanumber", nil, token)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleLogsEndpointNoBuffer(t *testing.T) {
	api, auth, _ := setupTestAPI(t)
	router := api.Router()
	token := authToken(t, auth)

	rec := doRequest(t, router, "GET", "/api/logs", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var entries []LogEntry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries when no buffer, got %d", len(entries))
	}
}
