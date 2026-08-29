package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"github.com/JekYUlll/Dipole/internal/transport/ws"
)

const defaultBaseURL = "http://127.0.0.1:8080"

type config struct {
	baseURL   string
	telephone string
	password  string
	target    string
}

type loginRequest struct {
	Telephone string `json:"telephone"`
	Password  string `json:"password"`
}

type loginResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
		User  struct {
			UUID     string `json:"uuid"`
			Nickname string `json:"nickname"`
		} `json:"user"`
	} `json:"data"`
}

type eventEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type app struct {
	cfg        config
	httpClient *http.Client

	mu       sync.RWMutex
	conn     *websocket.Conn
	token    string
	userUUID string
	nickname string
	target   string
}

func main() {
	cfg := parseFlags()
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		flag.Usage()
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := &app{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		target: strings.TrimSpace(cfg.target),
	}

	if err := client.run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "wscli error:", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.baseURL, "base", defaultBaseURL, "Dipole server base URL, for example http://127.0.0.1:8080")
	flag.StringVar(&cfg.telephone, "telephone", "", "login telephone")
	flag.StringVar(&cfg.password, "password", "", "login password")
	flag.StringVar(&cfg.target, "target", "", "default target user UUID")
	flag.Parse()
	return cfg
}

func validateConfig(cfg config) error {
	if strings.TrimSpace(cfg.telephone) == "" {
		return errors.New("telephone is required")
	}
	if strings.TrimSpace(cfg.password) == "" {
		return errors.New("password is required")
	}
	return nil
}

func (a *app) run(ctx context.Context) error {
	if err := a.login(ctx); err != nil {
		return err
	}
	if err := a.connect(ctx); err != nil {
		return err
	}

	fmt.Printf("logged in as %s (%s)\n", a.nickname, a.userUUID)
	if a.target != "" {
		fmt.Printf("default target: %s\n", a.target)
	}
	printHelp()

	readDone := make(chan error, 1)
	go func() {
		readDone <- a.readLoop()
	}()

	inputDone := make(chan error, 1)
	go func() {
		inputDone <- a.inputLoop()
	}()

	select {
	case <-ctx.Done():
		a.close()
		return nil
	case err := <-readDone:
		a.close()
		if err == nil || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return nil
		}
		return err
	case err := <-inputDone:
		a.close()
		if err != nil {
			return err
		}
		select {
		case readErr := <-readDone:
			if readErr == nil || websocket.IsCloseError(readErr, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return readErr
		case <-time.After(2 * time.Second):
			return nil
		}
	}
}

func (a *app) login(ctx context.Context) error {
	body, err := json.Marshal(loginRequest{
		Telephone: strings.TrimSpace(a.cfg.telephone),
		Password:  a.cfg.password,
	})
	if err != nil {
		return fmt.Errorf("marshal login request: %w", err)
	}

	loginURL, err := buildHTTPURL(a.cfg.baseURL, "/api/v1/auth/login")
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var response loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if response.Message == "" {
			response.Message = resp.Status
		}
		return fmt.Errorf("login failed: %s", response.Message)
	}
	if response.Code != 0 {
		return fmt.Errorf("login rejected: %s", response.Message)
	}

	a.token = response.Data.Token
	a.userUUID = response.Data.User.UUID
	a.nickname = response.Data.User.Nickname
	return nil
}

func (a *app) connect(ctx context.Context) error {
	wsURL, err := buildWebSocketURL(a.cfg.baseURL, a.token)
	if err != nil {
		return err
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("connect websocket: %w", err)
	}

	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()
	return nil
}

func (a *app) inputLoop() error {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("wscli> ")
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			fmt.Print("wscli> ")
			continue
		}

		if strings.HasPrefix(line, "/") {
			shouldExit, err := a.handleCommand(line)
			if err != nil {
				fmt.Println("error:", err)
			}
			if shouldExit {
				return nil
			}
		} else {
			if err := a.sendText(line); err != nil {
				fmt.Println("error:", err)
			}
		}

		fmt.Print("wscli> ")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	return nil
}

func (a *app) handleCommand(line string) (bool, error) {
	switch {
	case line == "/help":
		printHelp()
		return false, nil
	case line == "/quit" || line == "/exit":
		return true, nil
	case line == "/whoami":
		fmt.Printf("user=%s nickname=%s target=%s\n", a.userUUID, a.nickname, a.currentTarget())
		return false, nil
	case strings.HasPrefix(line, "/target"):
		target := strings.TrimSpace(strings.TrimPrefix(line, "/target"))
		if target == "" {
			fmt.Printf("current target: %s\n", a.currentTarget())
			return false, nil
		}
		a.setTarget(target)
		fmt.Printf("target set to %s\n", target)
		return false, nil
	case strings.HasPrefix(line, "/send "):
		content := strings.TrimSpace(strings.TrimPrefix(line, "/send "))
		if content == "" {
			return false, errors.New("message content is required")
		}
		return false, a.sendText(content)
	default:
		return false, fmt.Errorf("unknown command: %s", line)
	}
}

func (a *app) sendText(content string) error {
	target := a.currentTarget()
	if target == "" {
		return errors.New("target is not set, use /target <uuid> first")
	}

	payload, err := ws.EncodeCommand(ws.TypeChatSend, ws.SendTextMessageInput{
		TargetUUID: target,
		Content:    content,
	})
	if err != nil {
		return err
	}

	a.mu.RLock()
	conn := a.conn
	a.mu.RUnlock()
	if conn == nil {
		return errors.New("websocket connection is not ready")
	}

	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return fmt.Errorf("send websocket message: %w", err)
	}

	return nil
}

func (a *app) readLoop() error {
	a.mu.RLock()
	conn := a.conn
	a.mu.RUnlock()
	if conn == nil {
		return errors.New("websocket connection is not ready")
	}

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		if err := a.printEvent(payload); err != nil {
			fmt.Println("error:", err)
		}
		fmt.Print("wscli> ")
	}
}

func (a *app) printEvent(payload []byte) error {
	var envelope eventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("decode event envelope: %w", err)
	}

	switch envelope.Type {
	case ws.TypeConnected:
		var data ws.ConnectedEventData
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return err
		}
		fmt.Printf("\n[event:%s] user=%s connections=%d online=%d\n",
			envelope.Type, data.UserUUID, data.ConnectionCount, data.OnlineUserCount)
	case ws.TypeChatSent:
		var data ws.ChatSentData
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return err
		}
		fmt.Printf("\n[event:%s] id=%s to=%s delivered=%t content=%q\n",
			envelope.Type, data.MessageID, data.TargetUUID, data.Delivered, data.Content)
	case ws.TypeChatMessage:
		var data ws.ChatMessageData
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return err
		}
		fmt.Printf("\n[event:%s] id=%s from=%s content=%q\n",
			envelope.Type, data.MessageID, data.FromUUID, data.Content)
	case ws.TypeError:
		var data ws.ErrorEventData
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return err
		}
		fmt.Printf("\n[event:%s] code=%s request=%s message=%s\n",
			envelope.Type, data.Code, data.RequestType, data.Message)
	default:
		fmt.Printf("\n[event:%s] %s\n", envelope.Type, string(envelope.Data))
	}

	return nil
}

func (a *app) setTarget(target string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.target = strings.TrimSpace(target)
}

func (a *app) currentTarget() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.target
}

func (a *app) close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn != nil {
		_ = a.conn.Close()
		a.conn = nil
	}
}

func buildHTTPURL(baseURL string, endpoint string) (string, error) {
	u, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = joinPath(u.Path, endpoint)
	u.RawQuery = ""
	return u.String(), nil
}

func buildWebSocketURL(baseURL string, token string) (string, error) {
	u, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported base URL scheme: %s", u.Scheme)
	}

	u.Path = joinPath(u.Path, "/api/v1/ws")
	query := u.Query()
	query.Set("token", token)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("base URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid base URL: %s", raw)
	}

	return u, nil
}

func joinPath(basePath string, endpoint string) string {
	basePath = strings.TrimRight(basePath, "/")
	endpoint = "/" + strings.TrimLeft(endpoint, "/")
	if basePath == "" {
		return endpoint
	}
	return basePath + endpoint
}

func printHelp() {
	fmt.Println("commands:")
	fmt.Println("  /help                show this help")
	fmt.Println("  /target <uuid>       set or view current target user")
	fmt.Println("  /send <text>         send text message to current target")
	fmt.Println("  /whoami              print current login and target")
	fmt.Println("  /quit                exit client")
	fmt.Println("plain text input sends a message to the current target")
}
