package helper

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cleanupStreamBody struct {
	reader io.Reader
	closed chan struct{}
	once   sync.Once
	err    error
}

func (b *cleanupStreamBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	if n > 0 || err != io.EOF {
		return n, err
	}
	if b.err != nil {
		return 0, b.err
	}
	<-b.closed
	return 0, net.ErrClosed
}

func (b *cleanupStreamBody) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

func TestStreamScannerHandlerCleanupKeepsActualFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason relaycommon.StreamEndReason
		cause  error
	}{
		{"completed", relaycommon.StreamEndReasonDone, nil},
		{"client_cancel", relaycommon.StreamEndReasonClientGone, nil},
		{"read_failure", relaycommon.StreamEndReasonScannerErr, io.ErrUnexpectedEOF},
		{"peer_closed", relaycommon.StreamEndReasonScannerErr, net.ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, info := setupStreamTest(t, strings.NewReader(""))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			info.DisablePing = true
			body := &cleanupStreamBody{
				reader: strings.NewReader("data: partial\n\n"),
				closed: make(chan struct{}),
				err:    tc.cause,
			}
			t.Cleanup(func() { _ = body.Close() })
			var errors bytes.Buffer
			common.LogWriterMu.Lock()
			previous := gin.DefaultErrorWriter
			gin.DefaultErrorWriter = &errors
			common.LogWriterMu.Unlock()
			defer func() {
				common.LogWriterMu.Lock()
				gin.DefaultErrorWriter = previous
				common.LogWriterMu.Unlock()
			}()

			StreamScannerHandler(c, &http.Response{Body: body}, info, func(data string, sr *StreamResult) {
				switch tc.reason {
				case relaycommon.StreamEndReasonDone:
					sr.Done()
				case relaycommon.StreamEndReasonClientGone:
					cancel()
				}
			})
			require.NotNil(t, info.StreamStatus)
			assert.Equal(t, tc.reason, info.StreamStatus.EndReason)
			select {
			case <-body.closed:
			default:
				t.Fatal("handler must close the upstream body")
			}
			if tc.cause != nil {
				require.ErrorIs(t, info.StreamStatus.EndError, tc.cause)
				assert.Contains(t, errors.String(), "scanner error: "+tc.cause.Error())
				assert.False(t, info.StreamStatus.IsNormalEnd())
			} else {
				assert.NotContains(t, errors.String(), "scanner error:")
				assert.NotContains(t, errors.String(), "use of closed network connection")
			}
		})
	}
}
