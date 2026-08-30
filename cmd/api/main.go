package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"hris/backend/internal/app"
	"hris/backend/internal/config"
	"hris/backend/internal/database"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	db, err := database.Open(cfg.DatabaseURL, cfg.Environment == "production")
	if err != nil {
		log.Error("database startup failed", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddress, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	defer redisClient.Close()
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: app.New(cfg, db, redisClient, log), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Info("api listening", "address", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
	log.Info("api stopped")
}
