package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"openilink-tg/internal/app"
	"openilink-tg/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err = app.Run(ctx, cfg); err != nil {
		log.Printf("service exited with error: %v", err)
		os.Exit(1)
	}
}
