package deployer

import (
	"context"
	"fmt"
	"time"
)

// MockDeployer provides instant, zero-network deployments for automated tests and offline development.
type MockDeployer struct{}

func NewMockDeployer() *MockDeployer {
	return &MockDeployer{}
}

func (m *MockDeployer) DeployTenant(ctx context.Context, config TenantDeploymentConfig) (*DeploymentResult, error) {
	return &DeploymentResult{
		DeploymentID: fmt.Sprintf("mock_dep_%s", config.TenantSlug),
		ServiceID:    fmt.Sprintf("mock_srv_%s", config.TenantSlug),
		Provider:     TargetLocal,
		Status:       StatusLive,
		DefaultURL:   fmt.Sprintf("http://localhost:8081"),
		Logs:         fmt.Sprintf("Mock tenant runtime initialized for %s", config.TenantSlug),
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func (m *MockDeployer) GetDeploymentStatus(ctx context.Context, deploymentID string) (DeploymentStatus, error) {
	return StatusLive, nil
}
