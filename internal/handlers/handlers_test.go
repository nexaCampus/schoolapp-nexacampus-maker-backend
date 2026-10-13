package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/database"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/deployer"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/models"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/provisioner"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/security"
)

func TestOrchestratorAndTenantHandlers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "handlers-test")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Setup Master Database & Orchestrator
	masterDBPath := filepath.Join(tempDir, "master.db")
	masterDB, err := database.OpenSQLite(masterDBPath)
	if err != nil {
		t.Fatalf("opening master db: %v", err)
	}
	defer masterDB.Close()

	if err := database.MigrateMaster(masterDB); err != nil {
		t.Fatalf("migrating master: %v", err)
	}

	quotaMgr := provisioner.NewQuotaManager(masterDB)
	deployers := map[deployer.DeploymentTarget]deployer.Deployer{
		deployer.TargetRender: deployer.NewMockDeployer(),
		deployer.TargetLocal:  deployer.NewMockDeployer(),
	}
	dnsBinder := provisioner.NewDNSBinder("nexacampus.com", "", "", "")

	orch := provisioner.NewTenantOrchestrator(
		masterDB,
		quotaMgr,
		deployers,
		dnsBinder,
		"nexacampus.com",
		"ghcr.io/nexacampus/school-tenant:latest",
	)

	orchHandler := NewOrchestratorHandler(orch, quotaMgr, masterDB)

	// Test 1: Orchestrator /healthz
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	orchHandler.Healthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected healthz 200, got %d", rec.Code)
	}

	// Test 2: Slug Validation
	slugBody, _ := json.Marshal(models.ValidateSlugRequest{Slug: "lcgvm"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tenants/validate-slug", bytes.NewReader(slugBody))
	rec = httptest.NewRecorder()
	orchHandler.ValidateSlug(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected validate-slug 200, got %d", rec.Code)
	}

	// Test 3: Register Tenant with Free Tier
	regPayload := models.CreateTenantRequest{
		Slug:          "lcgvm",
		Name:          "Lions Calcutta Greater Vidya Mandir",
		Plan:          "free",
		Provider:      "render",
		AdminUsername: "admin",
		AdminPassword: "SuperSecretAdminPassword123!",
	}
	regBody, _ := json.Marshal(regPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tenants/register", bytes.NewReader(regBody))
	rec = httptest.NewRecorder()
	orchHandler.RegisterTenant(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected tenant registration 201, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var regResp models.CreateTenantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if !regResp.Success || regResp.Tenant.Slug != "lcgvm" {
		t.Errorf("unexpected tenant response: %+v", regResp)
	}
	if regResp.SubdomainURL != "https://api-lcgvm.nexacampus.com" {
		t.Errorf("unexpected subdomain: got %s, want https://api-lcgvm.nexacampus.com", regResp.SubdomainURL)
	}

	// Test 4: Second School attempting "free" plan -> Must return 409 Conflict
	secondReg := models.CreateTenantRequest{
		Slug:          "dps-kolkata",
		Name:          "DPS Kolkata",
		Plan:          "free",
		Provider:      "render",
		AdminUsername: "admin",
		AdminPassword: "SuperSecretAdminPassword123!",
	}
	secondBody, _ := json.Marshal(secondReg)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tenants/register", bytes.NewReader(secondBody))
	rec = httptest.NewRecorder()
	orchHandler.RegisterTenant(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for second free tier registration, got %d", rec.Code)
	}

	// 2. Setup Tenant Database & Tenant Handler
	tenantDBPath := filepath.Join(tempDir, "lcgvm.db")
	tenantDB, err := database.OpenSQLite(tenantDBPath)
	if err != nil {
		t.Fatalf("opening tenant db: %v", err)
	}
	defer tenantDB.Close()

	if err := database.MigrateTenant(tenantDB); err != nil {
		t.Fatalf("migrating tenant: %v", err)
	}

	jwtSecret := []byte("a-very-secure-32-byte-secret-key-for-tenant-tests!")
	passHash, _ := security.HashPassword("AdminPassword123!")
	_ = database.SeedTenantAdmin(tenantDB, "adm_1", "admin", passHash)
	_ = database.SeedSchoolMeta(tenantDB, "Lions Calcutta Greater Vidya Mandir", "#3b49df", "#12131a")

	tenantHandler := NewTenantHandler(tenantDB, "lcgvm", jwtSecret)

	// Test 5: Tenant Public /api/v1/meta
	req = httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
	rec = httptest.NewRecorder()
	tenantHandler.GetMeta(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected meta 200, got %d", rec.Code)
	}

	// Test 6: Tenant Public Admission Enquiry Submission
	admPayload := models.Admission{
		StudentName:   "Rahul Sharma",
		ParentName:    "Vikram Sharma",
		Email:         "vikram.sharma@example.com",
		Phone:         "+91 9876543210",
		GradeApplying: "Grade 11 Science",
	}
	admBody, _ := json.Marshal(admPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admissions", bytes.NewReader(admBody))
	rec = httptest.NewRecorder()
	tenantHandler.SubmitAdmission(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected admissions submission 201, got %d", rec.Code)
	}

	// Test 7: Tenant Admin Login
	loginPayload := models.LoginRequest{
		Username: "admin",
		Password: "AdminPassword123!",
	}
	loginBody, _ := json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", bytes.NewReader(loginBody))
	rec = httptest.NewRecorder()
	tenantHandler.AdminLogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected admin login 200, got %d", rec.Code)
	}

	var loginResp models.LoginResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &loginResp)
	if loginResp.Token == "" {
		t.Errorf("expected non-empty JWT token")
	}

	// Test 8: Admin Create Notice
	noticePayload := models.Notice{
		Title:    "Annual Sports Day 2026",
		Content:  "The annual sports championship will commence next Monday.",
		Category: "Sports",
		IsPinned: true,
	}
	noticeBody, _ := json.Marshal(noticePayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/notices", bytes.NewReader(noticeBody))
	rec = httptest.NewRecorder()
	tenantHandler.AdminCreateNotice(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected notice creation 201, got %d", rec.Code)
	}

	// Test 9: Public Notices query verification
	req = httptest.NewRequest(http.MethodGet, "/api/v1/notices", nil)
	rec = httptest.NewRecorder()
	tenantHandler.GetNotices(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected notices list 200, got %d", rec.Code)
	}
}
