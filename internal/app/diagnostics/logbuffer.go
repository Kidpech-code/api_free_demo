package diagnostics

import "sync"

// LogBuffer captures recent log lines for debug endpoints.
type LogBuffer struct {
	mu   sync.RWMutex
	data []string
	cap  int
}

// NewLogBuffer builds buffer.
func NewLogBuffer(limit int) *LogBuffer {
	if limit <= 0 {
		limit = 100
	}
	return &LogBuffer{cap: limit, data: make([]string, 0, limit)}
}

// Append stores new log line.
func (b *LogBuffer) Append(entry string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) >= b.cap {
		b.data = b.data[1:]
	}
	b.data = append(b.data, entry)
}

// Snapshot returns copy of log data.
func (b *LogBuffer) Snapshot() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, len(b.data))
	copy(out, b.data)
	return out
}
