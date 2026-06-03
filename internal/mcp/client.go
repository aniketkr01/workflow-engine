package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/domain"
)

// Client connects to an MCP server via HTTP or stdio.
type Client interface {
	Initialize(ctx context.Context) (*InitializeResult, error)
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, toolName string, args map[string]any) (*CallToolResult, error)
	Close() error
}

// NewClient creates an appropriate client based on the server transport.
func NewClient(srv *domain.MCPServer) (Client, error) {
	switch srv.Transport {
	case "http":
		return newHTTPClient(srv.Endpoint), nil
	case "stdio":
		return newStdioClient(srv.Endpoint)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", srv.Transport)
	}
}

// ---- HTTP Client ----

type httpClient struct {
	endpoint string
	http     *http.Client
	seq      int
	mu       sync.Mutex
}

func newHTTPClient(endpoint string) *httpClient {
	return &httpClient{
		endpoint: endpoint,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *httpClient) nextID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	return c.seq
}

func (c *httpClient) call(ctx context.Context, method string, params any, result any) error {
	req := Request{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  method,
		Params:  params,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp Response
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	return json.Unmarshal(resultBytes, result)
}

func (c *httpClient) Initialize(ctx context.Context) (*InitializeResult, error) {
	var result InitializeResult
	err := c.call(ctx, "initialize", InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo:      ClientInfo{Name: "workflow-engine", Version: "1.0.0"},
	}, &result)
	return &result, err
}

func (c *httpClient) ListTools(ctx context.Context) ([]Tool, error) {
	var result ListToolsResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *httpClient) CallTool(ctx context.Context, toolName string, args map[string]any) (*CallToolResult, error) {
	var result CallToolResult
	err := c.call(ctx, "tools/call", CallToolParams{Name: toolName, Arguments: args}, &result)
	return &result, err
}

func (c *httpClient) Close() error { return nil }

// ---- Stdio Client ----

type stdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	seq    int
	mu     sync.Mutex
}

func newStdioClient(command string) (*stdioClient, error) {
	cmd := exec.Command("sh", "-c", command)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mcp server: %w", err)
	}
	return &stdioClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}, nil
}

func (c *stdioClient) nextID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	return c.seq
}

func (c *stdioClient) call(ctx context.Context, method string, params any, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := Request{
		JSONRPC: "2.0",
		ID:      c.seq + 1,
		Method:  method,
		Params:  params,
	}
	c.seq++

	line, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	line = append(line, '\n')

	if _, err := c.stdin.Write(line); err != nil {
		return fmt.Errorf("write to stdin: %w", err)
	}

	// Read response line.
	done := make(chan error, 1)
	var rpcResp Response
	go func() {
		if c.stdout.Scan() {
			done <- json.Unmarshal(c.stdout.Bytes(), &rpcResp)
		} else {
			done <- fmt.Errorf("no response from stdio server")
		}
	}()

	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return err
	}
	return json.Unmarshal(resultBytes, result)
}

func (c *stdioClient) Initialize(ctx context.Context) (*InitializeResult, error) {
	var result InitializeResult
	err := c.call(ctx, "initialize", InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo:      ClientInfo{Name: "workflow-engine", Version: "1.0.0"},
	}, &result)
	return &result, err
}

func (c *stdioClient) ListTools(ctx context.Context) ([]Tool, error) {
	var result ListToolsResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *stdioClient) CallTool(ctx context.Context, toolName string, args map[string]any) (*CallToolResult, error) {
	var result CallToolResult
	err := c.call(ctx, "tools/call", CallToolParams{Name: toolName, Arguments: args}, &result)
	return &result, err
}

func (c *stdioClient) Close() error {
	_ = c.stdin.Close()
	return c.cmd.Wait()
}
