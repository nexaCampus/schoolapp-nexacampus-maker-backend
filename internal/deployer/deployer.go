package deployer

import (
	"context"
	"time"
)

type DeploymentTarget string

const (
	TargetRender DeploymentTarget = "render"
	TargetVercel DeploymentTarget = "vercel"
	TargetAWS    DeploymentTarget = "aws"
	TargetLocal  DeploymentTarget = "local"
)

type DeploymentStatus string

const (
	StatusPending   DeploymentStatus = "pending"
	StatusBuilding  DeploymentStatus = "building"
	StatusLive      DeploymentStatus = "live"
	StatusFailed    DeploymentStatus = "failed"
	StatusSuspended DeploymentStatus = "suspended"
)

// TenantDeploymentConfig contains all parameters needed to dispatch a tenant container.
type TenantDeploymentConfig struct {
	TenantSlug        string            `json:"tenant_slug"`
	SchoolName        string            `json:"school_name"`
	Plan              string            `json:"plan"` // "free", "starter", "byo"
	Provider          DeploymentTarget  `json:"provider"`
	DockerImage       string            `json:"docker_image"`
	AdminPasswordHash string            `json:"admin_password_hash"`
	AdminUsername     string            `json:"admin_username"`
	AdminEmail        string            `json:"admin_email"`
	JWTSecret         string            `json:"jwt_secret"`
	BYOApiKey         string            `json:"byo_api_key,omitempty"`
	SubdomainHost     string            `json:"subdomain_host"`
	Theme             map[string]string `json:"theme,omitempty"`
}

// DeploymentResult captures the provisioned cloud compute endpoints.
type DeploymentResult struct {
	DeploymentID string           `json:"deployment_id"`
	ServiceID    string           `json:"service_id"`
	Provider     DeploymentTarget `json:"provider"`
	Status       DeploymentStatus `json:"status"`
	DefaultURL   string           `json:"default_url"`
	Logs         string           `json:"logs"`
	CreatedAt    time.Time        `json:"created_at"`
}

// Deployer defines the contract for dispatching isolated school backend instances.
type Deployer interface {
	DeployTenant(ctx context.Context, config TenantDeploymentConfig) (*DeploymentResult, error)
	GetDeploymentStatus(ctx context.Context, deploymentID string) (DeploymentStatus, error)
}
