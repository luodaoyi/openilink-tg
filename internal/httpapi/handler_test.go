package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
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
	sendTextErr      error
	sendTypingErr    error
	sendTextResult   SentMessage
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
