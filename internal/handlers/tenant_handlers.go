package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/models"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/security"
)

// TenantHandler manages public and administrative endpoints for an isolated school tenant.
type TenantHandler struct {
	db         *sql.DB
	tenantSlug string
	jwtSecret  []byte
	startTime  time.Time
}

// NewTenantHandler constructs a tenant HTTP controller.
func NewTenantHandler(db *sql.DB, tenantSlug string, jwtSecret []byte) *TenantHandler {
	return &TenantHandler{
		db:         db,
		tenantSlug: tenantSlug,
		jwtSecret:  jwtSecret,
		startTime:  time.Now().UTC(),
	}
}

// Healthz reports tenant service health and resource usage (<40MB RAM).
func (h *TenantHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	dbConnected := true
	if err := h.db.PingContext(r.Context()); err != nil {
		dbConnected = false
	}

	uptime := int64(time.Since(h.startTime).Seconds())

	resp := models.HealthResponse{
		Status:            "ok",
		Service:           "nexacampus-tenant-runtime",
		Tenant:            h.tenantSlug,
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

// GetMeta returns institutional profile, branding, and contact info.
func (h *TenantHandler) GetMeta(w http.ResponseWriter, r *http.Request) {
	var meta models.SchoolMeta
	query := `
		SELECT id, name, COALESCE(tagline, ''), COALESCE(crest_url, ''), COALESCE(affiliation, ''),
		       COALESCE(contact_email, ''), COALESCE(phone, ''), COALESCE(address, ''),
		       COALESCE(theme_primary_color, '#3b49df'), COALESCE(theme_secondary_color, '#12131a'),
		       COALESCE(banner_url, ''), updated_at
		FROM school_meta WHERE id = 1
	`
	err := h.db.QueryRowContext(r.Context(), query).Scan(
		&meta.ID, &meta.Name, &meta.Tagline, &meta.CrestURL, &meta.Affiliation,
		&meta.ContactEmail, &meta.Phone, &meta.Address,
		&meta.ThemePrimaryColor, &meta.ThemeSecondaryColor, &meta.BannerURL, &meta.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Fallback defaults
			meta = models.SchoolMeta{
				ID:                  1,
				Name:                "NexaCampus Institutional Academy",
				Tagline:             "Excellence in Global Education",
				ThemePrimaryColor:   "#3b49df",
				ThemeSecondaryColor: "#12131a",
				UpdatedAt:           time.Now().UTC(),
			}
		} else {
			sendJSONError(w, http.StatusInternalServerError, "database error reading school meta: "+err.Error())
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meta)
}

// GetNotices lists official notices with pagination.
func (h *TenantHandler) GetNotices(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	category := strings.TrimSpace(r.URL.Query().Get("category"))

	var rows *sql.Rows
	var err error

	if category != "" {
		query := `SELECT id, title, content, category, COALESCE(attachment_url, ''), is_pinned, created_at, updated_at FROM notices WHERE category = ? ORDER BY is_pinned DESC, created_at DESC LIMIT ?`
		rows, err = h.db.QueryContext(r.Context(), query, category, limit)
	} else {
		query := `SELECT id, title, content, category, COALESCE(attachment_url, ''), is_pinned, created_at, updated_at FROM notices ORDER BY is_pinned DESC, created_at DESC LIMIT ?`
		rows, err = h.db.QueryContext(r.Context(), query, limit)
	}

	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "failed to query notices: "+err.Error())
		return
	}
	defer rows.Close()

	var notices []models.Notice
	for rows.Next() {
		var n models.Notice
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.Category, &n.AttachmentURL, &n.IsPinned, &n.CreatedAt, &n.UpdatedAt); err != nil {
			sendJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		notices = append(notices, n)
	}

	if notices == nil {
		notices = []models.Notice{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total":   len(notices),
		"notices": notices,
	})
}

// GetDepartments lists faculty departments.
func (h *TenantHandler) GetDepartments(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, name, COALESCE(head_of_department, ''), faculty_count, COALESCE(description, '') FROM departments ORDER BY name ASC`)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var depts []models.Department
	for rows.Next() {
		var d models.Department
		if err := rows.Scan(&d.ID, &d.Name, &d.HeadOfDepartment, &d.FacultyCount, &d.Description); err != nil {
			sendJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		depts = append(depts, d)
	}

	if depts == nil {
		depts = []models.Department{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(depts)
}

// SubmitAdmission processes incoming student enquiries with validation.
func (h *TenantHandler) SubmitAdmission(w http.ResponseWriter, r *http.Request) {
	var adm models.Admission
	if err := json.NewDecoder(r.Body).Decode(&adm); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid admission payload")
		return
	}

	if strings.TrimSpace(adm.StudentName) == "" {
		sendJSONError(w, http.StatusBadRequest, "student_name is required")
		return
	}
	if strings.TrimSpace(adm.ParentName) == "" {
		sendJSONError(w, http.StatusBadRequest, "parent_name is required")
		return
	}
	if strings.TrimSpace(adm.Email) == "" || !strings.Contains(adm.Email, "@") {
		sendJSONError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	if strings.TrimSpace(adm.Phone) == "" {
		sendJSONError(w, http.StatusBadRequest, "contact phone is required")
		return
	}

	adm.ID = uuid.New().String()
	adm.Status = "pending"
	adm.SubmittedAt = time.Now().UTC()

	query := `
		INSERT INTO admissions (id, student_name, parent_name, email, phone, grade_applying, previous_school, status, notes, submitted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := h.db.ExecContext(r.Context(), query,
		adm.ID, adm.StudentName, adm.ParentName, adm.Email, adm.Phone,
		adm.GradeApplying, adm.PreviousSchool, adm.Status, adm.Notes, adm.SubmittedAt,
	)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "failed to record admission enquiry: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"admission_id": adm.ID,
		"message":      "Enquiry received. The academic admissions cell will contact you shortly.",
	})
}

// AdminLogin validates bcrypt credentials and issues an HMAC-SHA256 JWT.
func (h *TenantHandler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var creds models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid login credentials payload")
		return
	}

	username := strings.TrimSpace(creds.Username)
	if username == "" || creds.Password == "" {
		sendJSONError(w, http.StatusUnauthorized, "username and password are required")
		return
	}

	var admin models.Admin
	query := `SELECT id, username, password_hash, created_at FROM admins WHERE username = ?`
	err := h.db.QueryRowContext(r.Context(), query, username).Scan(
		&admin.ID, &admin.Username, &admin.PasswordHash, &admin.CreatedAt,
	)
	if err != nil {
		sendJSONError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if !security.CheckPasswordHash(creds.Password, admin.PasswordHash) {
		sendJSONError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	tokenDuration := 24 * time.Hour
	token, err := security.GenerateJWT(admin.ID, admin.Username, h.tenantSlug, h.jwtSecret, tokenDuration)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "failed to sign authentication token: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models.LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(tokenDuration),
		Username:  admin.Username,
		Role:      "tenant_admin",
	})
}

// AdminCreateNotice publishes a new bulletin.
func (h *TenantHandler) AdminCreateNotice(w http.ResponseWriter, r *http.Request) {
	var n models.Notice
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid notice payload")
		return
	}

	if strings.TrimSpace(n.Title) == "" || strings.TrimSpace(n.Content) == "" {
		sendJSONError(w, http.StatusBadRequest, "title and content are required")
		return
	}

	n.ID = uuid.New().String()
	now := time.Now().UTC()
	n.CreatedAt = now
	n.UpdatedAt = now
	if n.Category == "" {
		n.Category = "General"
	}

	query := `INSERT INTO notices (id, title, content, category, attachment_url, is_pinned, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := h.db.ExecContext(r.Context(), query, n.ID, n.Title, n.Content, n.Category, n.AttachmentURL, n.IsPinned, n.CreatedAt, n.UpdatedAt)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "saving notice: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(n)
}

// AdminUpdateNotice updates an existing notice.
func (h *TenantHandler) AdminUpdateNotice(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 5 {
		sendJSONError(w, http.StatusBadRequest, "missing notice id")
		return
	}
	noticeID := pathParts[4]

	var n models.Notice
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid notice payload")
		return
	}

	now := time.Now().UTC()
	query := `UPDATE notices SET title = ?, content = ?, category = ?, attachment_url = ?, is_pinned = ?, updated_at = ? WHERE id = ?`
	res, err := h.db.ExecContext(r.Context(), query, n.Title, n.Content, n.Category, n.AttachmentURL, n.IsPinned, now, noticeID)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		sendJSONError(w, http.StatusNotFound, "notice not found")
		return
	}

	n.ID = noticeID
	n.UpdatedAt = now
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(n)
}

// AdminDeleteNotice deletes a circular.
func (h *TenantHandler) AdminDeleteNotice(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 5 {
		sendJSONError(w, http.StatusBadRequest, "missing notice id")
		return
	}
	noticeID := pathParts[4]

	query := `DELETE FROM notices WHERE id = ?`
	res, err := h.db.ExecContext(r.Context(), query, noticeID)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		sendJSONError(w, http.StatusNotFound, "notice not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "deleted_id": noticeID})
}

// AdminGetAdmissions lists enquiries submitted by prospective students.
func (h *TenantHandler) AdminGetAdmissions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, student_name, parent_name, email, phone, grade_applying, COALESCE(previous_school, ''), status, COALESCE(notes, ''), submitted_at FROM admissions ORDER BY submitted_at DESC LIMIT 100`)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var admissions []models.Admission
	for rows.Next() {
		var a models.Admission
		if err := rows.Scan(&a.ID, &a.StudentName, &a.ParentName, &a.Email, &a.Phone, &a.GradeApplying, &a.PreviousSchool, &a.Status, &a.Notes, &a.SubmittedAt); err != nil {
			sendJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		admissions = append(admissions, a)
	}

	if admissions == nil {
		admissions = []models.Admission{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total":      len(admissions),
		"admissions": admissions,
	})
}

// AdminUpdateTheme updates branding colors, tagline, or banner URL.
func (h *TenantHandler) AdminUpdateTheme(w http.ResponseWriter, r *http.Request) {
	var req models.UpdateThemeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid theme payload")
		return
	}

	now := time.Now().UTC()
	query := `
		INSERT INTO school_meta (id, name, theme_primary_color, theme_secondary_color, tagline, banner_url, updated_at)
		VALUES (1, 'NexaCampus Institutional Academy', ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			theme_primary_color = COALESCE(NULLIF(excluded.theme_primary_color, ''), school_meta.theme_primary_color),
			theme_secondary_color = COALESCE(NULLIF(excluded.theme_secondary_color, ''), school_meta.theme_secondary_color),
			tagline = COALESCE(NULLIF(excluded.tagline, ''), school_meta.tagline),
			banner_url = COALESCE(NULLIF(excluded.banner_url, ''), school_meta.banner_url),
			updated_at = excluded.updated_at
	`
	_, err := h.db.ExecContext(r.Context(), query, req.ThemePrimaryColor, req.ThemeSecondaryColor, req.Tagline, req.BannerURL, now)
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, "failed to update theme: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Institutional theme updated successfully.",
	})
}
