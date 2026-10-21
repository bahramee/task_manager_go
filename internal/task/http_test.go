package task

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func testRouter(repo *fakeRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(NewService(repo)).Register(r)
	return r
}

func TestHTTPCreateAndValidation(t *testing.T) {
	repo := &fakeRepo{}
	r := testRouter(repo)

	request := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"title":"  plan  "}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"title":"plan"`) || !strings.Contains(response.Body.String(), `"status":"pending"`) {
		t.Fatalf("create response = %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") == "" {
		t.Fatal("create response did not set Location")
	}

	request = httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"title":" "}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status = %d", response.Code)
	}
}

func TestHTTPReadUpdateDeleteAndErrors(t *testing.T) {
	repo := &fakeRepo{getTask: Task{ID: "00000000-0000-4000-8000-000000000001", Title: "demo"}}
	r := testRouter(repo)

	for _, tc := range []struct {
		method string
		path   string
		body   string
		want   int
	}{
		{http.MethodGet, "/tasks/00000000-0000-4000-8000-000000000001", "", http.StatusOK},
		{http.MethodPut, "/tasks/00000000-0000-4000-8000-000000000001", `{"title":"updated"}`, http.StatusOK},
		{http.MethodDelete, "/tasks/00000000-0000-4000-8000-000000000001", "", http.StatusNoContent},
		{http.MethodGet, "/tasks/missing", "", http.StatusBadRequest},
		{http.MethodGet, "/tasks", "", http.StatusOK},
	} {
		repo.err = nil
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if response.Code != tc.want {
			t.Errorf("%s %s status = %d, want %d; body=%s", tc.method, tc.path, response.Code, tc.want, response.Body.String())
		}
	}

	repo.err = ErrNotFound
	request := httptest.NewRequest(http.MethodGet, "/tasks/00000000-0000-4000-8000-000000000099", nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing task status = %d", response.Code)
	}

	repo.err = errors.New("database unavailable")
	request = httptest.NewRequest(http.MethodGet, "/tasks/00000000-0000-4000-8000-000000000001", nil)
	response = httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "database unavailable") {
		t.Fatalf("unexpected internal-error response: %d %s", response.Code, response.Body.String())
	}
}

func TestMetricsEndpoint(t *testing.T) {
	r := testRouter(&fakeRepo{})
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tasks", nil))
	response = httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", response.Code)
	}
	for _, metric := range []string{"requests_total", "request_latency_histogram", "tasks_count"} {
		if !strings.Contains(response.Body.String(), metric) {
			t.Errorf("metrics response missing %q", metric)
		}
	}
}

func TestHTTPListFiltersAndPagination(t *testing.T) {
	repo := &fakeRepo{}
	r := testRouter(repo)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tasks?status=completed&assignee=Sam&page=2&page_size=5", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", response.Code, response.Body.String())
	}
	if repo.listOptions.Status != Completed || repo.listOptions.Assignee != "Sam" || repo.listOptions.Page != 2 || repo.listOptions.PageSize != 5 {
		t.Fatalf("list options = %+v", repo.listOptions)
	}
	if !strings.Contains(response.Body.String(), `"total_pages":1`) {
		t.Fatalf("list response missing pagination metadata: %s", response.Body.String())
	}
	response = httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tasks?page=0", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid page status = %d", response.Code)
	}
}

func TestOpenAPISpecEndpoints(t *testing.T) {
	r := testRouter(&fakeRepo{})
	for _, path := range []string{"/swagger", "/openapi.yaml"} {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "openapi: 3.0.3") {
			t.Errorf("%s returned status %d and body %q", path, response.Code, response.Body.String())
		}
	}
}
