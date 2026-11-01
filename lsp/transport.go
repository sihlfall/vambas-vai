package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

const maxMessageSize = 16 << 20

// Each connection has one reader and a serialized writer. Workers can reply out
// of order without interleaving headers or JSON bodies on the stdio stream.
type connection struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
	err    error
}

func (c *connection) read() ([]byte, error) {
	length, headerSize := -1, 0
	for {
		fragment, err := c.reader.ReadSlice('\n')
		line := string(fragment)
		if err != nil {
			if err == bufio.ErrBufferFull {
				return nil, fmt.Errorf("LSP header line too large")
			}
			if err == io.EOF && (headerSize != 0 || len(line) != 0) {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
		headerSize += len(line)
		if headerSize > 8192 {
			return nil, fmt.Errorf("LSP header too large")
		}
		if line == "\r\n" {
			break
		}
		key, value, ok := strings.Cut(strings.TrimSuffix(line, "\r\n"), ":")
		if !ok {
			return nil, fmt.Errorf("invalid LSP header")
		}
		if strings.EqualFold(key, "Content-Length") {
			if length != -1 {
				return nil, fmt.Errorf("duplicate Content-Length")
			}
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n < 0 || n > maxMessageSize {
				return nil, fmt.Errorf("invalid Content-Length")
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(c.reader, body)
	return body, err
}

func (c *connection) write(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	_, c.err = fmt.Fprintf(c.writer, "Content-Length: %d\r\n\r\n", len(data))
	if c.err == nil {
		var n int
		n, c.err = c.writer.Write(data)
		if c.err == nil && n != len(data) {
			c.err = io.ErrShortWrite
		}
	}
	return c.err
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
}
type errorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   *responseError  `json:"error"`
}
type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

func (c *connection) reply(id json.RawMessage, value any, err *responseError) {
	if err != nil {
		_ = c.write(errorResponse{JSONRPC: "2.0", ID: id, Error: err})
	} else {
		_ = c.write(response{JSONRPC: "2.0", ID: id, Result: value})
	}
}
func (c *connection) notify(method string, params any) {
	_ = c.write(notification{JSONRPC: "2.0", Method: method, Params: params})
}
