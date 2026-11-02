package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/gin-gonic/gin"
)

func traceRelayRequest(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.Next()
		return
	}
	switch c.Request.URL.Path {
	case "/v1/messages", "/v1/chat/completions", "/v1/responses":
	default:
		c.Next()
		return
	}

	trace := common.NewRelayTrace(c)
	writer := c.Writer
	c.Writer = &relayTraceWriter{ResponseWriter: writer, trace: trace}
	defer func() {
		trace.Mark("handler_end_ms")
		trace.Set("http_status", writer.Status())
		trace.Set("client_canceled", c.Request.Context().Err() != nil)
		trace.Set("path", c.Request.URL.Path)
		c.Writer = writer
		if data, err := common.Marshal(trace.Snapshot()); err == nil {
			logger.LogInfo(c, "relay_trace "+string(data))
		}
	}()
	c.Next()
}

type relayTraceWriter struct {
	gin.ResponseWriter
	trace *common.RelayTrace
}

func (w *relayTraceWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if n > 0 {
		w.trace.Mark("first_write_return_ms")
		w.trace.Add("downstream_bytes_written", n)
	}
	w.trace.RecordError("downstream_write_error", err)
	return n, err
}

func (w *relayTraceWriter) WriteString(data string) (int, error) {
	n, err := w.ResponseWriter.WriteString(data)
	if n > 0 {
		w.trace.Mark("first_write_return_ms")
		w.trace.Add("downstream_bytes_written", n)
	}
	w.trace.RecordError("downstream_write_error", err)
	return n, err
}

func (w *relayTraceWriter) Flush() {
	w.ResponseWriter.Flush()
	// Gin's Flush has no error return. This is not a client receipt/ACK.
	w.trace.Mark("first_flush_return_ms")
}

func (w *relayTraceWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
