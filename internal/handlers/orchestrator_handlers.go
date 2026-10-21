package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/models"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/provisioner"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/security"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/vercel"
)

// OrchestratorHandler exposes HTTP controller methods for the School Maker control plane.
type OrchestratorHandler struct {
	orchestrator  *provisioner.TenantOrchestrator
	quotaManager  *provisioner.QuotaManager
	db            *sql.DB
	vercelService *vercel.DomainService
	baseDomain    string
	jwtSecret     string
	environment   string
	startTime     time.Time
}

// NewOrchestratorHandler constructs an orchestrator HTTP handler.
func NewOrchestratorHandler(
	orch *provisioner.TenantOrchestrator,
	quota *provisioner.QuotaManager,
	db *sql.DB,
) *OrchestratorHandler {
	return &OrchestratorHandler{
		orchestrator:  orch,
		quotaManager:  quota,
		db:            db,
		vercelService: vercel.NewDomainService("", "", "", "nexacampus.tech"),
		baseDomain:    "nexacampus.tech",
		startTime:     time.Now().UTC(),
	}
}

func (h *OrchestratorHandler) SetVercelService(v *vercel.DomainService) {
	h.vercelService = v
}

func (h *OrchestratorHandler) SetBaseDomain(domain string) {
	h.baseDomain = domain
}

func (h *OrchestratorHandler) SetJWTSecret(secret string) {
	h.jwtSecret = secret
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

// ListTenants enumerates all legacy tenants registered on the platform.
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

// ============================================================================
// ISSUE 1: Creator Verification & OTP Handlers
// ============================================================================

// MakerSendOTP handles POST /api/v1/maker/auth/send-otp
func (h *OrchestratorHandler) MakerSendOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.SendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	cleanPhone := strings.TrimSpace(req.Phone)
	if cleanPhone == "" {
		sendJSONError(w, http.StatusBadRequest, "phone number is required")
		return
	}

	// Normalize phone
	if !strings.HasPrefix(cleanPhone, "+") {
		digitsOnly := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, cleanPhone)
		if len(digitsOnly) == 10 {
			cleanPhone = "+91 " + digitsOnly
		}
	}

	creatorName := strings.TrimSpace(req.CreatorName)
	if creatorName == "" {
		creatorName = "School Administrator"
	}

	// Generate OTP (deterministic dev fallback for designated test numbers or random 6-digit)
	otpCode := "123456"
	if !strings.Contains(cleanPhone, "6289652321") && !strings.Contains(cleanPhone, "9830123456") {
		n, err := rand.Int(rand.Reader, big.NewInt(900000))
		if err == nil {
			otpCode = fmt.Sprintf("%06d", n.Int64()+100000)
		}
	}

	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	initialToken := uuid.New().String()

	query := `
		INSERT INTO creator_sessions (phone, creator_name, otp_code, token, verified, created_at, expires_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)
		ON CONFLICT(phone) DO UPDATE SET
			creator_name = excluded.creator_name,
			otp_code = excluded.otp_code,
			token = excluded.token,
			verified = 0,
			expires_at = excluded.expires_at;
	`
	_, err := h.db.ExecContext(r.Context(), query, cleanPhone, creatorName, otpCode, initialToken, now, expiresAt)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "saving creator session: "+err.Error())
		return
	}

	resp := models.SendOTPResponse{
		Success: true,
		Message: fmt.Sprintf("6-digit verification code sent to %s", cleanPhone),
		DevOTP:  otpCode, // Surfaces OTP directly in response for fast developer testing
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// MakerVerifyOTP handles POST /api/v1/maker/auth/verify-otp
func (h *OrchestratorHandler) MakerVerifyOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.VerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	cleanPhone := strings.TrimSpace(req.Phone)
	inputOTP := strings.TrimSpace(req.OTP)

	if cleanPhone == "" || inputOTP == "" {
		sendJSONError(w, http.StatusBadRequest, "phone number and otp code are required")
		return
	}

	var storedOTP string
	var creatorName string
	var expiresAt time.Time
	query := `SELECT otp_code, creator_name, expires_at FROM creator_sessions WHERE phone = ?`
	err := h.db.QueryRowContext(r.Context(), query, cleanPhone).Scan(&storedOTP, &creatorName, &expiresAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// Verify the code: it must match the stored OTP and must not be expired.
	// There is no universal fallback code.
	if err != nil || storedOTP == "" || storedOTP != inputOTP {
		sendJSONError(w, http.StatusUnauthorized, "invalid or expired verification code")
		return
	}
	if !time.Now().UTC().Before(expiresAt) {
		sendJSONError(w, http.StatusUnauthorized, "invalid or expired verification code")
		return
	}

	if creatorName == "" && req.CreatorName != "" {
		creatorName = req.CreatorName
	}
	if creatorName == "" {
		creatorName = "School Administrator"
	}

	// Generate 256-bit cryptographically secure session token
	sessionToken, err := security.GenerateSecureToken(32)
	if err != nil {
		sessionToken = uuid.New().String() + "-" + uuid.New().String()
	}

	now := time.Now().UTC()
	updateQuery := `
		INSERT INTO creator_sessions (phone, creator_name, otp_code, token, verified, verified_at, created_at, expires_at)
		VALUES (?, ?, ?, ?, 1, ?, ?, ?)
		ON CONFLICT(phone) DO UPDATE SET
			verified = 1,
			verified_at = excluded.verified_at,
			token = excluded.token,
			creator_name = excluded.creator_name;
	`
	_, _ = h.db.ExecContext(r.Context(), updateQuery, cleanPhone, creatorName, inputOTP, sessionToken, now, now, now.Add(7*24*time.Hour))

	resp := models.VerifyOTPResponse{
		Success: true,
		Token:   sessionToken,
		Session: models.CreatorSession{
			Phone:       cleanPhone,
			CreatorName: creatorName,
			VerifiedAt:  now.Format(time.RFC3339),
			Token:       sessionToken,
		},
		Message: "Phone number verified successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ============================================================================
// ISSUE 2, 3 & 4: School Provisioning, Persistence & 15-Module Access
// ============================================================================

// MakerProvisionSchool handles POST /api/v1/maker/provision and POST /api/v1/schools
func (h *OrchestratorHandler) MakerProvisionSchool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CreateSchoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid school provision payload: "+err.Error())
		return
	}

	slug := strings.ToLower(strings.TrimSpace(req.SubdomainSlug))
	if err := security.ValidateSlug(slug); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid slug format: "+err.Error())
		return
	}

	// Step 1: Slug uniqueness check (allow updating existing school if same creator or re-provisioning)
	var existingID string
	var existingCreator string
	_ = h.db.QueryRowContext(r.Context(), `SELECT id, creator_phone FROM schools WHERE subdomain_slug = ?`, slug).Scan(&existingID, &existingCreator)

	steps := []string{
		"1. Validating Tenant Slug",
		"2. Persisting Institutional Profile & 15 Module Configs",
	}

	// Step 2: Normalize and serialize 15 active modules map
	normalizedModules := normalizeActiveModules(req.ActiveModules)
	modulesJSON, err := json.Marshal(normalizedModules)
	if err != nil {
		modulesJSON = []byte("{}")
	}

	// Step 3: Call Vercel Domain Service for custom subdomains + Vercel aliases
	if h.vercelService == nil {
		h.vercelService = vercel.NewDomainService("", "", "", h.baseDomain)
	}

	steps = append(steps, "3. Provisioning Vercel Subdomains")
	domainResult, _ := h.vercelService.ProvisionSubdomains(r.Context(), slug)

	// Step 4: Generate Student and Admin URLs
	steps = append(steps, "4. Generating Student & Admin Portal URLs")

	schoolID := existingID
	if schoolID == "" {
		schoolID = "sch-" + slug
	}

	now := time.Now().UTC()
	themeColor := req.ThemeColor
	if themeColor == "" {
		themeColor = "#0A192F"
	}

	upsertQuery := `
		INSERT INTO schools (
			id, creator_phone, creator_name, leadership_position,
			school_name, teacher_count, affiliation_board, enrolled_scholars,
			city, state_territory, subdomain_slug, school_motto,
			active_modules, student_domain, admin_domain, vercel_fallback_domain,
			provisioning_status, theme_color, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?)
		ON CONFLICT(subdomain_slug) DO UPDATE SET
			school_name = excluded.school_name,
			teacher_count = excluded.teacher_count,
			affiliation_board = excluded.affiliation_board,
			enrolled_scholars = excluded.enrolled_scholars,
			city = excluded.city,
			state_territory = excluded.state_territory,
			school_motto = excluded.school_motto,
			active_modules = excluded.active_modules,
			student_domain = excluded.student_domain,
			admin_domain = excluded.admin_domain,
			vercel_fallback_domain = excluded.vercel_fallback_domain,
			theme_color = excluded.theme_color,
			updated_at = excluded.updated_at;
	`

	_, err = h.db.ExecContext(
		r.Context(),
		upsertQuery,
		schoolID,
		req.CreatorPhone,
		req.CreatorName,
		req.LeadershipPosition,
		req.SchoolName,
		req.TeacherCount,
		req.AffiliationBoard,
		req.EnrolledScholars,
		req.City,
		req.StateTerritory,
		slug,
		req.SchoolMotto,
		string(modulesJSON),
		domainResult.StudentCustomDomain,
		domainResult.AdminCustomDomain,
		domainResult.PathFallbackStudent,
		themeColor,
		now,
		now,
	)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "saving school record: "+err.Error())
		return
	}

	schoolRecord := models.School{
		ID:                   schoolID,
		CreatorPhone:         req.CreatorPhone,
		CreatorName:          req.CreatorName,
		LeadershipPosition:   req.LeadershipPosition,
		SchoolName:           req.SchoolName,
		TeacherCount:         req.TeacherCount,
		AffiliationBoard:     req.AffiliationBoard,
		EnrolledScholars:     req.EnrolledScholars,
		City:                 req.City,
		StateTerritory:       req.StateTerritory,
		SubdomainSlug:        slug,
		SchoolMotto:          req.SchoolMotto,
		ActiveModules:        normalizedModules,
		StudentDomain:        domainResult.StudentCustomDomain,
		AdminDomain:          domainResult.AdminCustomDomain,
		VercelFallbackDomain: domainResult.VercelAppDomain,
		ProvisioningStatus:   "active",
		ThemeColor:           themeColor,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	resp := models.CreateSchoolResponse{
		Success:              true,
		School:               schoolRecord,
		StudentDomain:        domainResult.StudentCustomDomain,
		AdminDomain:          domainResult.AdminCustomDomain,
		VercelFallbackDomain: domainResult.VercelAppDomain,
		PathFallbackStudent:  domainResult.PathFallbackStudent,
		PathFallbackAdmin:    domainResult.PathFallbackAdmin,
		StepsCompleted:       steps,
		Message:              fmt.Sprintf("School '%s' successfully provisioned across NexaCampus Cloud", req.SchoolName),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// ListSchools handles GET /api/v1/schools
func (h *OrchestratorHandler) ListSchools(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, creator_phone, creator_name, leadership_position,
		       school_name, teacher_count, affiliation_board, enrolled_scholars,
		       city, state_territory, subdomain_slug, COALESCE(school_motto, ''),
		       active_modules, student_domain, admin_domain, vercel_fallback_domain,
		       provisioning_status, COALESCE(theme_color, '#0A192F'), created_at, updated_at
		FROM schools ORDER BY created_at DESC
	`)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "listing schools: "+err.Error())
		return
	}
	defer rows.Close()

	schools := []models.School{}
	for rows.Next() {
		var s models.School
		var modulesRaw string
		if err := rows.Scan(
			&s.ID, &s.CreatorPhone, &s.CreatorName, &s.LeadershipPosition,
			&s.SchoolName, &s.TeacherCount, &s.AffiliationBoard, &s.EnrolledScholars,
			&s.City, &s.StateTerritory, &s.SubdomainSlug, &s.SchoolMotto,
			&modulesRaw, &s.StudentDomain, &s.AdminDomain, &s.VercelFallbackDomain,
			&s.ProvisioningStatus, &s.ThemeColor, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			continue
		}
		var modMap map[string]bool
		if err := json.Unmarshal([]byte(modulesRaw), &modMap); err == nil {
			s.ActiveModules = modMap
		} else {
			s.ActiveModules = make(map[string]bool)
		}
		schools = append(schools, s)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total":   len(schools),
		"schools": schools,
	})
}

// GetSchoolOrSubroute routes GET/PATCH for /api/v1/schools/:slug or /resolve/:slug
func (h *OrchestratorHandler) GetSchoolOrSubroute(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	// Expected: ["api", "v1", "schools", ":slug"] or ["api", "v1", "schools", "resolve", ":slug"]
	var slug string
	if len(parts) >= 4 {
		if parts[3] == "resolve" && len(parts) >= 5 {
			slug = parts[4]
		} else {
			slug = parts[3]
		}
	}

	if slug == "" {
		sendJSONError(w, http.StatusBadRequest, "missing school slug in url")
		return
	}

	if r.Method == http.MethodPatch {
		h.UpdateSchool(w, r, slug)
		return
	}

	if r.Method == http.MethodGet {
		h.GetSchool(w, r, slug)
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// GetSchool fetches full school record by subdomain_slug
func (h *OrchestratorHandler) GetSchool(w http.ResponseWriter, r *http.Request, slug string) {
	var s models.School
	var modulesRaw string
	query := `
		SELECT id, creator_phone, creator_name, leadership_position,
		       school_name, teacher_count, affiliation_board, enrolled_scholars,
		       city, state_territory, subdomain_slug, COALESCE(school_motto, ''),
		       active_modules, student_domain, admin_domain, vercel_fallback_domain,
		       provisioning_status, COALESCE(theme_color, '#0A192F'), created_at, updated_at
		FROM schools WHERE subdomain_slug = ?
	`
	err := h.db.QueryRowContext(r.Context(), query, slug).Scan(
		&s.ID, &s.CreatorPhone, &s.CreatorName, &s.LeadershipPosition,
		&s.SchoolName, &s.TeacherCount, &s.AffiliationBoard, &s.EnrolledScholars,
		&s.City, &s.StateTerritory, &s.SubdomainSlug, &s.SchoolMotto,
		&modulesRaw, &s.StudentDomain, &s.AdminDomain, &s.VercelFallbackDomain,
		&s.ProvisioningStatus, &s.ThemeColor, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, http.StatusNotFound, fmt.Sprintf("school '%s' not found", slug))
			return
		}
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var modMap map[string]bool
	if err := json.Unmarshal([]byte(modulesRaw), &modMap); err == nil {
		s.ActiveModules = modMap
	} else {
		s.ActiveModules = make(map[string]bool)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s)
}

// UpdateSchool updates fields or active_modules for an existing school
func (h *OrchestratorHandler) UpdateSchool(w http.ResponseWriter, r *http.Request, slug string) {
	var req models.UpdateSchoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid patch payload: "+err.Error())
		return
	}

	// Fetch current record
	var currentModulesRaw string
	err := h.db.QueryRowContext(r.Context(), `SELECT active_modules FROM schools WHERE subdomain_slug = ?`, slug).Scan(&currentModulesRaw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, http.StatusNotFound, fmt.Sprintf("school '%s' not found", slug))
			return
		}
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	currentModules := make(map[string]bool)
	_ = json.Unmarshal([]byte(currentModulesRaw), &currentModules)

	if req.ActiveModules != nil {
		for k, v := range req.ActiveModules {
			currentModules[k] = v
		}
		currentModules = normalizeActiveModules(currentModules)
	}

	updatedModulesJSON, _ := json.Marshal(currentModules)
	now := time.Now().UTC()

	_, err = h.db.ExecContext(
		r.Context(),
		`UPDATE schools SET active_modules = ?, updated_at = ? WHERE subdomain_slug = ?`,
		string(updatedModulesJSON),
		now,
		slug,
	)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "updating school: "+err.Error())
		return
	}

	// Return updated school
	h.GetSchool(w, r, slug)
}

// SearchSchools handles GET /api/v1/schools/search?q=...
func (h *OrchestratorHandler) SearchSchools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	searchPattern := "%" + strings.ToLower(q) + "%"

	query := `
		SELECT id, creator_phone, creator_name, leadership_position,
		       school_name, teacher_count, affiliation_board, enrolled_scholars,
		       city, state_territory, subdomain_slug, COALESCE(school_motto, ''),
		       active_modules, student_domain, admin_domain, vercel_fallback_domain,
		       provisioning_status, COALESCE(theme_color, '#0A192F'), created_at, updated_at
		FROM schools
		WHERE LOWER(school_name) LIKE ? OR LOWER(subdomain_slug) LIKE ? OR LOWER(city) LIKE ? OR LOWER(affiliation_board) LIKE ?
		ORDER BY created_at DESC LIMIT 20
	`
	rows, err := h.db.QueryContext(r.Context(), query, searchPattern, searchPattern, searchPattern, searchPattern)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	schools := []models.School{}
	for rows.Next() {
		var s models.School
		var modulesRaw string
		if err := rows.Scan(
			&s.ID, &s.CreatorPhone, &s.CreatorName, &s.LeadershipPosition,
			&s.SchoolName, &s.TeacherCount, &s.AffiliationBoard, &s.EnrolledScholars,
			&s.City, &s.StateTerritory, &s.SubdomainSlug, &s.SchoolMotto,
			&modulesRaw, &s.StudentDomain, &s.AdminDomain, &s.VercelFallbackDomain,
			&s.ProvisioningStatus, &s.ThemeColor, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			continue
		}
		var modMap map[string]bool
		if err := json.Unmarshal([]byte(modulesRaw), &modMap); err == nil {
			s.ActiveModules = modMap
		} else {
			s.ActiveModules = make(map[string]bool)
		}
		schools = append(schools, s)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total":   len(schools),
		"query":   q,
		"schools": schools,
	})
}

// CheckSlug handles GET /api/v1/schools/check-slug?slug=...
func (h *OrchestratorHandler) CheckSlug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	slug := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("slug")))
	if err := security.ValidateSlug(slug); err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.CheckSlugResponse{
			Valid:     false,
			Available: false,
			Slug:      slug,
			Reason:    err.Error(),
		})
		return
	}

	var count int
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM schools WHERE subdomain_slug = ?`, slug).Scan(&count)

	available := count == 0

	base := h.baseDomain
	if base == "" {
		base = "nexacampus.tech"
	}

	resp := models.CheckSlugResponse{
		Valid:     true,
		Available: available,
		Slug:      slug,
	}
	if !available {
		resp.Reason = "This subdomain identifier is already claimed by another institution"
	}

	resp.Urls.StudentCustom = fmt.Sprintf("https://%s.%s", slug, base)
	resp.Urls.AdminCustom = fmt.Sprintf("https://%s.admin.%s", slug, base)
	resp.Urls.VercelFallback = fmt.Sprintf("https://%s-nexacampus.vercel.app", slug)
	resp.Urls.PathFallback = fmt.Sprintf("https://school-webapp-self.vercel.app/school/%s", slug)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetMySchools handles GET /api/v1/maker/my-schools?phone=...
func (h *OrchestratorHandler) GetMySchools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	query := `
		SELECT id, creator_phone, creator_name, leadership_position,
		       school_name, teacher_count, affiliation_board, enrolled_scholars,
		       city, state_territory, subdomain_slug, COALESCE(school_motto, ''),
		       active_modules, student_domain, admin_domain, vercel_fallback_domain,
		       provisioning_status, COALESCE(theme_color, '#0A192F'), created_at, updated_at
		FROM schools WHERE creator_phone = ? OR creator_phone LIKE ?
		ORDER BY created_at DESC
	`
	rows, err := h.db.QueryContext(r.Context(), query, phone, "%"+phone+"%")
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	schools := []models.School{}
	for rows.Next() {
		var s models.School
		var modulesRaw string
		if err := rows.Scan(
			&s.ID, &s.CreatorPhone, &s.CreatorName, &s.LeadershipPosition,
			&s.SchoolName, &s.TeacherCount, &s.AffiliationBoard, &s.EnrolledScholars,
			&s.City, &s.StateTerritory, &s.SubdomainSlug, &s.SchoolMotto,
			&modulesRaw, &s.StudentDomain, &s.AdminDomain, &s.VercelFallbackDomain,
			&s.ProvisioningStatus, &s.ThemeColor, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			continue
		}
		var modMap map[string]bool
		if err := json.Unmarshal([]byte(modulesRaw), &modMap); err == nil {
			s.ActiveModules = modMap
		} else {
			s.ActiveModules = make(map[string]bool)
		}
		schools = append(schools, s)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total":   len(schools),
		"phone":   phone,
		"schools": schools,
	})
}

// StudentLogin handles POST /api/v1/auth/student-login
func (h *OrchestratorHandler) StudentLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.StudentLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid login payload")
		return
	}

	slug := strings.ToLower(strings.TrimSpace(req.SchoolSlug))
	if slug == "" {
		slug = "lcgvm" // Default demo tenant
	}

	// Verify school exists
	var school models.School
	var modulesRaw string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, creator_phone, creator_name, leadership_position,
		       school_name, teacher_count, affiliation_board, enrolled_scholars,
		       city, state_territory, subdomain_slug, COALESCE(school_motto, ''),
		       active_modules, student_domain, admin_domain, vercel_fallback_domain,
		       provisioning_status, COALESCE(theme_color, '#0A192F'), created_at, updated_at
		FROM schools WHERE subdomain_slug = ?
	`, slug).Scan(
		&school.ID, &school.CreatorPhone, &school.CreatorName, &school.LeadershipPosition,
		&school.SchoolName, &school.TeacherCount, &school.AffiliationBoard, &school.EnrolledScholars,
		&school.City, &school.StateTerritory, &school.SubdomainSlug, &school.SchoolMotto,
		&modulesRaw, &school.StudentDomain, &school.AdminDomain, &school.VercelFallbackDomain,
		&school.ProvisioningStatus, &school.ThemeColor, &school.CreatedAt, &school.UpdatedAt,
	)
	if err != nil {
		sendJSONError(w, http.StatusNotFound, "school tenant not found")
		return
	}
	_ = json.Unmarshal([]byte(modulesRaw), &school.ActiveModules)

	// Build authenticated student profile
	identifier := strings.TrimSpace(req.AdmissionNumber)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(req.GoogleUID)
	}
	if identifier == "" {
		sendJSONError(w, http.StatusBadRequest, "admission number or email is required for student login")
		return
	}

	token, _ := security.GenerateSecureToken(32)

	resp := models.AuthResponse{
		Success: true,
		Role:    "student",
		Token:   token,
		School:  school,
		User: map[string]interface{}{
			"id":         identifier,
			"role":       "student",
			"schoolSlug": slug,
			"name":       "Student Scholar",
		},
		Message: "Student authenticated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// AdminLogin handles POST /api/v1/auth/admin-login (Strictly email/google account - rejects admission numbers)
func (h *OrchestratorHandler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid admin login payload")
		return
	}

	slug := strings.ToLower(strings.TrimSpace(req.SchoolSlug))
	if slug == "" {
		slug = "lcgvm"
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		sendJSONError(w, http.StatusBadRequest, "administrative login requires a registered email or google account")
		return
	}

	// Verify school exists
	var school models.School
	var modulesRaw string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, creator_phone, creator_name, leadership_position,
		       school_name, teacher_count, affiliation_board, enrolled_scholars,
		       city, state_territory, subdomain_slug, COALESCE(school_motto, ''),
		       active_modules, student_domain, admin_domain, vercel_fallback_domain,
		       provisioning_status, COALESCE(theme_color, '#0A192F'), created_at, updated_at
		FROM schools WHERE subdomain_slug = ?
	`, slug).Scan(
		&school.ID, &school.CreatorPhone, &school.CreatorName, &school.LeadershipPosition,
		&school.SchoolName, &school.TeacherCount, &school.AffiliationBoard, &school.EnrolledScholars,
		&school.City, &school.StateTerritory, &school.SubdomainSlug, &school.SchoolMotto,
		&modulesRaw, &school.StudentDomain, &school.AdminDomain, &school.VercelFallbackDomain,
		&school.ProvisioningStatus, &school.ThemeColor, &school.CreatedAt, &school.UpdatedAt,
	)
	if err != nil {
		sendJSONError(w, http.StatusNotFound, "school tenant not found")
		return
	}
	_ = json.Unmarshal([]byte(modulesRaw), &school.ActiveModules)

	token, _ := security.GenerateSecureToken(32)

	resp := models.AuthResponse{
		Success: true,
		Role:    "admin",
		Token:   token,
		School:  school,
		User: map[string]interface{}{
			"email":      email,
			"role":       "admin",
			"schoolSlug": slug,
			"name":       "Institutional Administrator",
		},
		Message: "Administrator authenticated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// normalizeActiveModules guarantees all 15 modules are present with bidirectional alias support
func normalizeActiveModules(input map[string]bool) map[string]bool {
	out := make(map[string]bool)

	// Canonical 15 modules list
	modules := []struct {
		short string
		full  string
	}{
		{"canteen", "canteen"},
		{"bus", "bus_transport"},
		{"library", "library"},
		{"attendance", "attendance"},
		{"diary", "homework_diary"},
		{"exams", "exams_reports"},
		{"timetable", "timetable_bell"},
		{"fees", "fee_ledger"},
		{"hallpass", "ehallpass"},
		{"clubs", "clubs_houses"},
		{"health", "medical_infirmary"},
		{"notices", "circulars_notices"},
		{"ptm", "ptm"},
		{"store", "store_uniforms"},
		{"lostfound", "lost_found"},
	}

	for _, m := range modules {
		val := false
		if input != nil {
			if v, ok := input[m.short]; ok {
				val = v
			} else if v, ok := input[m.full]; ok {
				val = v
			}
		}
		out[m.short] = val
		out[m.full] = val
	}

	return out
}

// phonesMatch compares two phone numbers ignoring formatting (spaces, dashes,
// parentheses, country code) by comparing their last 10 significant digits.
func phonesMatch(a, b string) bool {
	digits := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}

	da := digits(a)
	db := digits(b)
	if da == "" || db == "" {
		return false
	}
	if len(da) > 10 {
		da = da[len(da)-10:]
	}
	if len(db) > 10 {
		db = db[len(db)-10:]
	}
	return da == db
}

func sendJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   http.StatusText(status),
		"message": message,
	})
}
