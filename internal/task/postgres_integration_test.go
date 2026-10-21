package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Set TEST_DATABASE_URL to a dedicated PostgreSQL test database to run this test.
// All test tables are created in a temporary schema and removed afterward.
func TestPostgresIntegrationCRUD(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("connect to test PostgreSQL: %v", err)
	}

	schema := fmt.Sprintf("task_test_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "001_create_tasks.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	repo := NewPostgresRepository(db)
	created, err := repo.Create(ctx, Input{Title: "  integration task  ", Description: "persisted", Assignee: "Sam"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Title != "integration task" || created.Status != Pending {
		t.Fatalf("unexpected created task: %+v", created)
	}
	loaded, err := repo.Get(ctx, created.ID)
	if err != nil || loaded.Description != "persisted" {
		t.Fatalf("Get = %+v, %v", loaded, err)
	}
	updated, err := repo.Update(ctx, created.ID, Input{Title: "updated", Status: Completed})
	if err != nil || updated.Title != "updated" || updated.Status != Completed || updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Fatalf("Update = %+v, %v", updated, err)
	}
	second, err := repo.Create(ctx, Input{Title: "second", Status: Completed, Assignee: "Sam"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, Input{Title: "other status", Status: Pending, Assignee: "Sam"}); err != nil {
		t.Fatal(err)
	}
	filter := ListOptions{Status: Completed, Assignee: "Sam", Page: 1, PageSize: 1}
	list, total, err := repo.List(ctx, filter)
	if err != nil || len(list) != 1 || total != 2 {
		t.Fatalf("filtered page 1 = %+v, total=%d, %v", list, total, err)
	}
	pageOneID := list[0].ID
	filter.Page = 2
	list, total, err = repo.List(ctx, filter)
	if err != nil || len(list) != 1 || total != 2 || list[0].ID == pageOneID || (list[0].ID != created.ID && list[0].ID != second.ID) {
		t.Fatalf("filtered page 2 = %+v, total=%d, %v", list, total, err)
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 3 {
		t.Fatalf("Count = %d, %v", count, err)
	}
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, created.ID); err != ErrNotFound {
		t.Fatalf("Get deleted task error = %v", err)
	}
	if err := repo.Delete(ctx, second.ID); err != nil {
		t.Fatal(err)
	}

	// Exercise the actual REST handlers, service, PostgreSQL repository, and
	// Redis cache invalidation together.
	gin.SetMode(gin.TestMode)
	cache := newMemoryListCache()
	cachedRepo := NewCachedRepository(repo, cache, time.Minute)
	router := gin.New()
	NewHandler(NewService(cachedRepo)).Register(router)
	doRequest := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	createdResponse := doRequest(http.MethodPost, "/tasks", `{"title":"HTTP task","assignee":"Alex"}`)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("HTTP create status = %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	var apiTask Task
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &apiTask); err != nil {
		t.Fatal(err)
	}
	listPath := "/tasks?status=pending&assignee=Alex&page=1&page_size=10"
	if response := doRequest(http.MethodGet, listPath, ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), apiTask.ID) {
		t.Fatalf("HTTP cached list = %d: %s", response.Code, response.Body.String())
	}
	if response := doRequest(http.MethodPut, "/tasks/"+apiTask.ID, `{"title":"HTTP task","status":"completed","assignee":"Alex"}`); response.Code != http.StatusOK {
		t.Fatalf("HTTP update = %d: %s", response.Code, response.Body.String())
	}
	if response := doRequest(http.MethodGet, listPath, ""); response.Code != http.StatusOK || strings.Contains(response.Body.String(), apiTask.ID) {
		t.Fatalf("stale pending list after update = %d: %s", response.Code, response.Body.String())
	}
	completePath := "/tasks?status=completed&assignee=Alex&page=1&page_size=10"
	if response := doRequest(http.MethodGet, completePath, ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), apiTask.ID) {
		t.Fatalf("completed list after update = %d: %s", response.Code, response.Body.String())
	}
	if response := doRequest(http.MethodDelete, "/tasks/"+apiTask.ID, ""); response.Code != http.StatusNoContent {
		t.Fatalf("HTTP delete = %d: %s", response.Code, response.Body.String())
	}
	if response := doRequest(http.MethodGet, completePath, ""); response.Code != http.StatusOK || strings.Contains(response.Body.String(), apiTask.ID) {
		t.Fatalf("stale list after delete = %d: %s", response.Code, response.Body.String())
	}
}
