package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

type requestHandler func(id any, method string, params json.RawMessage)
type notificationHandler func(method string, params json.RawMessage)

type transport struct {
	in            io.ReadCloser
	out           io.WriteCloser
	onRequest     requestHandler
	onNotify      notificationHandler
	nextID        int64
	pending       map[string]chan Response
	mu            sync.Mutex
	writeMu       sync.Mutex
	closeOnce     sync.Once
	closed        chan struct{}
	backgroundErr error
}

func newTransport(in io.ReadCloser, out io.WriteCloser, onRequest requestHandler, onNotify notificationHandler) *transport {
	return &transport{
		in:        in,
		out:       out,
		onRequest: onRequest,
		onNotify:  onNotify,
		pending:   map[string]chan Response{},
		closed:    make(chan struct{}),
	}
}

func (t *transport) start() {
	go t.readLoop()
}

func (t *transport) close() error {
	t.closeOnce.Do(func() {
		close(t.closed)
		_ = t.in.Close()
		_ = t.out.Close()
		t.mu.Lock()
		for key, ch := range t.pending {
			delete(t.pending, key)
			close(ch)
		}
		t.mu.Unlock()
	})
	return nil
}

func (t *transport) backgroundError() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.backgroundErr
}

func (t *transport) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&t.nextID, 1)
	key := idKey(float64(id))
	ch := make(chan Response, 1)
	t.mu.Lock()
	t.pending[key] = ch
	t.mu.Unlock()

	if err := t.write(Request{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		t.mu.Lock()
		delete(t.pending, key)
		t.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pending, key)
		t.mu.Unlock()
		return nil, ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("transport closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("jsonrpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (t *transport) sendResponse(id any, result any, rpcErr *Error) error {
	resp := Response{JSONRPC: "2.0", ID: id, Error: rpcErr}
	if rpcErr == nil {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		resp.Result = data
	}
	return t.write(resp)
}

func (t *transport) write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if _, err := t.out.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (t *transport) readLoop() {
	defer t.close()
	scanner := bufio.NewScanner(t.in)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			continue
		}
		if _, hasMethod := raw["method"]; hasMethod {
			t.handleMethod(raw)
			continue
		}
		t.handleResponse(raw)
	}
	t.mu.Lock()
	t.backgroundErr = scanner.Err()
	t.mu.Unlock()
}

func (t *transport) handleMethod(raw map[string]json.RawMessage) {
	var method string
	_ = json.Unmarshal(raw["method"], &method)
	params := raw["params"]
	if idRaw, ok := raw["id"]; ok {
		var id any
		_ = json.Unmarshal(idRaw, &id)
		if t.onRequest != nil {
			t.onRequest(id, method, params)
			return
		}
		_ = t.sendResponse(id, nil, &Error{Code: -32601, Message: "method not found"})
		return
	}
	if t.onNotify != nil {
		t.onNotify(method, params)
	}
}

func (t *transport) handleResponse(raw map[string]json.RawMessage) {
	var resp Response
	if err := json.Unmarshal(mustMarshal(raw), &resp); err != nil {
		return
	}
	key := idKey(resp.ID)
	t.mu.Lock()
	ch := t.pending[key]
	delete(t.pending, key)
	t.mu.Unlock()
	if ch != nil {
		ch <- resp
		close(ch)
	}
}

func mustMarshal(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

func idKey(id any) string {
	switch v := id.(type) {
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case string:
		return v
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}
