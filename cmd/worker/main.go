package main

import (
	"github.com/hibiken/asynq"
	"hris/backend/internal/config"
	"log/slog"
	"os"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	server := asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.RedisAddress, Password: cfg.RedisPassword, DB: cfg.RedisDB}, asynq.Config{Concurrency: 5})
	mux := asynq.NewServeMux()
	log.Info("worker started")
	if err := server.Run(mux); err != nil {
		log.Error("worker failed", "error", err)
		os.Exit(1)
	}
}
