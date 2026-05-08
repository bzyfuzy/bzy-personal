package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SSEEvent is a parsed Server-Sent Event.
type SSEEvent struct {
	ID    string
	Event string // event type field, e.g. "delta", "done", "error"
	Data  string // raw JSON string
}

// SSEReader reads Server-Sent Events from an HTTP response body.
type SSEReader struct {
	resp   *http.Response
	reader *bufio.Reader
}

func newSSEReader(resp *http.Response) *SSEReader {
	return &SSEReader{
		resp:   resp,
		reader: bufio.NewReaderSize(resp.Body, 8192),
	}
}

// Next blocks until the next event arrives, returns io.EOF when stream ends.
func (r *SSEReader) Next() (*SSEEvent, error) {
	event := &SSEEvent{}
	for {
		line, err := r.reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")

		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("sse read: %w", err)
		}

		switch {
		case strings.HasPrefix(line, "id:"):
			event.ID = strings.TrimSpace(line[3:])
		case strings.HasPrefix(line, "event:"):
			event.Event = strings.TrimSpace(line[6:])
		case strings.HasPrefix(line, "data:"):
			event.Data = strings.TrimSpace(line[5:])
		case line == "":
			// blank line = dispatch event
			if event.Event == "done" || event.Data == "[DONE]" {
				return nil, io.EOF
			}
			if event.Data != "" {
				return event, nil
			}
			event = &SSEEvent{} // reset for next event
		}

		if err == io.EOF {
			return nil, io.EOF
		}
	}
}

// DecodeData unmarshals the event Data field into v.
func (e *SSEEvent) DecodeData(v any) error {
	return json.Unmarshal([]byte(e.Data), v)
}

// Close releases the underlying HTTP response.
func (r *SSEReader) Close() error {
	return r.resp.Body.Close()
}

// Iter calls fn for each event until the stream ends or ctx is cancelled.
func (r *SSEReader) Iter(ctx context.Context, fn func(*SSEEvent) error) error {
	defer r.Close()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		event, err := r.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := fn(event); err != nil {
			return err
		}
	}
}
