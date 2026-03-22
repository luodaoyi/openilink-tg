package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	ilink "github.com/openilink/openilink-sdk-go"

	"openilink-tg/internal/bridge"
	"openilink-tg/internal/config"
	"openilink-tg/internal/httpapi"
	"openilink-tg/internal/state"
	"openilink-tg/internal/version"
)

// Run 组装全部运行时依赖并启动 HTTP 服务。
func Run(ctx context.Context, cfg config.Config) error {
	options := []ilink.Option{
		ilink.WithVersion(version.Version),
	}
	if cfg.ILinkBaseURL != "" {
		options = append(options, ilink.WithBaseURL(cfg.ILinkBaseURL))
	}

	client := ilink.NewClient(cfg.ILinkToken, options...)

	store := state.NewFileStore(cfg.StateFile)
	service, err := bridge.NewService(client, store, bridge.Config{
		BotID:       cfg.BotID,
		BotName:     cfg.BotName,
		BotUsername: cfg.BotUsername,
	})
	if err != nil {
		return err
	}

	handler := httpapi.NewHandler(httpapi.Options{
		TelegramToken: cfg.TelegramToken,
		Bridge:        service,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if cfg.MonitorEnabled {
		go runMonitor(ctx, service)
	}

	go shutdownOnContextDone(ctx, server)

	log.Printf("[app] listening on %s", cfg.HTTPAddr)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func runMonitor(ctx context.Context, service *bridge.Service) {
	err := service.StartMonitor(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("[app] monitor stopped with error: %v", err)
	}
}

func shutdownOnContextDone(ctx context.Context, server *http.Server) {
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("[app] shutdown failed: %v", err)
	}
}
