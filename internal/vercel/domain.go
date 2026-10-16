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
	token      string
	projectID  string
	teamID     string
	baseDomain string
	httpClient *http.Client
}

// ProvisionedDomains holds the resulting subdomains and path fallback routes.
type ProvisionedDomains struct {
	StudentCustomDomain string   `json:"student_custom_domain"`
	AdminCustomDomain   string   `json:"admin_custom_domain"`
	VercelAppDomain     string   `json:"vercel_app_domain"`
	VercelAdminDomain   string   `json:"vercel_admin_domain"`
	PathFallbackStudent string   `json:"path_fallback_student"`
	PathFallbackAdmin   string   `json:"path_fallback_admin"`
	AttachedDomains     []string `json:"attached_domains"`
	Errors              []string `json:"errors,omitempty"`
}

// NewDomainService creates an instance of the Vercel domain provisioning service.
func NewDomainService(token, projectID, teamID, baseDomain string) *DomainService {
	if baseDomain == "" {
		baseDomain = "nexacampus.tech"
	}
	return &DomainService{
		token:      strings.TrimSpace(token),
		projectID:  strings.TrimSpace(projectID),
		teamID:     strings.TrimSpace(teamID),
		baseDomain: strings.TrimSpace(baseDomain),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ProvisionSubdomains provisions all four custom and fallback domains for a school tenant on Vercel.
func (s *DomainService) ProvisionSubdomains(ctx context.Context, slug string) (*ProvisionedDomains, error) {
	cleanSlug := strings.ToLower(strings.TrimSpace(slug))

	studentCustom := fmt.Sprintf("%s.%s", cleanSlug, s.baseDomain)
	adminCustom := fmt.Sprintf("%s.admin.%s", cleanSlug, s.baseDomain)
	vercelStudent := fmt.Sprintf("%s-nexacampus.vercel.app", cleanSlug)
	vercelAdmin := fmt.Sprintf("%s-admin-nexacampus.vercel.app", cleanSlug)

	res := &ProvisionedDomains{
		StudentCustomDomain: fmt.Sprintf("https://%s", studentCustom),
		AdminCustomDomain:   fmt.Sprintf("https://%s", adminCustom),
		VercelAppDomain:     fmt.Sprintf("https://%s", vercelStudent),
		VercelAdminDomain:   fmt.Sprintf("https://%s", vercelAdmin),
		PathFallbackStudent: fmt.Sprintf("https://school-webapp-self.vercel.app/school/%s", cleanSlug),
		PathFallbackAdmin:   fmt.Sprintf("https://school-webapp-self.vercel.app/school/%s/admin", cleanSlug),
		AttachedDomains:     []string{},
		Errors:              []string{},
	}

	// Graceful handling if VERCEL_API_TOKEN or VERCEL_PROJECT_ID is omitted
	if s.token == "" || s.projectID == "" {
		log.Printf("[Vercel Domain Provisioner] VERCEL_API_TOKEN or VERCEL_PROJECT_ID is not configured. Returning preview & path fallback URLs for '%s'", cleanSlug)
		res.AttachedDomains = append(res.AttachedDomains, studentCustom, adminCustom, vercelStudent, vercelAdmin)
		return res, nil
	}

	// Register each domain via Vercel REST API
	targetDomains := []string{
		studentCustom,
		adminCustom,
		vercelStudent,
		vercelAdmin,
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
