package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tinypulse/internal/db"
	"tinypulse/internal/monitor"
	"tinypulse/internal/notifier"
	"tinypulse/internal/server"
)

func getEnvOrDefault(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func main() {
	var (
		addr   = flag.String("addr", getEnvOrDefault("TINYPULSE_ADDR", ":8080"), "HTTP listen address")
		dbPath = flag.String("db", getEnvOrDefault("TINYPULSE_DB", "./tinypulse.db"), "SQLite database path")
		pass   = flag.String("password", getEnvOrDefault("TINYPULSE_PASSWORD", ""), "Admin password for UI and API")
	)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	database, err := db.New(*dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	dispatcher := notifier.NewDispatcher(database)
	manager := monitor.New(database, dispatcher)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := manager.Start(ctx); err != nil {
		slog.Error("failed to start manager", "error", err)
		os.Exit(1)
	}

	srv := server.New(database, manager, *pass)
	httpServer := &http.Server{
		Addr:         *addr,
		Handler:      srv.Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", *addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully")

	manager.Stop()
	dispatcher.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	slog.Info("shutdown complete")
	fmt.Println("TinyPulse stopped.")
}
