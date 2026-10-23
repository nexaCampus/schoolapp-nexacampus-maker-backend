package vercel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// DomainService manages Vercel REST API project domain registration.
type DomainService struct {
	token         string
	projectID     string
	teamID        string
	baseDomain    string
	appBaseDomain string
	httpClient    *http.Client
}

// ProvisionedDomains holds the resulting subdomains and path fallback routes.
type ProvisionedDomains struct {
	// StudentCustomDomain is the DNS-bound production domain: https://<slug>.nexacampus.tech
	StudentCustomDomain string `json:"student_custom_domain"`
	// AdminCustomDomain is the DNS-bound admin domain: https://<slug>.admin.nexacampus.tech
	AdminCustomDomain string `json:"admin_custom_domain"`
	// AppDomain is the canonical student portal: https://<slug>.<APP_BASE_DOMAIN>
	AppDomain string `json:"app_domain"`
	// AppAdminDomain is the canonical admin portal: https://<slug>.admin.<APP_BASE_DOMAIN>
	AppAdminDomain string `json:"app_admin_domain"`
	// VercelAppDomain is kept for backwards compatibility and mirrors AppDomain.
	VercelAppDomain string `json:"vercel_app_domain"`
	// VercelAdminDomain is kept for backwards compatibility and mirrors AppAdminDomain.
	VercelAdminDomain   string   `json:"vercel_admin_domain"`
	PathFallbackStudent string   `json:"path_fallback_student"`
	PathFallbackAdmin   string   `json:"path_fallback_admin"`
	AttachedDomains     []string `json:"attached_domains"`
	Errors              []string `json:"errors,omitempty"`
}

// NewDomainService creates an instance of the Vercel domain provisioning service.
// appBaseDomain is the host serving the multi-tenant webapp; every tenant lives on
// <slug>.<appBaseDomain>. An empty value falls back to school-webapp-self.vercel.app.
func NewDomainService(token, projectID, teamID, baseDomain, appBaseDomain string) *DomainService {
	if baseDomain == "" {
		baseDomain = "nexacampus.school"
	}
	if appBaseDomain == "" {
		appBaseDomain = "nexacampus.school"
	}
	return &DomainService{
		token:         strings.TrimSpace(token),
		projectID:     strings.TrimSpace(projectID),
		teamID:        strings.TrimSpace(teamID),
		baseDomain:    strings.TrimSpace(baseDomain),
		appBaseDomain: strings.TrimSpace(appBaseDomain),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ProvisionSubdomains provisions all custom and fallback domains for a school tenant on Vercel.
func (s *DomainService) ProvisionSubdomains(ctx context.Context, slug string) (*ProvisionedDomains, error) {
	cleanSlug := strings.ToLower(strings.TrimSpace(slug))

	studentCustom := fmt.Sprintf("%s.%s", cleanSlug, s.baseDomain)
	adminCustom := fmt.Sprintf("%s.admin.%s", cleanSlug, s.baseDomain)
	// Canonical app-host subdomains — this is how a tenant is addressed.
	appStudent := fmt.Sprintf("%s.%s", cleanSlug, s.appBaseDomain)
	appAdmin := fmt.Sprintf("%s.admin.%s", cleanSlug, s.appBaseDomain)

	res := &ProvisionedDomains{
		StudentCustomDomain: fmt.Sprintf("https://%s", studentCustom),
		AdminCustomDomain:   fmt.Sprintf("https://%s", adminCustom),
		AppDomain:           fmt.Sprintf("https://%s", appStudent),
		AppAdminDomain:      fmt.Sprintf("https://%s", appAdmin),
		VercelAppDomain:     fmt.Sprintf("https://%s", appStudent),
		VercelAdminDomain:   fmt.Sprintf("https://%s", appAdmin),
		PathFallbackStudent: fmt.Sprintf("https://%s/school/%s", s.appBaseDomain, cleanSlug),
		PathFallbackAdmin:   fmt.Sprintf("https://%s/school/%s/admin", s.appBaseDomain, cleanSlug),
		AttachedDomains:     []string{},
		Errors:              []string{},
	}

	// Graceful handling if VERCEL_API_TOKEN or VERCEL_PROJECT_ID is omitted
	if s.token == "" || s.projectID == "" {
		log.Printf("[Vercel Domain Provisioner] VERCEL_API_TOKEN or VERCEL_PROJECT_ID is not configured. Returning preview & path fallback URLs for '%s'", cleanSlug)
		res.AttachedDomains = append(res.AttachedDomains, studentCustom, adminCustom, appStudent, appAdmin)
		return res, nil
	}

	// Register each domain via Vercel REST API
	targetDomains := []string{
		studentCustom,
		adminCustom,
		appStudent,
		appAdmin,
	}

	for _, domain := range targetDomains {
		err := s.attachVercelDomain(ctx, domain)
		if err != nil {
			log.Printf("[Vercel Domain Provisioner] Notice: could not attach domain '%s' on Vercel: %v", domain, err)
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", domain, err))
		} else {
			res.AttachedDomains = append(res.AttachedDomains, domain)
			log.Printf("[Vercel Domain Provisioner] Successfully registered domain '%s' on Vercel project %s", domain, s.projectID)
		}
	}

	return res, nil
}

func (s *DomainService) attachVercelDomain(ctx context.Context, domainName string) error {
	endpoint := fmt.Sprintf("https://api.vercel.com/v10/projects/%s/domains", s.projectID)
	if s.teamID != "" {
		endpoint += fmt.Sprintf("?teamId=%s", s.teamID)
	}

	bodyMap := map[string]interface{}{
		"name": domainName,
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// If domain already attached or duplicate, don't fail the entire provisioning flow
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusConflict || strings.Contains(string(respBody), "already exists") || strings.Contains(string(respBody), "domain_already_in_use") {
		return nil
	}

	return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
}
