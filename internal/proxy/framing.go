package proxy

import (
	"bufio"
	"io"
	"sync"
)

// lineReader reads newline-delimited JSON-RPC messages.
//
// The MCP stdio transport frames each message as one line, and a message
// must not contain a raw newline (MCP spec, basic/transports/stdio,
// revisions 2025-11-25 and 2026-07-28). The spec sets no maximum size, so
// this uses bufio.Reader.ReadBytes, which grows as needed, rather than
// bufio.Scanner, which stops at 64 KiB unless told otherwise.
type lineReader struct {
	r *bufio.Reader
}

func newLineReader(r io.Reader) *lineReader {
	return &lineReader{r: bufio.NewReaderSize(r, 64*1024)}
}

// next returns the next line exactly as read, including its "\n" (or
// "\r\n"). The last line of a stream may lack the newline. At the end of
// the stream it returns io.EOF and no data.
func (l *lineReader) next() ([]byte, error) {
	line, err := l.r.ReadBytes('\n')
	if len(line) > 0 {
		// Return what was read even if err is io.EOF; the caller sees
		// io.EOF on the following call.
		return line, nil
	}
	return nil, err
}

// syncWriter serialises whole-line writes from several goroutines. Both
// directions of the proxy can write to the client: forwarded server output
// and locally generated denials. Without the mutex two lines could
// interleave mid-line.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) writeLine(line []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.w.Write(line)
	return err
}
