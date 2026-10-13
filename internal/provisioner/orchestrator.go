package provisioner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/deployer"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/models"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/security"
)

var (
	ErrTenantAlreadyExists = errors.New("a school with this identifier slug has already been claimed")
	ErrUnsupportedProvider = errors.New("the requested cloud deployment target is unsupported")
)

// TenantOrchestrator coordinates registration, cloud deployment, and DNS wiring.
type TenantOrchestrator struct {
	db           *sql.DB
	quotaManager *QuotaManager
	deployers    map[deployer.DeploymentTarget]deployer.Deployer
	dnsBinder    DNSBinder
	baseDomain   string
	tenantImage  string
}

// NewTenantOrchestrator constructs the School Maker registration and provisioning pipeline.
func NewTenantOrchestrator(
	db *sql.DB,
	quotaManager *QuotaManager,
	deployers map[deployer.DeploymentTarget]deployer.Deployer,
	dnsBinder DNSBinder,
	baseDomain string,
	tenantImage string,
) *TenantOrchestrator {
	return &TenantOrchestrator{
		db:           db,
		quotaManager: quotaManager,
		deployers:    deployers,
		dnsBinder:    dnsBinder,
		baseDomain:   baseDomain,
		tenantImage:  tenantImage,
	}
}

// ValidateSlug checks format, blocklist, and database availability.
func (o *TenantOrchestrator) ValidateSlug(ctx context.Context, slug string) (*models.ValidateSlugResponse, error) {
	cleanSlug := strings.TrimSpace(strings.ToLower(slug))
	if err := security.ValidateSlug(cleanSlug); err != nil {
		return &models.ValidateSlugResponse{
			Valid:     false,
			Available: false,
			Reason:    err.Error(),
		}, nil
	}

	var count int
	query := `SELECT COUNT(*) FROM tenants WHERE slug = ?`
	if err := o.db.QueryRowContext(ctx, query, cleanSlug).Scan(&count); err != nil {
		return nil, fmt.Errorf("checking slug existence: %w", err)
	}

	if count > 0 {
		return &models.ValidateSlugResponse{
			Valid:     true,
			Available: false,
			Reason:    "slug is already claimed by another institution",
		}, nil
	}

	return &models.ValidateSlugResponse{
		Valid:     true,
		Available: true,
	}, nil
}

// RegisterAndProvisionTenant executes the full onboarding pipeline.
func (o *TenantOrchestrator) RegisterAndProvisionTenant(ctx context.Context, req models.CreateTenantRequest) (*models.CreateTenantResponse, error) {
	slug := strings.TrimSpace(strings.ToLower(req.Slug))
	if err := security.ValidateSlug(slug); err != nil {
		return nil, err
	}

	// 1. Ensure slug uniqueness
	valRes, err := o.ValidateSlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !valRes.Available {
		return nil, ErrTenantAlreadyExists
	}

	// 2. Enforce Free-Tier Single-Use Quota if plan is "free"
	plan := strings.ToLower(strings.TrimSpace(req.Plan))
	if plan == "" {
		plan = "starter"
	}

	if plan == "free" {
		if err := o.quotaManager.ConsumeFreeTierAtomic(ctx, slug); err != nil {
			return nil, err
		}
	}

	// 3. Resolve deployment target
	providerTarget := deployer.DeploymentTarget(strings.ToLower(strings.TrimSpace(req.Provider)))
	if providerTarget == "" {
		providerTarget = deployer.TargetRender
	}

	dep, exists := o.deployers[providerTarget]
	if !exists {
		// Fallback to local or render
		dep = o.deployers[deployer.TargetLocal]
		if dep == nil {
			return nil, ErrUnsupportedProvider
		}
	}

	// 4. Generate dynamic, cryptographically secure credentials
	jwtSecret, err := security.GenerateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generating jwt secret: %w", err)
	}

	adminUser := strings.TrimSpace(req.AdminUsername)
	if adminUser == "" {
		adminUser = "admin"
	}

	rawPassword := req.AdminPassword
	if rawPassword == "" {
		rawPassword = security.MustGenerateSecureToken(10) // 20 chars
	}

	passwordHash, err := security.HashPassword(rawPassword)
	if err != nil {
		return nil, fmt.Errorf("hashing admin password: %w", err)
	}

	// 5. Cloud deployment dispatch
	subdomainHost := fmt.Sprintf("api-%s.%s", slug, o.baseDomain)
	deployConfig := deployer.TenantDeploymentConfig{
		TenantSlug:        slug,
		SchoolName:        req.Name,
		Plan:              plan,
		Provider:          providerTarget,
		DockerImage:       o.tenantImage,
		AdminPasswordHash: passwordHash,
		AdminUsername:     adminUser,
		AdminEmail:        req.AdminEmail,
		JWTSecret:         jwtSecret,
		BYOApiKey:         req.BYOApiKey,
		SubdomainHost:     subdomainHost,
		Theme:             req.Theme,
	}

	deployResult, err := dep.DeployTenant(ctx, deployConfig)
	if err != nil {
		return nil, fmt.Errorf("cloud deployment error: %w", err)
	}

	// 6. Subdomain-to-Backend DNS Binding
	dnsResult, err := o.dnsBinder.BindSubdomain(ctx, slug, deployResult.ServiceID, deployResult.DefaultURL)
	if err != nil {
		// DNS error logged but does not abort the provisioned service
		dnsResult = &DNSBindingResult{
			CustomDomain: fmt.Sprintf("api-%s.%s", slug, o.baseDomain),
		}
	}

	// 7. Store tenant record in master database
	tenantID := uuid.New().String()
	now := time.Now().UTC()

	insertQuery := `
		INSERT INTO tenants (
			id, slug, name, plan, provider, status, service_id, backend_url, custom_domain, admin_username, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = o.db.ExecContext(
		ctx,
		insertQuery,
		tenantID,
		slug,
		req.Name,
		plan,
		string(providerTarget),
		string(deployResult.Status),
		deployResult.ServiceID,
		deployResult.DefaultURL,
		dnsResult.CustomDomain,
		adminUser,
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("saving tenant to master catalog: %w", err)
	}

	// Record deployment log
	_, _ = o.db.ExecContext(
		ctx,
		`INSERT INTO deployments (id, tenant_slug, provider, status, service_id, logs, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		deployResult.DeploymentID,
		slug,
		string(providerTarget),
		string(deployResult.Status),
		deployResult.ServiceID,
		deployResult.Logs,
		now,
	)

	tenantRecord := models.Tenant{
		ID:            tenantID,
		Slug:          slug,
		Name:          req.Name,
		Plan:          plan,
		Provider:      string(providerTarget),
		Status:        string(deployResult.Status),
		ServiceID:     deployResult.ServiceID,
		BackendURL:    deployResult.DefaultURL,
		CustomDomain:  dnsResult.CustomDomain,
		AdminUsername: adminUser,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	return &models.CreateTenantResponse{
		Success:          true,
		Tenant:           tenantRecord,
		DirectBackendURL: deployResult.DefaultURL,
		SubdomainURL:     fmt.Sprintf("https://%s", dnsResult.CustomDomain),
		InitialAdminUser: adminUser,
		InitialAdminPass: rawPassword,
		DeploymentID:     deployResult.DeploymentID,
		Message:          fmt.Sprintf("School backend '%s' provisioned and bound to https://%s", slug, dnsResult.CustomDomain),
		CreatedAt:        now,
	}, nil
}

// ListTenants retrieves all active and provisioned schools.
func (o *TenantOrchestrator) ListTenants(ctx context.Context) ([]models.Tenant, error) {
	rows, err := o.db.QueryContext(ctx, `SELECT id, slug, name, plan, provider, status, COALESCE(service_id, ''), COALESCE(backend_url, ''), COALESCE(custom_domain, ''), admin_username, created_at, updated_at FROM tenants ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []models.Tenant
	for rows.Next() {
		var t models.Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Plan, &t.Provider, &t.Status, &t.ServiceID, &t.BackendURL, &t.CustomDomain, &t.AdminUsername, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}

	return tenants, nil
}

// GetTenantBySlug fetches a single school record.
func (o *TenantOrchestrator) GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	var t models.Tenant
	query := `SELECT id, slug, name, plan, provider, status, COALESCE(service_id, ''), COALESCE(backend_url, ''), COALESCE(custom_domain, ''), admin_username, created_at, updated_at FROM tenants WHERE slug = ?`
	err := o.db.QueryRowContext(ctx, query, slug).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Plan, &t.Provider, &t.Status, &t.ServiceID, &t.BackendURL, &t.CustomDomain, &t.AdminUsername, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}
