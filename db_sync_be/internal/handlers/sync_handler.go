package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"db-sync-scheduler/internal/services"
)

// Handler holds service dependencies
type Handler struct {
	syncService *services.SyncService
}

func NewHandler(syncService *services.SyncService) *Handler {
	return &Handler{
		syncService: syncService,
	}
}

type Response struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type ConfigRequest struct {
	CronSchedule   string `json:"cronSchedule,omitempty"`
	BatchSize      int    `json:"batchSize,omitempty"`
	AutoSchemaSync *bool  `json:"autoSchemaSync,omitempty"`
}

func (h *Handler) StartSyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := h.syncService.StartSync()
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	sendSuccessResponse(w, "Sync service started", nil)
}

func (h *Handler) StopSyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := h.syncService.StopSync()
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	sendSuccessResponse(w, "Sync service stopped", nil)
}

func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := h.syncService.GetStatus()
	sendSuccessResponse(w, "", status)
}

func (h *Handler) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var configReq ConfigRequest
	err := json.NewDecoder(r.Body).Decode(&configReq)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.syncService.UpdateConfig(configReq.CronSchedule, configReq.BatchSize, configReq.AutoSchemaSync)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	status := h.syncService.GetStatus()
	sendSuccessResponse(w, "Configuration updated", status)
}

func (h *Handler) SchemaSyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := h.syncService.TriggerSchemaSync()
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Schema synchronization completed", nil)
}

func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	sendSuccessResponse(w, "Service is running", nil)
}

func (h *Handler) RootHandler(w http.ResponseWriter, r *http.Request) {
	endpoints := map[string]string{
		"health":       "GET /health",
		"startSync":    "POST /api/sync/start",
		"stopSync":     "POST /api/sync/stop",
		"status":       "GET /api/sync/status",
		"updateConfig": "PUT /api/sync/config",
		"schemaSync":   "POST /api/schema/sync",
	}

	response := Response{
		Success: true,
		Message: "Database Sync Scheduler Service with Auto Schema Sync",
		Data:    map[string]interface{}{"endpoints": endpoints},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	response := Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := Response{
		Success: false,
		Error:   message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
