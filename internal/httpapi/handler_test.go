package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakeBridge struct {
	profile          BotProfile
	lastChatID       string
	lastText         string
	lastAction       string
	lastCaption      string
	lastFileName     string
	lastMediaData    []byte
	sendTextErr      error
	sendMediaErr     error
	sendTypingErr    error
	sendTextResult   SentMessage
	sendMediaResult  SentMessage
	sendTypingCalled bool
}

func (f *fakeBridge) GetMe(_ context.Context) (BotProfile, error) {
	return f.profile, nil
}

func (f *fakeBridge) SendText(_ context.Context, chatID string, text string) (SentMessage, error) {
	f.lastChatID = chatID
	f.lastText = text
	return f.sendTextResult, f.sendTextErr
}

func (f *fakeBridge) SendMedia(_ context.Context, chatID string, method string, fileName string, data []byte, caption string) (SentMessage, error) {
	f.lastChatID = chatID
	f.lastFileName = fileName
	f.lastMediaData = append([]byte(nil), data...)
	f.lastCaption = caption
	f.lastAction = method
	return f.sendMediaResult, f.sendMediaErr
}

func (f *fakeBridge) SendTyping(_ context.Context, chatID string, action string) error {
	f.lastChatID = chatID
	f.lastAction = action
	f.sendTypingCalled = true
	return f.sendTypingErr
}

func TestGetMeReturnsTelegramCompatibleResponse(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{
		profile: BotProfile{
			ID:        42,
			FirstName: "Weixin Bridge",
			Username:  "weixin_bridge_bot",
		},
	}

	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        bridge,
		Now: func() time.Time {
			return time.Unix(1700000000, 0)
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/botsecret-token/getMe", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}

	var resp apiResponse[botResult]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !resp.OK {
		t.Fatalf("expected ok response")
	}

	if resp.Result.Username != "weixin_bridge_bot" {
		t.Fatalf("unexpected username: %s", resp.Result.Username)
	}
}

func TestSendMessageWithJSONBodyCallsBridge(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{
		sendTextResult: SentMessage{
			MessageID: 1001,
			ChatID:    "wx-user-1",
			Text:      "hello from telegram",
			SentAt:    time.Unix(1700000000, 0),
		},
	}

	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        bridge,
		Now: func() time.Time {
			return time.Unix(1700000000, 0)
		},
	})

	body := bytes.NewBufferString(`{"chat_id":"wx-user-1","text":"hello from telegram"}`)
	req := httptest.NewRequest(http.MethodPost, "/botsecret-token/sendMessage", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}

	if bridge.lastChatID != "wx-user-1" {
		t.Fatalf("unexpected chat_id: %s", bridge.lastChatID)
	}

	if bridge.lastText != "hello from telegram" {
		t.Fatalf("unexpected text: %s", bridge.lastText)
	}

	var resp apiResponse[messageResult]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !resp.OK {
		t.Fatalf("expected ok response")
	}

	if resp.Result.Text != "hello from telegram" {
		t.Fatalf("unexpected response text: %s", resp.Result.Text)
	}
}

func TestSendMessageWithFormBodyCallsBridge(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{
		sendTextResult: SentMessage{
			MessageID: 1002,
			ChatID:    "wx-user-2",
			Text:      "hello form",
			SentAt:    time.Unix(1700000001, 0),
		},
	}

	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        bridge,
	})

	form := url.Values{}
	form.Set("chat_id", "wx-user-2")
	form.Set("text", "hello form")

	req := httptest.NewRequest(http.MethodPost, "/botsecret-token/sendMessage", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}

	if bridge.lastChatID != "wx-user-2" || bridge.lastText != "hello form" {
		t.Fatalf("bridge did not receive expected payload")
	}
}

func TestSendPhotoWithMultipartBodyCallsBridge(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{
		sendMediaResult: SentMessage{
			MessageID: 2001,
			ChatID:    "wx-user-media",
			Text:      "caption text",
			SentAt:    time.Unix(1700000002, 0),
		},
	}

	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        bridge,
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("chat_id", "wx-user-media")
	_ = writer.WriteField("caption", "caption text")
	part, err := writer.CreateFormFile("photo", "photo.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = part.Write([]byte("image-bytes")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/botsecret-token/sendPhoto", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}
	if bridge.lastChatID != "wx-user-media" {
		t.Fatalf("unexpected chat_id: %s", bridge.lastChatID)
	}
	if bridge.lastAction != "sendPhoto" {
		t.Fatalf("unexpected method: %s", bridge.lastAction)
	}
	if bridge.lastFileName != "photo.jpg" {
		t.Fatalf("unexpected file name: %s", bridge.lastFileName)
	}
	if bridge.lastCaption != "caption text" {
		t.Fatalf("unexpected caption: %s", bridge.lastCaption)
	}
	if string(bridge.lastMediaData) != "image-bytes" {
		t.Fatalf("unexpected payload: %s", string(bridge.lastMediaData))
	}

	var resp apiResponse[messageResult]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Result.Caption != "caption text" {
		t.Fatalf("unexpected response caption: %s", resp.Result.Caption)
	}
}

func TestSendVoiceWithMultipartBodyCallsBridge(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{
		sendMediaResult: SentMessage{
			MessageID: 2002,
			ChatID:    "wx-user-voice",
			Text:      "voice caption",
			SentAt:    time.Unix(1700000003, 0),
		},
	}

	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        bridge,
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("chat_id", "wx-user-voice")
	_ = writer.WriteField("caption", "voice caption")
	part, err := writer.CreateFormFile("voice", "voice.ogg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = part.Write([]byte("voice-bytes")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/botsecret-token/sendVoice", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}
	if bridge.lastChatID != "wx-user-voice" {
		t.Fatalf("unexpected chat_id: %s", bridge.lastChatID)
	}
	if bridge.lastAction != "sendVoice" {
		t.Fatalf("unexpected method: %s", bridge.lastAction)
	}
	if bridge.lastFileName != "voice.ogg" {
		t.Fatalf("unexpected file name: %s", bridge.lastFileName)
	}
	if bridge.lastCaption != "voice caption" {
		t.Fatalf("unexpected caption: %s", bridge.lastCaption)
	}
	if string(bridge.lastMediaData) != "voice-bytes" {
		t.Fatalf("unexpected payload: %s", string(bridge.lastMediaData))
	}
}

func TestSendAudioWithMultipartBodyCallsBridge(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{
		sendMediaResult: SentMessage{
			MessageID: 2003,
			ChatID:    "wx-user-audio",
			Text:      "audio caption",
			SentAt:    time.Unix(1700000004, 0),
		},
	}

	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        bridge,
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("chat_id", "wx-user-audio")
	_ = writer.WriteField("caption", "audio caption")
	part, err := writer.CreateFormFile("audio", "audio.mp3")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = part.Write([]byte("audio-bytes")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/botsecret-token/sendAudio", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}
	if bridge.lastChatID != "wx-user-audio" {
		t.Fatalf("unexpected chat_id: %s", bridge.lastChatID)
	}
	if bridge.lastAction != "sendAudio" {
		t.Fatalf("unexpected method: %s", bridge.lastAction)
	}
	if bridge.lastFileName != "audio.mp3" {
		t.Fatalf("unexpected file name: %s", bridge.lastFileName)
	}
	if bridge.lastCaption != "audio caption" {
		t.Fatalf("unexpected caption: %s", bridge.lastCaption)
	}
	if string(bridge.lastMediaData) != "audio-bytes" {
		t.Fatalf("unexpected payload: %s", string(bridge.lastMediaData))
	}
}

func TestSendAnimationWithMultipartBodyCallsBridge(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{
		sendMediaResult: SentMessage{
			MessageID: 2004,
			ChatID:    "wx-user-animation",
			Text:      "animation caption",
			SentAt:    time.Unix(1700000005, 0),
		},
	}

	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        bridge,
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("chat_id", "wx-user-animation")
	_ = writer.WriteField("caption", "animation caption")
	part, err := writer.CreateFormFile("animation", "funny.gif")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = part.Write([]byte("gif-bytes")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/botsecret-token/sendAnimation", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}
	if bridge.lastChatID != "wx-user-animation" {
		t.Fatalf("unexpected chat_id: %s", bridge.lastChatID)
	}
	if bridge.lastAction != "sendAnimation" {
		t.Fatalf("unexpected method: %s", bridge.lastAction)
	}
	if bridge.lastFileName != "funny.gif" {
		t.Fatalf("unexpected file name: %s", bridge.lastFileName)
	}
	if bridge.lastCaption != "animation caption" {
		t.Fatalf("unexpected caption: %s", bridge.lastCaption)
	}
	if string(bridge.lastMediaData) != "gif-bytes" {
		t.Fatalf("unexpected payload: %s", string(bridge.lastMediaData))
	}
}

func TestSendMessageReturnsTelegramStyleError(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{
		sendTextErr: ErrBadRequest("chat context token not found"),
	}

	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        bridge,
	})

	body := bytes.NewBufferString(`{"chat_id":"wx-user-1","text":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/botsecret-token/sendMessage", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusBadRequest)
	}

	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.OK {
		t.Fatalf("expected failure response")
	}

	if !strings.Contains(resp.Description, "chat context token not found") {
		t.Fatalf("unexpected error description: %s", resp.Description)
	}
}

func TestRejectsInvalidTelegramToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        &fakeBridge{},
	})

	req := httptest.NewRequest(http.MethodGet, "/botwrong/getMe", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSendChatActionMapsToBridge(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{}
	handler := NewHandler(Options{
		TelegramToken: "secret-token",
		Bridge:        bridge,
	})

	body := bytes.NewBufferString(`{"chat_id":"wx-user-3","action":"typing"}`)
	req := httptest.NewRequest(http.MethodPost, "/botsecret-token/sendChatAction", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}

	if !bridge.sendTypingCalled {
		t.Fatalf("expected SendTyping to be called")
	}

	if bridge.lastAction != "typing" {
		t.Fatalf("unexpected action: %s", bridge.lastAction)
	}
}
