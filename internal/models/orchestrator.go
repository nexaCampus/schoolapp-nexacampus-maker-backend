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
