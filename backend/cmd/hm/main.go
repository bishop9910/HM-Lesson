package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/conf"
	"github.com/bishop9910/hm-lesson/backend/internal/version"
)

func main() {
	cfg := conf.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel()}))
	slog.SetDefault(logger)

	srv := khttp.NewServer(
		khttp.Address(cfg.HTTPAddr),
		khttp.Timeout(15*time.Second),
		khttp.Middleware(recovery.Recovery()),
	)

	router := srv.Route("/")
	router.GET("/healthz", func(ctx khttp.Context) error {
		return ctx.JSON(200, map[string]string{
			"status":  "ok",
			"version": version.Version,
		})
	})
	router.GET("/readyz", func(ctx khttp.Context) error {
		// M1: 只检查进程；M2 接入 DB/存储探活。
		return ctx.JSON(200, map[string]string{"status": "ready"})
	})

	app := kratos.New(
		kratos.Name("hm-api"),
		kratos.Version(version.Version),
		kratos.Logger(logger),
		kratos.Server(srv),
	)

	logger.Info("hm-api starting",
		"addr", cfg.HTTPAddr,
		"env", cfg.Env,
		"version", version.Version,
	)
	if err := app.Run(); err != nil {
		logger.Error("hm-api stopped with error", "err", err)
		os.Exit(1)
	}
	logger.Info("hm-api stopped")
}
