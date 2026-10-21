package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	task "taskmanager/internal/task"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		if e = db.PingContext(ctx); e == nil {
			break
		}
		if ctx.Err() != nil {
			log.Fatalf("database not ready: %v", e)
		}
		time.Sleep(time.Second)
	}
	if e = runMigrations(ctx, db); e != nil {
		log.Fatal(e)
	}
	exporter, e := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if e != nil {
		log.Fatalf("create trace exporter: %v", e)
	}
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(tracerProvider)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			log.Printf("shut down tracing: %v", err)
		}
	}()
	gin.SetMode(env("GIN_MODE", "release"))
	r := gin.New()
	r.Use(gin.Recovery(), otelMiddleware())
	// PostgreSQL is always the durable store. Redis, when configured, wraps
	// only task-list reads and never stores the authoritative task records.
	var repo task.Repository = task.NewPostgresRepository(db)
	if address := os.Getenv("REDIS_ADDR"); address != "" {
		redisDB, err := strconv.Atoi(env("REDIS_DB", "0"))
		if err != nil || redisDB < 0 {
			log.Fatal("REDIS_DB must be a non-negative integer")
		}
		cache := task.NewRedisCache(address, os.Getenv("REDIS_PASSWORD"), redisDB)
		defer cache.Close()
		repo = task.NewCachedRepository(repo, cache, 5*time.Minute)
	}
	svc := task.NewService(repo)
	task.NewHandler(svc).Register(r)
	server := &http.Server{Addr: env("HTTP_ADDR", ":8080"), Handler: r, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-done
		c, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = server.Shutdown(c)
	}()
	log.Printf("listening on %s", server.Addr)
	if e = server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal(e)
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func runMigrations(ctx context.Context, db *sql.DB) error {
	b, e := os.ReadFile("migrations/001_create_tasks.sql")
	if e != nil {
		return e
	}
	_, e = db.ExecContext(ctx, string(b))
	return e
}
func otelMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := otel.Tracer("taskmanager/http").Start(c.Request.Context(), c.Request.Method+" "+c.FullPath(), trace.WithAttributes(attribute.String("http.method", c.Request.Method)))
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		span.SetAttributes(attribute.Int("http.status_code", c.Writer.Status()))
	}
}
