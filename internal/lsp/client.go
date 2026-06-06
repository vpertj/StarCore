package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type Client struct {
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stdout         *bufio.Reader
	mu             sync.Mutex
	idSeq          int
	pending        map[int]chan *Response
	notifyHandlers map[string]func(params json.RawMessage)
	done           chan struct{}
}

func NewClient(command string, args ...string) (*Client, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", command, err)
	}

	c := &Client{
		cmd:            cmd,
		stdin:          stdin,
		stdout:         bufio.NewReader(stdout),
		pending:        make(map[int]chan *Response),
		notifyHandlers: make(map[string]func(params json.RawMessage)),
		done:           make(chan struct{}),
	}

	go c.readLoop()
	go c.readStderr(stderr)

	return c, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	close(c.done)
	c.mu.Unlock()
	c.stdin.Close()
	return c.cmd.Wait()
}

func (c *Client) OnNotification(method string, handler func(params json.RawMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifyHandlers[method] = handler
}

func (c *Client) Call(method string, params interface{}, result interface{}) error {
	c.mu.Lock()
	c.idSeq++
	id := c.idSeq
	ch := make(chan *Response, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if err := c.write(data); err != nil {
		return err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return fmt.Errorf("LSP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if result != nil && resp.Result != nil {
			return json.Unmarshal(*rawMessage(resp.Result), result)
		}
		return nil
	case <-c.done:
		return fmt.Errorf("client closed")
	}
}

func (c *Client) Notify(method string, params interface{}) error {
	req := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return c.write(data)
}

func (c *Client) write(data []byte) error {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	_, err := c.stdin.Write(data)
	return err
}

func (c *Client) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		msg, err := c.readMessage()
		if err != nil {
			return
		}

		// Check if it's a response
		var resp Response
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID != nil {
			c.mu.Lock()
			ch, ok := c.pending[*resp.ID]
			if ok {
				delete(c.pending, *resp.ID)
			}
			c.mu.Unlock()
			if ok {
				ch <- &resp
			}
			continue
		}

		// Check if it's a notification
		var notif Notification
		if err := json.Unmarshal(msg, &notif); err == nil && notif.Method != "" {
			c.mu.Lock()
			handler, ok := c.notifyHandlers[notif.Method]
			c.mu.Unlock()
			if ok && notif.Params != nil {
				params, _ := json.Marshal(notif.Params)
				handler(params)
			}
		}
	}
}

func (c *Client) readMessage() ([]byte, error) {
	// Read Content-Length header
	var contentLength int
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("invalid content length")
	}

	buf := make([]byte, contentLength)
	_, err := io.ReadFull(c.stdout, buf)
	return buf, err
}

func (c *Client) readStderr(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		_ = scanner.Text()
	}
	_ = scanner.Err()
}

func rawMessage(v interface{}) *json.RawMessage {
	b, _ := json.Marshal(v)
	raw := json.RawMessage(b)
	return &raw
}
