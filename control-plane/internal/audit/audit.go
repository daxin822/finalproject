package audit

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Entry 为一次写操作或敏感读的审计记录（内存环形缓冲）。
type Entry struct {
	Time   time.Time `json:"time"`
	Actor  string    `json:"actor"`
	Role   string    `json:"role"`
	Method string    `json:"method"`
	Path   string    `json:"path"`
	Status int       `json:"status"`
	Detail string    `json:"detail,omitempty"`
}

// Ring 固定保留最近 max 条。
type Ring struct {
	mu      sync.Mutex
	max     int
	entries []Entry
}

func New(max int) *Ring {
	if max < 8 {
		max = 8
	}
	return &Ring{max: max}
}

func (r *Ring) Append(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	if len(r.entries) > r.max {
		r.entries = r.entries[len(r.entries)-r.max:]
	}
}

// Recent 返回最新 n 条（时间降序）。
func (r *Ring) Recent(n int) []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || len(r.entries) == 0 {
		return nil
	}
	start := len(r.entries) - n
	if start < 0 {
		start = 0
	}
	out := make([]Entry, 0, len(r.entries)-start)
	for i := len(r.entries) - 1; i >= start; i-- {
		out = append(out, r.entries[i])
	}
	return out
}

// ResponseWriter 包装以捕获 HTTP 状态码。
type ResponseWriter struct {
	http.ResponseWriter
	Status int
}

func (w *ResponseWriter) WriteHeader(code int) {
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	if w.Status == 0 {
		w.Status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func DetailJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	if len(b) > 512 {
		return string(b[:512]) + "..."
	}
	return string(b)
}
