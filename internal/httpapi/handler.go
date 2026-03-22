package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Bridge 定义 Telegram 兼容层所依赖的最小发送能力。
type Bridge interface {
	GetMe(ctx context.Context) (BotProfile, error)
	SendText(ctx context.Context, chatID string, text string) (SentMessage, error)
	SendTyping(ctx context.Context, chatID string, action string) error
}

// Options 是 HTTP 处理器的构造参数。
type Options struct {
	TelegramToken string
	Bridge        Bridge
	Now           func() time.Time
}

// BotProfile 描述对外暴露的 Telegram Bot 信息。
type BotProfile struct {
	ID        int64
	FirstName string
	Username  string
}

// SentMessage 描述桥接层发送成功后的最小结果。
type SentMessage struct {
	MessageID int64
	ChatID    string
	Text      string
	SentAt    time.Time
}

// Handler 实现 Telegram Bot 风格路由。
type Handler struct {
	telegramToken string
	bridge        Bridge
	now           func() time.Time
}

// NewHandler 创建一个新的 Telegram 兼容处理器。
func NewHandler(opts Options) *Handler {
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	return &Handler{
		telegramToken: opts.TelegramToken,
		bridge:        opts.Bridge,
		now:           now,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	method, authorized := h.parseTelegramMethod(r.URL.Path)
	if !authorized {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch method {
	case "getMe":
		h.handleGetMe(w, r)
	case "sendMessage":
		h.handleSendMessage(w, r)
	case "sendChatAction":
		h.handleSendChatAction(w, r)
	default:
		writeError(w, http.StatusNotFound, "Not Found")
	}
}

func (h *Handler) parseTelegramMethod(path string) (string, bool) {
	if !strings.HasPrefix(path, "/bot") {
		return "", false
	}

	trimmed := strings.TrimPrefix(path, "/bot")
	parts := strings.Split(strings.TrimPrefix(trimmed, "/"), "/")
	if len(parts) == 1 {
		tokenAndMethod := parts[0]
		index := strings.Index(tokenAndMethod, "/")
		if index >= 0 {
			parts = []string{tokenAndMethod[:index], tokenAndMethod[index+1:]}
		}
	}

	if len(parts) != 2 {
		tokenAndMethod := strings.SplitN(trimmed, "/", 2)
		if len(tokenAndMethod) != 2 {
			return "", false
		}
		parts = tokenAndMethod
	}

	token := strings.TrimPrefix(parts[0], "/")
	method := parts[1]
	if token != h.telegramToken {
		return "", false
	}

	return method, true
}

func (h *Handler) handleGetMe(w http.ResponseWriter, r *http.Request) {
	profile, err := h.bridge.GetMe(r.Context())
	if err != nil {
		writeBridgeError(w, err)
		return
	}

	result := botResult{
		ID:                      profile.ID,
		IsBot:                   true,
		FirstName:               profile.FirstName,
		Username:                profile.Username,
		CanJoinGroups:           false,
		CanReadAllGroupMessages: false,
		SupportsInlineQueries:   false,
	}

	writeJSON(w, http.StatusOK, apiResponse[botResult]{
		OK:     true,
		Result: result,
	})
}

func (h *Handler) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSendMessageRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, sendErr := h.bridge.SendText(r.Context(), req.ChatID, req.Text)
	if sendErr != nil {
		writeBridgeError(w, sendErr)
		return
	}

	writeJSON(w, http.StatusOK, apiResponse[messageResult]{
		OK: true,
		Result: messageResult{
			MessageID: result.MessageID,
			Date:      result.SentAt.Unix(),
			Chat: chatResult{
				ID:   result.ChatID,
				Type: "private",
			},
			Text: result.Text,
		},
	})
}

func (h *Handler) handleSendChatAction(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSendChatActionRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err = h.bridge.SendTyping(r.Context(), req.ChatID, req.Action); err != nil {
		writeBridgeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, apiResponse[bool]{
		OK:     true,
		Result: true,
	})
}

type sendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type sendChatActionRequest struct {
	ChatID string `json:"chat_id"`
	Action string `json:"action"`
}

func decodeSendMessageRequest(r *http.Request) (sendMessageRequest, error) {
	var req sendMessageRequest
	if err := decodeBody(r, &req); err != nil {
		return sendMessageRequest{}, err
	}
	if strings.TrimSpace(req.ChatID) == "" {
		return sendMessageRequest{}, errors.New("chat_id is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return sendMessageRequest{}, errors.New("text is required")
	}
	return req, nil
}

func decodeSendChatActionRequest(r *http.Request) (sendChatActionRequest, error) {
	var req sendChatActionRequest
	if err := decodeBody(r, &req); err != nil {
		return sendChatActionRequest{}, err
	}
	if strings.TrimSpace(req.ChatID) == "" {
		return sendChatActionRequest{}, errors.New("chat_id is required")
	}
	if strings.TrimSpace(req.Action) == "" {
		return sendChatActionRequest{}, errors.New("action is required")
	}
	return req, nil
}

func decodeBody(r *http.Request, target any) error {
	contentType := r.Header.Get("Content-Type")

	switch {
	case strings.HasPrefix(contentType, "application/json"):
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(target); err != nil {
			return fmt.Errorf("invalid json body: %w", err)
		}
		return nil

	case strings.HasPrefix(contentType, "application/x-www-form-urlencoded"),
		strings.HasPrefix(contentType, "multipart/form-data"),
		contentType == "":
		if err := r.ParseForm(); err != nil {
			return fmt.Errorf("invalid form body: %w", err)
		}

		switch value := target.(type) {
		case *sendMessageRequest:
			value.ChatID = r.FormValue("chat_id")
			value.Text = r.FormValue("text")
			return nil
		case *sendChatActionRequest:
			value.ChatID = r.FormValue("chat_id")
			value.Action = r.FormValue("action")
			return nil
		default:
			return errors.New("unsupported form payload")
		}

	default:
		return fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// APIError 用于桥接层将业务错误映射为 Telegram 风格 HTTP 错误。
type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return e.Description
}

// ErrBadRequest 创建一个 400 级别的业务错误。
func ErrBadRequest(description string) error {
	return &APIError{
		Code:        http.StatusBadRequest,
		Description: description,
	}
}

// ErrBadGateway 创建一个 502 级别的业务错误。
func ErrBadGateway(description string) error {
	return &APIError{
		Code:        http.StatusBadGateway,
		Description: description,
	}
}

func writeBridgeError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		writeError(w, apiErr.Code, apiErr.Description)
		return
	}

	writeError(w, http.StatusBadGateway, err.Error())
}

func writeError(w http.ResponseWriter, statusCode int, description string) {
	writeJSON(w, statusCode, errorResponse{
		OK:          false,
		ErrorCode:   statusCode,
		Description: description,
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

type apiResponse[T any] struct {
	OK     bool `json:"ok"`
	Result T    `json:"result"`
}

type errorResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

type botResult struct {
	ID                      int64  `json:"id"`
	IsBot                   bool   `json:"is_bot"`
	FirstName               string `json:"first_name"`
	Username                string `json:"username"`
	CanJoinGroups           bool   `json:"can_join_groups"`
	CanReadAllGroupMessages bool   `json:"can_read_all_group_messages"`
	SupportsInlineQueries   bool   `json:"supports_inline_queries"`
}

type messageResult struct {
	MessageID int64      `json:"message_id"`
	Date      int64      `json:"date"`
	Chat      chatResult `json:"chat"`
	Text      string     `json:"text"`
}

type chatResult struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
