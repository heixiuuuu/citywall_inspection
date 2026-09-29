// Command server 是 Go 业务服务的入口：加载配置、启动 HTTP 服务并优雅退出。
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"citywall/server/internal/bootstrap"
	"citywall/server/internal/config"
)

func main() {
	configPath := flag.String("config", "conf/server.json", "path to server config (or set CITY_WALL_CONFIG)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	app, err := bootstrap.New(cfg)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer func() { _ = app.Close() }()

	srv := &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      app.Router(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutS) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutS) * time.Second,
	}

	// 优雅退出：等待 SIGINT/SIGTERM。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("[server] %s listening on %s (env=%s)", cfg.App.Name, cfg.Server.Listen, cfg.App.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.GracefulShutdownS)*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Print("[server] stopped")
	os.Exit(0)
}
