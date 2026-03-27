package bridge

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"strings"
	"sync"
	"time"

	ilink "github.com/openilink/openilink-sdk-go"

	"openilink-tg/internal/httpapi"
	"openilink-tg/internal/state"
)

// Client 描述桥接层所依赖的 iLink 客户端能力。
type Client interface {
	SendText(ctx context.Context, to string, text string, contextToken string) (string, error)
	SendMediaFile(ctx context.Context, to string, contextToken string, data []byte, fileName string, caption string) error
	GetContextToken(userID string) (string, bool)
	SetContextToken(userID string, token string)
	GetConfig(ctx context.Context, userID string, contextToken string) (*ilink.GetConfigResp, error)
	SendTyping(ctx context.Context, userID string, typingTicket string, status ilink.TypingStatus) error
	Monitor(ctx context.Context, handler ilink.MessageHandler, opts *ilink.MonitorOptions) error
}

// StateStore 抽象了运行状态的持久化方式。
type StateStore interface {
	Load() (state.State, error)
	Save(next state.State) error
}

// Config 保存桥接层自己的业务配置。
type Config struct {
	BotID       int64
	BotName     string
	BotUsername string
}

// Service 负责将 Telegram 兼容调用翻译为 iLink SDK 调用。
type Service struct {
	client Client
	store  StateStore
	config Config

	mu      sync.Mutex
	now     func() time.Time
	syncBuf string
	state   state.State
}

// NewService 创建桥接服务，并恢复历史上下文 token。
func NewService(client Client, store StateStore, cfg Config) (*Service, error) {
	loadedState, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	if loadedState.ContextTokens == nil {
		loadedState.ContextTokens = make(map[string]string)
	}

	service := &Service{
		client:  client,
		store:   store,
		config:  cfg,
		now:     time.Now,
		syncBuf: loadedState.SyncBuf,
		state:   loadedState,
	}

	for userID, token := range loadedState.ContextTokens {
		service.client.SetContextToken(userID, token)
	}

	return service, nil
}

// GetMe 返回 Telegram Bot 风格的机器人描述。
func (s *Service) GetMe(_ context.Context) (httpapi.BotProfile, error) {
	return httpapi.BotProfile{
		ID:        s.config.BotID,
		FirstName: s.config.BotName,
		Username:  s.config.BotUsername,
	}, nil
}

// SendText 发送文本消息，要求目标 chat_id 已具备 context token。
func (s *Service) SendText(ctx context.Context, chatID string, text string) (httpapi.SentMessage, error) {
	contextToken, ok := s.client.GetContextToken(chatID)
	if !ok || strings.TrimSpace(contextToken) == "" {
		return httpapi.SentMessage{}, httpapi.ErrBadRequest("chat context token not found")
	}

	clientID, err := s.client.SendText(ctx, chatID, text, contextToken)
	if err != nil {
		return httpapi.SentMessage{}, httpapi.ErrBadGateway(err.Error())
	}

	now := s.now()
	return httpapi.SentMessage{
		MessageID: s.synthesizeMessageID(clientID, now),
		ChatID:    chatID,
		Text:      text,
		SentAt:    now,
	}, nil
}

// SendTyping 将 Telegram chat action 转换为微信 typing 状态。
func (s *Service) SendMedia(ctx context.Context, chatID string, fileName string, data []byte, caption string) (httpapi.SentMessage, error) {
	contextToken, ok := s.client.GetContextToken(chatID)
	if !ok || strings.TrimSpace(contextToken) == "" {
		return httpapi.SentMessage{}, httpapi.ErrBadRequest("chat context token not found")
	}
	if strings.TrimSpace(fileName) == "" {
		return httpapi.SentMessage{}, httpapi.ErrBadRequest("file name is required")
	}
	if len(data) == 0 {
		return httpapi.SentMessage{}, httpapi.ErrBadRequest("media payload is empty")
	}

	if err := s.client.SendMediaFile(ctx, chatID, contextToken, data, fileName, caption); err != nil {
		return httpapi.SentMessage{}, httpapi.ErrBadGateway(err.Error())
	}

	now := s.now()
	return httpapi.SentMessage{
		MessageID: s.synthesizeMessageID(chatID+":"+fileName, now),
		ChatID:    chatID,
		Text:      caption,
		SentAt:    now,
	}, nil
}

func (s *Service) SendTyping(ctx context.Context, chatID string, _ string) error {
	contextToken, ok := s.client.GetContextToken(chatID)
	if !ok || strings.TrimSpace(contextToken) == "" {
		return httpapi.ErrBadRequest("chat context token not found")
	}

	configResp, err := s.client.GetConfig(ctx, chatID, contextToken)
	if err != nil {
		return httpapi.ErrBadGateway(err.Error())
	}

	if strings.TrimSpace(configResp.TypingTicket) == "" {
		return httpapi.ErrBadGateway("typing ticket is empty")
	}

	if err = s.client.SendTyping(ctx, chatID, configResp.TypingTicket, ilink.Typing); err != nil {
		return httpapi.ErrBadGateway(err.Error())
	}

	return nil
}

// StartMonitor 持续同步微信消息上下文，并将状态落盘。
func (s *Service) StartMonitor(ctx context.Context) error {
	return s.client.Monitor(ctx, s.handleMessage, &ilink.MonitorOptions{
		InitialBuf:  s.syncBuf,
		OnBufUpdate: s.handleBufUpdate,
		OnError:     s.handleMonitorError,
	})
}

func (s *Service) handleMessage(msg ilink.WeixinMessage) {
	if msg.FromUserID == "" || msg.ContextToken == "" {
		return
	}

	s.mu.Lock()
	s.state.ContextTokens[msg.FromUserID] = msg.ContextToken
	s.mu.Unlock()

	s.client.SetContextToken(msg.FromUserID, msg.ContextToken)
	if err := s.saveState(); err != nil {
		log.Printf("[bridge] save state after message failed: %v", err)
	}
}

func (s *Service) handleBufUpdate(buf string) {
	s.mu.Lock()
	s.syncBuf = buf
	s.state.SyncBuf = buf
	s.mu.Unlock()

	if err := s.saveState(); err != nil {
		log.Printf("[bridge] save state after buf update failed: %v", err)
	}
}

func (s *Service) handleMonitorError(err error) {
	log.Printf("[bridge] monitor error: %v", err)
}

func (s *Service) saveState() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := state.State{
		SyncBuf:       s.state.SyncBuf,
		ContextTokens: make(map[string]string, len(s.state.ContextTokens)),
	}
	for userID, token := range s.state.ContextTokens {
		snapshot.ContextTokens[userID] = token
	}

	return s.store.Save(snapshot)
}

func (s *Service) synthesizeMessageID(clientID string, now time.Time) int64 {
	if clientID == "" {
		return now.UnixMilli()
	}

	hash := fnv.New32a()
	_, _ = hash.Write([]byte(clientID))
	return int64(hash.Sum32())
}
