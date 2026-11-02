package common

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const relayTraceKey = "relay_timing_trace"

var relayTraceIDPattern = regexp.MustCompile(`^(?:(?:client:)?[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|(?:msg_|req_)[a-zA-Z0-9_-]{1,100}|[0-9a-fA-F]{16,32}-[A-Z]{3})$`)

// RelayTrace stores metadata only. All times are relative to gateway ingress,
// including per-attempt times, so retries cannot overwrite an earlier attempt.
type RelayTrace struct {
	mu       sync.Mutex
	start    time.Time
	fields   map[string]any
	attempts []*RelayTrace
}

func NewRelayTrace(c *gin.Context) *RelayTrace {
	t := &RelayTrace{start: time.Now(), fields: make(map[string]any)}
	t.Set("started_at", t.start.UTC().Format(time.RFC3339Nano))
	t.Set("gateway_request_id", c.GetString(RequestIdKey))
	t.Set("client_request_id", RelayTraceID(c.GetHeader("X-Client-Request-Id")))
	t.Set("cf_ray", RelayTraceID(c.GetHeader("Cf-Ray")))
	c.Set(relayTraceKey, t)
	return t
}

func RelayTraceFromContext(c *gin.Context) *RelayTrace {
	if c == nil {
		return nil
	}
	value, _ := c.Get(relayTraceKey)
	trace, _ := value.(*RelayTrace)
	return trace
}

// Unknown identifiers are fingerprinted instead of logging arbitrary headers.
func RelayTraceID(value string) string {
	if value == "" {
		return ""
	}
	if relayTraceIDPattern.MatchString(value) {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%x", sum[:12])
}

func (t *RelayTrace) Mark(phase string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.fields[phase]; !exists {
		t.fields[phase] = time.Since(t.start).Milliseconds()
	}
}

func (t *RelayTrace) Set(key string, value any) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fields[key] = value
}

func (t *RelayTrace) Add(key string, n int) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	current, _ := t.fields[key].(int64)
	t.fields[key] = current + int64(n)
}

func (t *RelayTrace) BeginAttempt(channelID int) *RelayTrace {
	if t == nil {
		return nil
	}
	attempt := &RelayTrace{start: t.start, fields: make(map[string]any)}
	attempt.Set("channel_id", channelID)
	attempt.Mark("upstream_start_ms")
	t.mu.Lock()
	t.attempts = append(t.attempts, attempt)
	t.mu.Unlock()
	return attempt
}

func (t *RelayTrace) CurrentAttempt() *RelayTrace {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.attempts) == 0 {
		return nil
	}
	return t.attempts[len(t.attempts)-1]
}

func (t *RelayTrace) RecordError(key string, err error) {
	if t == nil || err == nil {
		return
	}
	kind := "io_error"
	var netErr net.Error
	switch {
	case errors.Is(err, context.Canceled):
		kind = "canceled"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		kind = "timeout"
	case errors.Is(err, io.ErrUnexpectedEOF):
		kind = "unexpected_eof"
	case errors.Is(err, net.ErrClosed), errors.Is(err, io.ErrClosedPipe):
		kind = "closed"
	}
	t.Set(key, kind)
	t.Mark(key + "_ms")
}

func (t *RelayTrace) Snapshot() map[string]any {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make(map[string]any, len(t.fields)+1)
	for key, value := range t.fields {
		result[key] = value
	}
	if len(t.attempts) > 0 {
		attempts := make([]map[string]any, 0, len(t.attempts))
		for _, attempt := range t.attempts {
			attempts = append(attempts, attempt.Snapshot())
		}
		result["attempts"] = attempts
	}
	return result
}
