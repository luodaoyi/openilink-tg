package bridge

import (
	"context"
	"errors"
	"testing"
	"time"

	ilink "github.com/openilink/openilink-sdk-go"

	"openilink-tg/internal/httpapi"
	"openilink-tg/internal/state"
)

type fakeClient struct {
	contextTokens       map[string]string
	lastSendTo          string
	lastSendText        string
	lastSendContext     string
	lastMediaFileName   string
	lastMediaCaption    string
	lastMediaData       []byte
	sendTextResult      string
	sendTextErr         error
	sendMediaErr        error
	getConfigResult     *ilink.GetConfigResp
	getConfigErr        error
	sendTypingErr       error
	lastTypingUser      string
	lastTypingTicket    string
	lastTypingStatus    ilink.TypingStatus
	monitorMessage      ilink.WeixinMessage
	monitorUpdatedBuf   string
	monitorErr          error
	setContextTokenHits int
}

func (f *fakeClient) SendText(_ context.Context, to string, text string, contextToken string) (string, error) {
	f.lastSendTo = to
	f.lastSendText = text
	f.lastSendContext = contextToken
	return f.sendTextResult, f.sendTextErr
}

func (f *fakeClient) SendMediaFile(_ context.Context, to string, contextToken string, data []byte, fileName string, caption string) error {
	f.lastSendTo = to
	f.lastSendContext = contextToken
	f.lastMediaData = append([]byte(nil), data...)
	f.lastMediaFileName = fileName
	f.lastMediaCaption = caption
	return f.sendMediaErr
}

func (f *fakeClient) GetContextToken(userID string) (string, bool) {
	token, ok := f.contextTokens[userID]
	return token, ok
}

func (f *fakeClient) SetContextToken(userID string, token string) {
	if f.contextTokens == nil {
		f.contextTokens = make(map[string]string)
	}
	f.contextTokens[userID] = token
	f.setContextTokenHits++
}

func (f *fakeClient) GetConfig(_ context.Context, userID string, contextToken string) (*ilink.GetConfigResp, error) {
	if f.getConfigResult == nil && f.getConfigErr == nil {
		return &ilink.GetConfigResp{TypingTicket: "typing-ticket"}, nil
	}
	return f.getConfigResult, f.getConfigErr
}

func (f *fakeClient) SendTyping(_ context.Context, userID string, typingTicket string, status ilink.TypingStatus) error {
	f.lastTypingUser = userID
	f.lastTypingTicket = typingTicket
	f.lastTypingStatus = status
	return f.sendTypingErr
}

func (f *fakeClient) Monitor(ctx context.Context, handler ilink.MessageHandler, opts *ilink.MonitorOptions) error {
	if opts != nil && opts.OnBufUpdate != nil && f.monitorUpdatedBuf != "" {
		opts.OnBufUpdate(f.monitorUpdatedBuf)
	}
	if handler != nil && f.monitorMessage.FromUserID != "" {
		handler(f.monitorMessage)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return f.monitorErr
	}
}

type fakeStore struct {
	state     state.State
	saveCalls int
	loadErr   error
	saveErr   error
}

func (f *fakeStore) Load() (state.State, error) {
	return f.state, f.loadErr
}

func (f *fakeStore) Save(next state.State) error {
	f.saveCalls++
	f.state = next
	return f.saveErr
}

func TestNewServiceRestoresStateIntoClient(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	store := &fakeStore{
		state: state.State{
			SyncBuf: "cursor-1",
			ContextTokens: map[string]string{
				"wx-user-1": "ctx-1",
			},
		},
	}

	service, err := NewService(client, store, Config{
		BotID:       42,
		BotName:     "Weixin Bridge",
		BotUsername: "weixin_bridge_bot",
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if service.syncBuf != "cursor-1" {
		t.Fatalf("unexpected sync buf: %s", service.syncBuf)
	}

	if client.contextTokens["wx-user-1"] != "ctx-1" {
		t.Fatalf("expected context token restored")
	}
}

func TestSendTextReturnsBadRequestWhenContextMissing(t *testing.T) {
	t.Parallel()

	service, err := NewService(&fakeClient{}, &fakeStore{}, Config{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = service.SendText(context.Background(), "wx-user-1", "hello")
	if err == nil {
		t.Fatalf("expected error")
	}

	var apiErr *httpapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}

	if apiErr.Code != 400 {
		t.Fatalf("unexpected error code: %d", apiErr.Code)
	}
}

func TestSendTextUsesCachedContextToken(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		contextTokens: map[string]string{
			"wx-user-1": "ctx-1",
		},
		sendTextResult: "client-msg-1",
	}

	service, err := NewService(client, &fakeStore{}, Config{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	sent, err := service.SendText(context.Background(), "wx-user-1", "hello")
	if err != nil {
		t.Fatalf("send text: %v", err)
	}

	if client.lastSendContext != "ctx-1" {
		t.Fatalf("unexpected context token: %s", client.lastSendContext)
	}

	if sent.ChatID != "wx-user-1" || sent.Text != "hello" {
		t.Fatalf("unexpected send result: %+v", sent)
	}
}

func TestSendMediaUsesCachedContextToken(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		contextTokens: map[string]string{
			"wx-user-1": "ctx-1",
		},
	}

	service, err := NewService(client, &fakeStore{}, Config{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	sent, err := service.SendMedia(context.Background(), "wx-user-1", "sendPhoto", "photo.jpg", []byte("image-bytes"), "hi")
	if err != nil {
		t.Fatalf("send media: %v", err)
	}

	if client.lastSendContext != "ctx-1" {
		t.Fatalf("unexpected context token: %s", client.lastSendContext)
	}
	if client.lastMediaFileName != "photo.jpg" {
		t.Fatalf("unexpected file name: %s", client.lastMediaFileName)
	}
	if client.lastMediaCaption != "hi" {
		t.Fatalf("unexpected caption: %s", client.lastMediaCaption)
	}
	if string(client.lastMediaData) != "image-bytes" {
		t.Fatalf("unexpected media payload: %s", string(client.lastMediaData))
	}
	if sent.ChatID != "wx-user-1" || sent.Text != "hi" {
		t.Fatalf("unexpected send result: %+v", sent)
	}
}

func TestSendVoiceUsesVoicePath(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		contextTokens: map[string]string{
			"wx-user-1": "ctx-1",
		},
	}

	service, err := NewService(client, &fakeStore{}, Config{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	sent, err := service.SendMedia(context.Background(), "wx-user-1", "sendVoice", "voice.ogg", []byte("voice-bytes"), "voice caption")
	if err != nil {
		t.Fatalf("send media: %v", err)
	}

	if client.lastSendContext != "ctx-1" {
		t.Fatalf("unexpected context token: %s", client.lastSendContext)
	}
	if client.lastMediaFileName != "voice.ogg" {
		t.Fatalf("unexpected file name: %s", client.lastMediaFileName)
	}
	if client.lastMediaCaption != "voice caption" {
		t.Fatalf("unexpected caption: %s", client.lastMediaCaption)
	}
	if string(client.lastMediaData) != "voice-bytes" {
		t.Fatalf("unexpected media payload: %s", string(client.lastMediaData))
	}
	if sent.ChatID != "wx-user-1" || sent.Text != "voice caption" {
		t.Fatalf("unexpected send result: %+v", sent)
	}
}

func TestSendAnimationUsesGenericMediaPath(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		contextTokens: map[string]string{
			"wx-user-1": "ctx-1",
		},
	}

	service, err := NewService(client, &fakeStore{}, Config{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	sent, err := service.SendMedia(context.Background(), "wx-user-1", "sendAnimation", "funny.gif", []byte("gif-bytes"), "gif caption")
	if err != nil {
		t.Fatalf("send media: %v", err)
	}

	if client.lastSendContext != "ctx-1" {
		t.Fatalf("unexpected context token: %s", client.lastSendContext)
	}
	if client.lastMediaFileName != "funny.gif" {
		t.Fatalf("unexpected file name: %s", client.lastMediaFileName)
	}
	if client.lastMediaCaption != "gif caption" {
		t.Fatalf("unexpected caption: %s", client.lastMediaCaption)
	}
	if string(client.lastMediaData) != "gif-bytes" {
		t.Fatalf("unexpected media payload: %s", string(client.lastMediaData))
	}
	if sent.ChatID != "wx-user-1" || sent.Text != "gif caption" {
		t.Fatalf("unexpected send result: %+v", sent)
	}
}

func TestSendTypingUsesTypingTicket(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		contextTokens: map[string]string{
			"wx-user-1": "ctx-1",
		},
		getConfigResult: &ilink.GetConfigResp{
			TypingTicket: "typing-ticket-1",
		},
	}

	service, err := NewService(client, &fakeStore{}, Config{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err = service.SendTyping(context.Background(), "wx-user-1", "typing"); err != nil {
		t.Fatalf("send typing: %v", err)
	}

	if client.lastTypingTicket != "typing-ticket-1" {
		t.Fatalf("unexpected typing ticket: %s", client.lastTypingTicket)
	}

	if client.lastTypingStatus != ilink.Typing {
		t.Fatalf("unexpected typing status: %d", client.lastTypingStatus)
	}
}

func TestStartMonitorPersistsState(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		monitorUpdatedBuf: "cursor-2",
		monitorMessage: ilink.WeixinMessage{
			FromUserID:   "wx-user-1",
			ContextToken: "ctx-2",
		},
	}
	store := &fakeStore{}

	service, err := NewService(client, store, Config{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = service.StartMonitor(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("start monitor: %v", err)
	}

	if store.state.SyncBuf != "cursor-2" {
		t.Fatalf("unexpected sync buf: %s", store.state.SyncBuf)
	}

	if store.state.ContextTokens["wx-user-1"] != "ctx-2" {
		t.Fatalf("unexpected stored context token")
	}

	if store.saveCalls == 0 {
		t.Fatalf("expected state to be saved")
	}
}

func TestStartMonitorPersistsStateWhenVoicePayloadIncludesEncodeType(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		monitorMessage: ilink.WeixinMessage{
			FromUserID:   "wx-user-voice",
			ContextToken: "ctx-voice",
			ItemList: []ilink.MessageItem{
				{
					Type: ilink.ItemVoice,
					VoiceItem: &ilink.VoiceItem{
						EncodeType: ilink.VoiceFormatSILK,
						SampleRate: 24000,
						PlayTime:   3,
					},
				},
			},
		},
	}
	store := &fakeStore{}

	service, err := NewService(client, store, Config{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = service.StartMonitor(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("start monitor: %v", err)
	}

	if got := store.state.ContextTokens["wx-user-voice"]; got != "ctx-voice" {
		t.Fatalf("unexpected stored context token: %s", got)
	}

	if client.contextTokens["wx-user-voice"] != "ctx-voice" {
		t.Fatalf("expected client context token to be updated")
	}
}

func TestGetMeReturnsConfiguredProfile(t *testing.T) {
	t.Parallel()

	service, err := NewService(&fakeClient{}, &fakeStore{}, Config{
		BotID:       42,
		BotName:     "Weixin Bridge",
		BotUsername: "weixin_bridge_bot",
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	profile, err := service.GetMe(context.Background())
	if err != nil {
		t.Fatalf("get me: %v", err)
	}

	if profile.ID != 42 || profile.Username != "weixin_bridge_bot" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

func TestSendTextFallsBackToCurrentTimeWhenMessageIDNeedsSynthesizing(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		contextTokens: map[string]string{
			"wx-user-1": "ctx-1",
		},
	}

	service, err := NewService(client, &fakeStore{}, Config{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	service.now = func() time.Time {
		return time.Unix(1700000000, 0)
	}

	sent, err := service.SendText(context.Background(), "wx-user-1", "hello")
	if err != nil {
		t.Fatalf("send text: %v", err)
	}

	if sent.MessageID == 0 {
		t.Fatalf("expected synthesized message id")
	}
}
