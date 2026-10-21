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
)

func newTestOrchestrator(t *testing.T) (*OrchestratorHandler, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "orchestrator-test")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}

	masterDB, err := database.OpenSQLite(filepath.Join(tempDir, "master.db"))
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("opening master db: %v", err)
	}
	if err := database.MigrateMaster(masterDB); err != nil {
		masterDB.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("migrating master: %v", err)
	}

	quotaMgr := provisioner.NewQuotaManager(masterDB)
	deployers := map[deployer.DeploymentTarget]deployer.Deployer{
		deployer.TargetRender: deployer.NewMockDeployer(),
		deployer.TargetLocal:  deployer.NewMockDeployer(),
	}
	dnsBinder := provisioner.NewDNSBinder("nexacampus.tech", "", "", "")
	orch := provisioner.NewTenantOrchestrator(
		masterDB,
		quotaMgr,
		deployers,
		dnsBinder,
		"nexacampus.tech",
		"ghcr.io/nexacampus/school-tenant:latest",
	)

	h := NewOrchestratorHandler(orch, quotaMgr, masterDB)
	h.SetBaseDomain("nexacampus.tech")
	h.SetEnvironment("production")

	cleanup := func() {
		masterDB.Close()
		os.RemoveAll(tempDir)
	}
	return h, cleanup
}

func provisionRequest(phone, slug string) *http.Request {
	payload := models.CreateSchoolRequest{
		CreatorPhone:       phone,
		CreatorName:        "Test Principal",
		LeadershipPosition: "Principal",
		SchoolName:         "Test International School",
		TeacherCount:       "40",
		AffiliationBoard:   "CBSE",
		EnrolledScholars:   "900",
		City:               "Kolkata",
		StateTerritory:     "West Bengal",
		SubdomainSlug:      slug,
		SchoolMotto:        "Knowledge is Power",
		ActiveModules:      map[string]bool{"canteen": true, "bus": true},
	}
	body, _ := json.Marshal(payload)
	return httptest.NewRequest(http.MethodPost, "/api/v1/maker/provision", bytes.NewReader(body))
}

// A school slug already owned by a different creator must not be overwritable.
func TestProvisionSchoolRejectsForeignCreator(t *testing.T) {
	h, cleanup := newTestOrchestrator(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	h.MakerProvisionSchool(rec, provisionRequest("+91 9000000001", "riverton"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected first provisioning 201, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.MakerProvisionSchool(rec, provisionRequest("+91 9000000002", "riverton"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when a foreign creator reuses a slug, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// The same creator may re-provision (update) their own school.
func TestProvisionSchoolAllowsOwnerToUpdate(t *testing.T) {
	h, cleanup := newTestOrchestrator(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	h.MakerProvisionSchool(rec, provisionRequest("+91 9000000001", "riverton"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.MakerProvisionSchool(rec, provisionRequest("+91 9000000001", "riverton"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for owner re-provision, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// The universal "123456" OTP must no longer be accepted in production.
func TestVerifyOTPRemovesUniversalFallback(t *testing.T) {
	h, cleanup := newTestOrchestrator(t)
	defer cleanup()

	sendBody, _ := json.Marshal(models.SendOTPRequest{Phone: "+91 9000000003", CreatorName: "Test"})
	rec := httptest.NewRecorder()
	h.MakerSendOTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/maker/auth/send-otp", bytes.NewReader(sendBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected send-otp 200, got %d", rec.Code)
	}

	var sendResp models.SendOTPResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sendResp)
	if sendResp.DevOTP != "" {
		t.Errorf("production must not echo the verification code, got %q", sendResp.DevOTP)
	}

	verifyBody, _ := json.Marshal(models.VerifyOTPRequest{Phone: "+91 9000000003", OTP: "123456", CreatorName: "Test"})
	rec = httptest.NewRecorder()
	h.MakerVerifyOTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/maker/auth/verify-otp", bytes.NewReader(verifyBody)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for the universal fallback code, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// Outside production the code is echoed so the local flow stays usable.
func TestSendOTPExposesDevCodeOutsideProduction(t *testing.T) {
	h, cleanup := newTestOrchestrator(t)
	defer cleanup()
	h.SetEnvironment("development")

	sendBody, _ := json.Marshal(models.SendOTPRequest{Phone: "+91 9000000004", CreatorName: "Test"})
	rec := httptest.NewRecorder()
	h.MakerSendOTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/maker/auth/send-otp", bytes.NewReader(sendBody)))

	var sendResp models.SendOTPResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sendResp)
	if sendResp.DevOTP == "" {
		t.Errorf("expected a development verification code to be returned")
	}

	verifyBody, _ := json.Marshal(models.VerifyOTPRequest{Phone: "+91 9000000004", OTP: sendResp.DevOTP, CreatorName: "Test"})
	rec = httptest.NewRecorder()
	h.MakerVerifyOTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/maker/auth/verify-otp", bytes.NewReader(verifyBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected verification 200 with the issued code, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// Empty filters must not enumerate the whole schools table.
func TestListEndpointsRejectEmptyFilters(t *testing.T) {
	h, cleanup := newTestOrchestrator(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	h.SearchSchools(rec, httptest.NewRequest(http.MethodGet, "/api/v1/schools/search?q=", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for an empty search query, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.GetMySchools(rec, httptest.NewRequest(http.MethodGet, "/api/v1/maker/my-schools?phone=", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for an empty creator phone, got %d", rec.Code)
	}
}

// A partial patch must persist the supplied fields, not only the module map.
func TestUpdateSchoolPersistsSuppliedFields(t *testing.T) {
	h, cleanup := newTestOrchestrator(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	h.MakerProvisionSchool(rec, provisionRequest("+91 9000000005", "northgate"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	patchBody, _ := json.Marshal(models.UpdateSchoolRequest{
		SchoolName: strPtr("Northgate Renamed School"),
		City:       strPtr("Howrah"),
	})
	rec = httptest.NewRecorder()
	h.UpdateSchool(rec, httptest.NewRequest(http.MethodPatch, "/api/v1/schools/northgate", bytes.NewReader(patchBody)), "northgate")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected patch 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.GetSchool(rec, httptest.NewRequest(http.MethodGet, "/api/v1/schools/northgate", nil), "northgate")
	var school models.School
	_ = json.Unmarshal(rec.Body.Bytes(), &school)
	if school.SchoolName != "Northgate Renamed School" {
		t.Errorf("expected patched school name, got %q", school.SchoolName)
	}
	if school.City != "Howrah" {
		t.Errorf("expected patched city, got %q", school.City)
	}
}

func strPtr(s string) *string { return &s }
