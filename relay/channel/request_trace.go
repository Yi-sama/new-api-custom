package channel

import (
	"io"

	"github.com/QuantumNous/new-api/common"
)

type tracedUpstreamBody struct {
	io.ReadCloser
	trace *common.RelayTrace
}

func (b *tracedUpstreamBody) Read(data []byte) (int, error) {
	n, err := b.ReadCloser.Read(data)
	if n > 0 {
		b.trace.Mark("first_body_read_ms")
		b.trace.Add("body_bytes_read", n)
	}
	if err == io.EOF {
		b.trace.Mark("body_eof_ms")
	} else {
		b.trace.RecordError("body_read_error", err)
	}
	return n, err
}

func (b *tracedUpstreamBody) Close() error {
	err := b.ReadCloser.Close()
	b.trace.Mark("body_close_ms")
	b.trace.RecordError("body_close_error", err)
	return err
}
