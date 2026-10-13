package deployer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/security"
)

// RenderDeployer provisions isolated school tenant services on Render via its REST API.
type RenderDeployer struct {
	defaultAPIKey string
	httpClient    *http.Client
}

// NewRenderDeployer constructs a Render cloud deployer.
func NewRenderDeployer(defaultAPIKey string) *RenderDeployer {
	return &RenderDeployer{
		defaultAPIKey: defaultAPIKey,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// DeployTenant spins up a dedicated containerized web service on Render.
func (r *RenderDeployer) DeployTenant(ctx context.Context, config TenantDeploymentConfig) (*DeploymentResult, error) {
	apiKey := r.defaultAPIKey
	if config.BYOApiKey != "" {
		apiKey = config.BYOApiKey
	}

	serviceName := fmt.Sprintf("nexacampus-api-%s", config.TenantSlug)
	defaultHostname := fmt.Sprintf("https://%s.onrender.com", serviceName)

	// If no API key configured (e.g., local dev or offline), return simulated deployment
	if apiKey == "" {
		return &DeploymentResult{
			DeploymentID: fmt.Sprintf("rnd_mock_%s", config.TenantSlug),
			ServiceID:    fmt.Sprintf("srv_mock_%s", config.TenantSlug),
			Provider:     TargetRender,
			Status:       StatusLive,
			DefaultURL:   defaultHostname,
			Logs:         "Service spun up via local developer simulation driver",
			CreatedAt:    time.Now().UTC(),
		}, nil
	}

	rawURL := "https://api.render.com/v1/services"
	validatedURL, err := security.ValidateOutboundURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("validating render API url: %w", err)
	}

	planType := "starter"
	if config.Plan == "free" {
		planType = "free"
	}

	// Prepare payload for Render Web Service deployment
	payload := map[string]interface{}{
		"type": "web_service",
		"name": serviceName,
		"serviceDetails": map[string]interface{}{
			"env": "docker",
			"plan": planType,
			"region": "oregon",
			"image": map[string]string{
				"imagePath": config.DockerImage,
			},
			"envVars": []map[string]string{
				{"key": "PORT", "value": "8080"},
				{"key": "TENANT_SLUG", "value": config.TenantSlug},
				{"key": "SCHOOL_NAME", "value": config.SchoolName},
				{"key": "JWT_SECRET", "value": config.JWTSecret},
				{"key": "ADMIN_PASSWORD_HASH", "value": config.AdminPasswordHash},
				{"key": "ENV", "value": "production"},
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding render payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validatedURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating render request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dispatching render deployment: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("render deployment failed with HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var renderResponse struct {
		ID      string `json:"id"`
		Service struct {
			ID          string `json:"id"`
			ServiceURL  string `json:"serviceDetails.url"`
			Name        string `json:"name"`
		} `json:"service"`
	}

	_ = json.Unmarshal(respBody, &renderResponse)

	actualURL := defaultHostname
	serviceID := renderResponse.Service.ID
	if serviceID == "" {
		serviceID = renderResponse.ID
	}
	if serviceID == "" {
		serviceID = fmt.Sprintf("srv_%s", config.TenantSlug)
	}

	return &DeploymentResult{
		DeploymentID: fmt.Sprintf("dep_%s", serviceID),
		ServiceID:    serviceID,
		Provider:     TargetRender,
		Status:       StatusBuilding,
		DefaultURL:   actualURL,
		Logs:         fmt.Sprintf("Service created on Render: %s", serviceName),
		CreatedAt:    time.Now().UTC(),
	}, nil
}

// GetDeploymentStatus inspects the health of an active Render service.
func (r *RenderDeployer) GetDeploymentStatus(ctx context.Context, serviceID string) (DeploymentStatus, error) {
	if r.defaultAPIKey == "" {
		return StatusLive, nil
	}

	rawURL := fmt.Sprintf("https://api.render.com/v1/services/%s", serviceID)
	validatedURL, err := security.ValidateOutboundURL(rawURL)
	if err != nil {
		return StatusFailed, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL.String(), nil)
	if err != nil {
		return StatusFailed, err
	}
	req.Header.Set("Authorization", "Bearer "+r.defaultAPIKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return StatusFailed, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return StatusLive, nil
	}
	return StatusPending, nil
}
