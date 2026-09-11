package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/steled/budget-book/internal/auth"
	"github.com/steled/budget-book/internal/database"
	"github.com/steled/budget-book/internal/handlers"
	"github.com/steled/budget-book/internal/scheduler"
)

//go:embed templates static
var embeddedFS embed.FS

var version = "dev"

func main() {
	logger := slog.Default()

	username := getenv("APP_USERNAME", "admin")
	password := mustenv("APP_PASSWORD")
	sessionSecret := mustenv("APP_SESSION_SECRET")
	if len(sessionSecret) < 32 {
		logger.Error("APP_SESSION_SECRET must be at least 32 characters (use: openssl rand -hex 32)")
		os.Exit(1)
	}
	secureCookies := os.Getenv("APP_SECURE_COOKIES") == "true"
	dbPath := getenv("DATABASE_PATH", "/data/budget-book.db")
	addr := getenv("APP_ADDR", ":8080")

	db, err := database.Open(dbPath)
	if err != nil {
		logger.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	a, err := auth.New(username, password, sessionSecret, secureCookies)
	if err != nil {
		logger.Error("failed to configure auth", "err", err)
		os.Exit(1)
	}

	staticSubFS, err := fs.Sub(embeddedFS, "static")
	if err != nil {
		logger.Error("failed to create static sub-fs", "err", err)
		os.Exit(1)
	}

	h, err := handlers.New(db, a, embeddedFS, version, logger)
	if err != nil {
		logger.Error("failed to create handlers", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	h.Register(mux)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticSubFS)))

	srv := &http.Server{
		Addr:         addr,
		Handler:      securityHeaders(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sched := scheduler.New(db, logger)
	go sched.Run(ctx)

	logger.Info("starting server", "addr", addr, "version", version)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self'; "+
				"img-src 'self' data:; "+
				"font-src 'self';")
		next.ServeHTTP(w, r)
	})
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable not set", "var", key)
		os.Exit(1)
	}
	return v
}
