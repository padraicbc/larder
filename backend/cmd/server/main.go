package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/padraicbc/larder/internal/api"
	"github.com/padraicbc/larder/internal/store"
	"github.com/padraicbc/larder/internal/webui"
)

func main() {
	addr := flag.String("addr", envOr("NUTRI_ADDR", ":8080"), "listen address")
	dataPath := flag.String("data", envOr("NUTRI_DATA", "data/nutrition.json"), "path to JSON data file")
	static := flag.String("static", envOr("NUTRI_STATIC", ""), "serve the web app from this dir instead of the embedded copy (e.g. ../web/build)")
	signup := flag.Bool("signup", envOr("NUTRI_SIGNUP", "open") != "closed", "allow new accounts after the first (NUTRI_SIGNUP=closed to disable)")
	trustProxy := flag.Bool("trust-proxy", envOr("NUTRI_TRUST_PROXY", "") == "1", "use X-Forwarded-For for the client IP (only behind your own reverse proxy)")
	secure := flag.Bool("secure-cookies", envOr("NUTRI_SECURE_COOKIES", "") == "1", "mark the session cookie Secure (use behind HTTPS)")
	flag.Parse()

	st, err := store.Open(*dataPath)
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	secret := []byte(os.Getenv("NUTRI_JWT_SECRET"))
	if len(secret) == 0 {
		// Persisted in the data file so restarts don't sign everyone out.
		// Set NUTRI_JWT_SECRET in production so the secret isn't stored next to the data.
		slog.Warn("NUTRI_JWT_SECRET not set; using a generated secret stored in the data file")
		if secret, err = st.JWTSecret(); err != nil {
			slog.Error("jwt secret", "err", err)
			os.Exit(1)
		}
	} else if len(secret) < 32 {
		slog.Error("NUTRI_JWT_SECRET must be at least 32 bytes (try: openssl rand -hex 32)")
		os.Exit(1)
	}

	srv := api.New(st, secret)
	srv.EnvAPIKey = os.Getenv("NUTRI_LLM_API_KEY")
	if *static != "" {
		srv.StaticFS = os.DirFS(*static)
	} else if ui, ok := webui.FS(); ok {
		srv.StaticFS = ui
	} else {
		slog.Warn("no web app embedded; run `make build`, or pass -static, or use `make web` for development")
	}
	srv.SignupOpen = *signup
	srv.SecureCookies = *secure
	srv.TrustProxy = *trustProxy

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: plan generation can take a while on local models.
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		slog.Info("listening", "addr", *addr, "data", *dataPath)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
