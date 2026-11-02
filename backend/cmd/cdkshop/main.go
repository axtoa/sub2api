package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/cdkshop"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server, err := cdkshop.OpenServer(ctx, required("CDKSHOP_DATABASE_URL"), required("CDKSHOP_ENCRYPTION_KEY"), required("CDKSHOP_AUTH_SECRET"), required("CDKSHOP_EASYPAY_API_BASE"), required("CDKSHOP_EASYPAY_PID"), required("CDKSHOP_EASYPAY_KEY"), required("CDKSHOP_PUBLIC_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			log.Printf("close CDK shop: %v", err)
		}
	}()
	addr := strings.TrimSpace(os.Getenv("CDKSHOP_LISTEN"))
	if addr == "" {
		addr = ":8082"
	}
	httpServer := &http.Server{Addr: addr, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	log.Printf("CDK shop listening on %s", addr)
	if err = httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func required(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}
