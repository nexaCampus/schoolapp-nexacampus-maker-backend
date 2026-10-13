package models

import "time"

// SchoolMeta represents the school's public institutional profile.
type SchoolMeta struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	Tagline             string    `json:"tagline"`
	CrestURL            string    `json:"crest_url"`
	Affiliation         string    `json:"affiliation"`
	ContactEmail        string    `json:"contact_email"`
	Phone               string    `json:"phone"`
	Address             string    `json:"address"`
	ThemePrimaryColor   string    `json:"theme_primary_color"`
	ThemeSecondaryColor string    `json:"theme_secondary_color"`
	BannerURL           string    `json:"banner_url"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Notice represents a circular or official bulletin.
type Notice struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	Category      string    `json:"category"`
	AttachmentURL string    `json:"attachment_url,omitempty"`
	IsPinned      bool      `json:"is_pinned"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Department represents an academic department.
type Department struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	HeadOfDepartment  string `json:"head_of_department"`
	FacultyCount      int    `json:"faculty_count"`
	Description       string `json:"description"`
}

// Admission represents an enquiry from a prospective student or parent.
type Admission struct {
	ID             string    `json:"id"`
	StudentName    string    `json:"student_name"`
	ParentName     string    `json:"parent_name"`
	Email          string    `json:"email"`
	Phone          string    `json:"phone"`
	GradeApplying  string    `json:"grade_applying"`
	PreviousSchool string    `json:"previous_school,omitempty"`
	Status         string    `json:"status"` // pending, approved, waitlisted, rejected
	Notes          string    `json:"notes,omitempty"`
	SubmittedAt    time.Time `json:"submitted_at"`
}

// Admin represents an authenticated administrator account.
type Admin struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// LoginRequest credentials.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse contains the issued JWT token.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
}

// UpdateThemeRequest payload.
type UpdateThemeRequest struct {
	ThemePrimaryColor   string `json:"theme_primary_color"`
	ThemeSecondaryColor string `json:"theme_secondary_color"`
	Tagline             string `json:"tagline,omitempty"`
	BannerURL           string `json:"banner_url,omitempty"`
}

// HealthResponse telemetry structure.
type HealthResponse struct {
	Status            string      `json:"status"`
	Service           string      `json:"service"`
	Tenant            string      `json:"tenant,omitempty"`
	DatabaseConnected bool        `json:"database_connected"`
	UptimeSeconds     int64       `json:"uptime_seconds"`
	Memory            MemoryStats `json:"memory"`
}

// MemoryStats conveys heap and system allocation metrics.
type MemoryStats struct {
	AllocMB      float64 `json:"alloc_mb"`
	TotalAllocMB float64 `json:"total_alloc_mb"`
	SysMB        float64 `json:"sys_mb"`
	NumGC        uint32  `json:"num_gc"`
}
