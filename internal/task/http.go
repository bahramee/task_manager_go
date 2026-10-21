package task

import (
	"context"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }

type apiError struct {
	Error string `json:"error"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validID(c *gin.Context) bool {
	if uuidPattern.MatchString(c.Param("id")) {
		return true
	}
	writeError(c, http.StatusBadRequest, "invalid task id")
	return false
}

func writeError(c *gin.Context, code int, msg string) { c.JSON(code, apiError{msg}) }
func (h *Handler) Register(r *gin.Engine) {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "requests_total", Help: "HTTP requests by method, route, and status."}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "request_latency_histogram", Help: "HTTP request duration in seconds.", Buckets: prometheus.DefBuckets}, []string{"method", "route"})
	tasks := prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "tasks_count", Help: "Current number of tasks stored in PostgreSQL."}, func() float64 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		n, e := h.service.Count(ctx)
		if e != nil {
			return 0
		}
		return float64(n)
	})
	registry := prometheus.NewRegistry()
	registry.MustRegister(requests, duration, tasks)
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		requests.WithLabelValues(c.Request.Method, route, http.StatusText(c.Writer.Status())).Inc()
		duration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	})
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(registry, promhttp.HandlerOpts{})))
	r.GET("/swagger", func(c *gin.Context) { c.File(openAPISpecPath()) })
	r.GET("/openapi.yaml", func(c *gin.Context) { c.File(openAPISpecPath()) })
	r.GET("/tasks", h.list)
	r.POST("/tasks", h.create)
	r.GET("/tasks/:id", h.get)
	r.PUT("/tasks/:id", h.update)
	r.DELETE("/tasks/:id", h.delete)
}

func openAPISpecPath() string {
	for _, path := range []string{"openapi.yaml", "../../openapi.yaml"} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "openapi.yaml"
}
func (h *Handler) list(c *gin.Context) {
	ctx, span := otel.Tracer("taskmanager").Start(c.Request.Context(), "tasks.list")
	defer span.End()
	options, err := listOptions(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid list parameters")
		return
	}
	items, total, e := h.service.List(ctx, options)
	if errors.Is(e, ErrInvalid) {
		writeError(c, http.StatusBadRequest, "invalid list parameters")
		return
	}
	if e != nil {
		writeError(c, 500, "internal server error")
		return
	}
	pages := (total + options.PageSize - 1) / options.PageSize
	c.JSON(200, gin.H{"items": items, "page": options.Page, "page_size": options.PageSize, "total": total, "total_pages": pages})
}

func listOptions(c *gin.Context) (ListOptions, error) {
	options := ListOptions{Status: Status(c.Query("status")), Assignee: c.Query("assignee"), Page: 1, PageSize: 20}
	if value := c.Query("page"); value != "" {
		page, err := strconv.Atoi(value)
		if err != nil || page < 1 {
			return options, ErrInvalid
		}
		options.Page = page
	}
	if value := c.Query("page_size"); value != "" {
		size, err := strconv.Atoi(value)
		if err != nil || size < 1 {
			return options, ErrInvalid
		}
		options.PageSize = size
	}
	return options, nil
}
func (h *Handler) create(c *gin.Context) {
	var in Input
	if e := c.ShouldBindJSON(&in); e != nil {
		writeError(c, 400, "invalid request body")
		return
	}
	v, e := h.service.Create(c.Request.Context(), in)
	if errors.Is(e, ErrInvalid) {
		writeError(c, 400, "invalid task")
		return
	}
	if e != nil {
		writeError(c, 500, "internal server error")
		return
	}
	c.Header("Location", "/tasks/"+v.ID)
	c.JSON(201, v)
}
func (h *Handler) get(c *gin.Context) {
	if !validID(c) {
		return
	}
	v, e := h.service.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(e, ErrNotFound) {
		writeError(c, 404, "task not found")
		return
	}
	if e != nil {
		writeError(c, 500, "internal server error")
		return
	}
	c.JSON(200, v)
}
func (h *Handler) update(c *gin.Context) {
	if !validID(c) {
		return
	}
	var in Input
	if e := c.ShouldBindJSON(&in); e != nil {
		writeError(c, 400, "invalid request body")
		return
	}
	v, e := h.service.Update(c.Request.Context(), c.Param("id"), in)
	if errors.Is(e, ErrInvalid) {
		writeError(c, 400, "invalid task")
		return
	}
	if errors.Is(e, ErrNotFound) {
		writeError(c, 404, "task not found")
		return
	}
	if e != nil {
		writeError(c, 500, "internal server error")
		return
	}
	c.JSON(200, v)
}
func (h *Handler) delete(c *gin.Context) {
	if !validID(c) {
		return
	}
	e := h.service.Delete(c.Request.Context(), c.Param("id"))
	if errors.Is(e, ErrNotFound) {
		writeError(c, 404, "task not found")
		return
	}
	if e != nil {
		writeError(c, 500, "internal server error")
		return
	}
	c.Status(204)
}
