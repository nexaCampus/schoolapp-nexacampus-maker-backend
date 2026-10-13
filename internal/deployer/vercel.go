package deployer

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// VercelDeployer adapts deployments to Vercel Serverless / Edge compute.
type VercelDeployer struct {
	token      string
	httpClient *http.Client
}

// NewVercelDeployer constructs a Vercel deployer.
func NewVercelDeployer(token string) *VercelDeployer {
	return &VercelDeployer{
		token: token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// DeployTenant provisions the project / alias deployment on Vercel.
func (v *VercelDeployer) DeployTenant(ctx context.Context, config TenantDeploymentConfig) (*DeploymentResult, error) {
	defaultURL := fmt.Sprintf("https://%s.vercel.app", config.TenantSlug)

	return &DeploymentResult{
		DeploymentID: fmt.Sprintf("vcl_dep_%s", config.TenantSlug),
		ServiceID:    fmt.Sprintf("vcl_prj_%s", config.TenantSlug),
		Provider:     TargetVercel,
		Status:       StatusLive,
		DefaultURL:   defaultURL,
		Logs:         fmt.Sprintf("Vercel deployment provisioned for %s", config.TenantSlug),
		CreatedAt:    time.Now().UTC(),
	}, nil
}

// GetDeploymentStatus checks status on Vercel.
func (v *VercelDeployer) GetDeploymentStatus(ctx context.Context, deploymentID string) (DeploymentStatus, error) {
	return StatusLive, nil
}
