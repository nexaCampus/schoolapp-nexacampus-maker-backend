package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/models"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/provisioner"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/security"
)

// OrchestratorHandler exposes HTTP controller methods for the School Maker control plane.
type OrchestratorHandler struct {
	orchestrator *provisioner.TenantOrchestrator
	quotaManager *provisioner.QuotaManager
	db           *sql.DB
	startTime    time.Time
}

// NewOrchestratorHandler constructs an orchestrator HTTP handler.
func NewOrchestratorHandler(
	orch *provisioner.TenantOrchestrator,
	quota *provisioner.QuotaManager,
	db *sql.DB,
) *OrchestratorHandler {
	return &OrchestratorHandler{
		orchestrator: orch,
		quotaManager: quota,
		db:           db,
		startTime:    time.Now().UTC(),
	}
}

// Healthz serves system telemetry and database health.
func (h *OrchestratorHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	dbConnected := true
	if err := h.db.PingContext(r.Context()); err != nil {
		dbConnected = false
	}

	uptime := int64(time.Since(h.startTime).Seconds())

	resp := models.HealthResponse{
		Status:            "ok",
		Service:           "nexacampus-orchestrator",
		DatabaseConnected: dbConnected,
		UptimeSeconds:     uptime,
		Memory: models.MemoryStats{
			AllocMB:      float64(m.Alloc) / 1024 / 1024,
			TotalAllocMB: float64(m.TotalAlloc) / 1024 / 1024,
			SysMB:        float64(m.Sys) / 1024 / 1024,
			NumGC:        m.NumGC,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetDeploymentOptions returns cloud tiers and enforces free-tier suppression.
func (h *OrchestratorHandler) GetDeploymentOptions(w http.ResponseWriter, r *http.Request) {
	opts, err := h.quotaManager.GetDeploymentOptions(r.Context())
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "failed to evaluate cloud quota: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(opts)
}

// ValidateSlug checks slug formatting and namespace availability.
func (h *OrchestratorHandler) ValidateSlug(w http.ResponseWriter, r *http.Request) {
	var req models.ValidateSlugRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid request json payload")
		return
	}

	res, err := h.orchestrator.ValidateSlug(r.Context(), req.Slug)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// RegisterTenant provisions an isolated school instance.
func (h *OrchestratorHandler) RegisterTenant(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "malformed registration payload")
		return
	}

	res, err := h.orchestrator.RegisterAndProvisionTenant(r.Context(), req)
	if err != nil {
		if errors.Is(err, provisioner.ErrFreeTierExhausted) {
			sendJSONError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, provisioner.ErrTenantAlreadyExists) {
			sendJSONError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, security.ErrInvalidSlugFormat) || errors.Is(err, security.ErrReservedSlug) {
			sendJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		sendJSONError(w, http.StatusInternalServerError, "tenant provisioning failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

// ListTenants enumerates all schools registered on the platform.
func (h *OrchestratorHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.orchestrator.ListTenants(r.Context())
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if tenants == nil {
		tenants = []models.Tenant{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total":   len(tenants),
		"tenants": tenants,
	})
}

// GetTenant fetches details for a specific school slug.
func (h *OrchestratorHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 {
		sendJSONError(w, http.StatusBadRequest, "missing tenant slug in url path")
		return
	}
	slug := pathParts[3]

	tenant, err := h.orchestrator.GetTenantBySlug(r.Context(), slug)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tenant == nil {
		sendJSONError(w, http.StatusNotFound, "school tenant not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tenant)
}

func sendJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   http.StatusText(status),
		"message": message,
	})
}
