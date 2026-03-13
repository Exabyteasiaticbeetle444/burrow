package server

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type LogEntry struct {
	Time    time.Time         `json:"time"`
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Attrs   map[string]string `json:"attrs,omitempty"`
}

type LogBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	size    int
	pos     int
	count   int
}

func NewLogBuffer(size int) *LogBuffer {
	if size <= 0 {
		size = 500
	}
	return &LogBuffer{
		entries: make([]LogEntry, size),
		size:    size,
	}
}

func (b *LogBuffer) Add(entry LogEntry) {
	b.mu.Lock()
	b.entries[b.pos] = entry
	b.pos = (b.pos + 1) % b.size
	if b.count < b.size {
		b.count++
	}
	b.mu.Unlock()
}

func (b *LogBuffer) Entries(limit int) []LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if limit <= 0 || limit > b.count {
		limit = b.count
	}

	result := make([]LogEntry, limit)
	start := (b.pos - b.count + b.size) % b.size
	skip := b.count - limit
	start = (start + skip) % b.size

	for i := 0; i < limit; i++ {
		result[i] = b.entries[(start+i)%b.size]
	}
	return result
}

func (b *LogBuffer) Handler() slog.Handler {
	return &logBufferHandler{buf: b}
}

type logBufferHandler struct {
	buf   *LogBuffer
	attrs []slog.Attr
	group string
}

func (h *logBufferHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *logBufferHandler) Handle(_ context.Context, r slog.Record) error {
	entry := LogEntry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
	}

	prefix := h.group

	if len(h.attrs) > 0 || r.NumAttrs() > 0 {
		entry.Attrs = make(map[string]string)
		for _, a := range h.attrs {
			key := a.Key
			if prefix != "" {
				key = prefix + "." + key
			}
			entry.Attrs[key] = a.Value.String()
		}
		r.Attrs(func(a slog.Attr) bool {
			key := a.Key
			if prefix != "" {
				key = prefix + "." + key
			}
			entry.Attrs[key] = a.Value.String()
			return true
		})
	}

	h.buf.Add(entry)
	return nil
}

func (h *logBufferHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &logBufferHandler{
		buf:   h.buf,
		attrs: newAttrs,
		group: h.group,
	}
}

func (h *logBufferHandler) WithGroup(name string) slog.Handler {
	g := h.group
	if g != "" {
		g += "." + name
	} else {
		g = name
	}
	return &logBufferHandler{
		buf:   h.buf,
		attrs: h.attrs,
		group: g,
	}
}

type multiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}
