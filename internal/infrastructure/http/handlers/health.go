package handlers

import (
	"database/sql"
	"net/http"
	"runtime"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/http/response"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type HealthHandler struct {
	db        *sql.DB
	redis     *redis.Client
	log       *logger.Logger
	startTime time.Time
}

func NewHealthHandler(db *sql.DB, redis *redis.Client, log *logger.Logger) *HealthHandler {
	return &HealthHandler{
		db:        db,
		redis:     redis,
		log:       log,
		startTime: time.Now().UTC(),
	}
}

type MemoryMetrics struct {
	Alloc      uint64 `json:"alloc"`
	TotalAlloc uint64 `json:"total_alloc"`
	Sys        uint64 `json:"sys"`
	NumGC      uint32 `json:"num_gc"`
}

type ServicesStatus struct {
	App      string `json:"app"`
	Database string `json:"database"`
	Redis    string `json:"redis"`
}

type HealthData struct {
	ServicesStatus ServicesStatus `json:"services_status"`
	Uptime         string         `json:"uptime"`
	Memory         MemoryMetrics  `json:"memory"`
	Goroutines     int            `json:"goroutines"`
}

func (h *HealthHandler) HandleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "UP"
		if err := h.db.Ping(); err != nil {
			dbStatus = "DOWN"
		}

		redisStatus := "UP"
		if err := h.redis.Ping(r.Context()).Err(); err != nil {
			redisStatus = "DOWN"
		}

		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		data := HealthData{
			ServicesStatus: ServicesStatus{
				App:      "UP",
				Database: dbStatus,
				Redis:    redisStatus,
			},
			Uptime: time.Since(h.startTime).String(),
			Memory: MemoryMetrics{
				Alloc:      mem.Alloc,
				TotalAlloc: mem.TotalAlloc,
				Sys:        mem.Sys,
				NumGC:      mem.NumGC,
			},
			Goroutines: runtime.NumGoroutine(),
		}

		response.WriteSuccess(w, data)
	}
}
