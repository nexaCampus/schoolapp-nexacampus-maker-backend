package models

import "time"

// Tenant represents an onboarded school tenant in the master catalog.
type Tenant struct {
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Plan          string    `json:"plan"`
	Provider      string    `json:"provider"`
	Status        string    `json:"status"`
	ServiceID     string    `json:"service_id,omitempty"`
	BackendURL    string    `json:"backend_url,omitempty"`
	CustomDomain  string    `json:"custom_domain,omitempty"`
	AdminUsername string    `json:"admin_username"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Deployment records an execution run by the cloud deployer.
type Deployment struct {
	ID         string    `json:"id"`
	TenantSlug string    `json:"tenant_slug"`
	Provider   string    `json:"provider"`
	Status     string    `json:"status"`
	ServiceID  string    `json:"service_id,omitempty"`
	Logs       string    `json:"logs,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateTenantRequest captures the user's submission from the School Maker wizard.
type CreateTenantRequest struct {
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	Plan          string            `json:"plan"` // "free", "starter", "byo"
	Provider      string            `json:"provider"` // "render", "vercel", "aws", "local"
	AdminUsername string            `json:"admin_username"`
	AdminPassword string            `json:"admin_password"`
	AdminEmail    string            `json:"admin_email"`
	BYOApiKey     string            `json:"byo_api_key,omitempty"`
	Theme         map[string]string `json:"theme,omitempty"`
}

// CreateTenantResponse conveys the provisioned school details back to the client.
type CreateTenantResponse struct {
	Success          bool      `json:"success"`
	Tenant           Tenant    `json:"tenant"`
	DirectBackendURL string    `json:"direct_backend_url"`
	SubdomainURL     string    `json:"subdomain_url"`
	InitialAdminUser string    `json:"initial_admin_user"`
	InitialAdminPass string    `json:"initial_admin_pass,omitempty"`
	DeploymentID     string    `json:"deployment_id"`
	Message          string    `json:"message"`
	CreatedAt        time.Time `json:"created_at"`
}

// DeploymentOptionsResponse informs the frontend wizard which tiers and providers are available.
type DeploymentOptionsResponse struct {
	FreeTierAvailable bool     `json:"free_tier_available"`
	FreeTierConsumed  bool     `json:"free_tier_consumed"`
	ActiveProviders   []string `json:"active_providers"`
	AvailablePlans    []Plan   `json:"available_plans"`
	Message           string   `json:"message,omitempty"`
}

// Plan represents an available hosting plan.
type Plan struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RAMBudget   string `json:"ram_budget"`
	Available   bool   `json:"available"`
	RequiresBYO bool   `json:"requires_byo"`
}

// ValidateSlugRequest payload.
type ValidateSlugRequest struct {
	Slug string `json:"slug"`
}

// ValidateSlugResponse payload.
type ValidateSlugResponse struct {
	Valid     bool   `json:"valid"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// CreatorSession represents a verified creator's session.
type CreatorSession struct {
	Phone       string `json:"phone"`
	CreatorName string `json:"creatorName"`
	VerifiedAt  string `json:"verifiedAt"`
	Token       string `json:"token,omitempty"`
}

type SendOTPRequest struct {
	Phone       string `json:"phone"`
	CreatorName string `json:"creator_name"`
}

type SendOTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	DevOTP  string `json:"dev_otp,omitempty"`
}

type VerifyOTPRequest struct {
	Phone       string `json:"phone"`
	OTP         string `json:"otp"`
	CreatorName string `json:"creator_name"`
}

type VerifyOTPResponse struct {
	Success bool           `json:"success"`
	Token   string         `json:"token"`
	Session CreatorSession `json:"session"`
	Message string         `json:"message,omitempty"`
}

// School represents an institutional tenant profile.
type School struct {
	ID                   string          `json:"id"`
	CreatorPhone         string          `json:"creator_phone"`
	CreatorName          string          `json:"creator_name"`
	LeadershipPosition   string          `json:"leadership_position"`
	SchoolName           string          `json:"school_name"`
	TeacherCount         string          `json:"teacher_count"`
	AffiliationBoard     string          `json:"affiliation_board"`
	EnrolledScholars     string          `json:"enrolled_scholars"`
	City                 string          `json:"city"`
	StateTerritory       string          `json:"state_territory"`
	SubdomainSlug        string          `json:"subdomain_slug"`
	SchoolMotto          string          `json:"school_motto"`
	ActiveModules        map[string]bool `json:"active_modules"`
	StudentDomain        string          `json:"student_domain"`
	AdminDomain          string          `json:"admin_domain"`
	VercelFallbackDomain string          `json:"vercel_fallback_domain"`
	ProvisioningStatus   string          `json:"provisioning_status"`
	ThemeColor           string          `json:"theme_color"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type CreateSchoolRequest struct {
	CreatorPhone       string          `json:"creator_phone"`
	CreatorName        string          `json:"creator_name"`
	LeadershipPosition string          `json:"leadership_position"`
	SchoolName         string          `json:"school_name"`
	TeacherCount       string          `json:"teacher_count"`
	AffiliationBoard   string          `json:"affiliation_board"`
	EnrolledScholars   string          `json:"enrolled_scholars"`
	City               string          `json:"city"`
	StateTerritory     string          `json:"state_territory"`
	SubdomainSlug      string          `json:"subdomain_slug"`
	SchoolMotto        string          `json:"school_motto"`
	ActiveModules      map[string]bool `json:"active_modules"`
	ThemeColor         string          `json:"theme_color,omitempty"`
}

type CreateSchoolResponse struct {
	Success              bool     `json:"success"`
	School               School   `json:"school"`
	StudentDomain        string   `json:"student_domain"`
	AdminDomain          string   `json:"admin_domain"`
	VercelFallbackDomain string   `json:"vercel_fallback_domain"`
	PathFallbackStudent  string   `json:"path_fallback_student"`
	PathFallbackAdmin    string   `json:"path_fallback_admin"`
	StepsCompleted       []string `json:"steps_completed"`
	Message              string   `json:"message"`
}

type CheckSlugResponse struct {
	Valid     bool   `json:"valid"`
	Available bool   `json:"available"`
	Slug      string `json:"slug"`
	Reason    string `json:"reason,omitempty"`
	Urls      struct {
		StudentCustom  string `json:"student_custom"`
		AdminCustom    string `json:"admin_custom"`
		VercelFallback string `json:"vercel_fallback"`
		PathFallback   string `json:"path_fallback"`
	} `json:"urls"`
}

type UpdateSchoolRequest struct {
	SchoolName       *string         `json:"school_name,omitempty"`
	TeacherCount     *string         `json:"teacher_count,omitempty"`
	AffiliationBoard *string         `json:"affiliation_board,omitempty"`
	EnrolledScholars *string         `json:"enrolled_scholars,omitempty"`
	City             *string         `json:"city,omitempty"`
	StateTerritory   *string         `json:"state_territory,omitempty"`
	SchoolMotto      *string         `json:"school_motto,omitempty"`
	ActiveModules    map[string]bool `json:"active_modules,omitempty"`
	ThemeColor       *string         `json:"theme_color,omitempty"`
}

type StudentLoginRequest struct {
	SchoolSlug      string `json:"school_slug"`
	AdmissionNumber string `json:"admission_number,omitempty"`
	Email           string `json:"email,omitempty"`
	GoogleUID       string `json:"google_uid,omitempty"`
}

type AdminLoginRequest struct {
	SchoolSlug string `json:"school_slug"`
	Email      string `json:"email"`
	Password   string `json:"password,omitempty"`
	GoogleUID  string `json:"google_uid,omitempty"`
}

type AuthResponse struct {
	Success bool                   `json:"success"`
	Role    string                 `json:"role"` // "student" | "admin"
	Token   string                 `json:"token"`
	School  School                 `json:"school"`
	User    map[string]interface{} `json:"user"`
	Message string                 `json:"message,omitempty"`
}
