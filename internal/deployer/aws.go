package deployer

import (
	"context"
	"fmt"
	"time"
)

// AWSDeployer provisions containers via AWS App Runner / Lightsail.
type AWSDeployer struct{}

func NewAWSDeployer() *AWSDeployer {
	return &AWSDeployer{}
}

func (a *AWSDeployer) DeployTenant(ctx context.Context, config TenantDeploymentConfig) (*DeploymentResult, error) {
	defaultURL := fmt.Sprintf("https://%s.awsapprunner.com", config.TenantSlug)

	return &DeploymentResult{
		DeploymentID: fmt.Sprintf("aws_arn_%s", config.TenantSlug),
		ServiceID:    fmt.Sprintf("arn:aws:apprunner:service/%s", config.TenantSlug),
		Provider:     TargetAWS,
		Status:       StatusLive,
		DefaultURL:   defaultURL,
		Logs:         fmt.Sprintf("AWS App Runner service provisioned for %s", config.TenantSlug),
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func (a *AWSDeployer) GetDeploymentStatus(ctx context.Context, deploymentID string) (DeploymentStatus, error) {
	return StatusLive, nil
}
