package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"bolty.studio/internal/studio"
	"bolty.studio/web"
)

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	password := os.Getenv("APP_PASSWORD")
	if len(password) < 20 {
		log.Error("APP_PASSWORD must be at least 20 characters; generate a random studio password")
		os.Exit(1)
	}
	origin := env("PUBLIC_URL", "http://127.0.0.1:8080")
	secure, e := studio.ValidateOrigin(origin)
	if e != nil {
		log.Error("invalid origin", "error", e)
		os.Exit(1)
	}
	dataDir := env("DATA_DIR", "./data")
	if e = os.MkdirAll(dataDir, 0700); e != nil {
		log.Error("data directory failed", "error", e)
		os.Exit(1)
	}
	// Exactly one process owns this studio volume. Queue claiming is transactional,
	// but this lock also avoids unsafe cross-process restart recovery.
	lock, e := os.OpenFile(filepath.Join(dataDir, "studio.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		log.Error("volume lock failed", "error", e)
		os.Exit(1)
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		log.Error("another Bolty process is using this data volume; single replica required")
		os.Exit(1)
	}
	provider := env("DEFAULT_PROVIDER", "openai")
	model := os.Getenv("OPENAI_MODEL")
	if provider == "anthropic" {
		model = os.Getenv("ANTHROPIC_MODEL")
	}
	role := studio.Role{Provider: provider, Model: model}
	settings := studio.Settings{Writer: role, Judge: role, Researcher: role, PassScore: 85, MinDimension: 7, MaxCalls: 30, DailyCalls: 90}
	store, e := studio.OpenStore(filepath.Join(dataDir, "studio.db"), settings)
	if e != nil {
		log.Error("database startup failed", "error", e)
		os.Exit(1)
	}
	defer store.DB.Close()
	providers := studio.NewProviders(os.Getenv("OPENAI_API_KEY"), os.Getenv("ANTHROPIC_API_KEY"))
	engine := &studio.Engine{Store: store, Providers: providers, YouTube: studio.YouTubeCredentials{ClientID: os.Getenv("YOUTUBE_CLIENT_ID"), ClientSecret: os.Getenv("YOUTUBE_CLIENT_SECRET"), RefreshToken: os.Getenv("YOUTUBE_REFRESH_TOKEN")}, Log: log}
	app := &studio.Server{Store: store, Engine: engine, Assets: web.Assets, Password: password, Origin: origin, Secure: secure, Python: env("PYTHON_BIN", "python3"), Renderer: env("PDF_RENDERER", "./tools/render.py"), Log: log}
	server := &http.Server{Addr: env("LISTEN_ADDR", "127.0.0.1:8080"), Handler: app.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 75 * time.Second, MaxHeaderBytes: 16384}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); engine.Worker(ctx) }()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	log.Info("Bolty Studio ready", "origin", origin, "address", server.Addr, "policy", studio.PolicyVersion, "providers_configured", os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("ANTHROPIC_API_KEY") != "")
	if e = server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Error("server failed", "error", e)
		stop()
	}
	stop()
	<-workerDone
}
