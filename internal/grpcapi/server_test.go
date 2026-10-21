package grpcapi

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/database"
)

func newTestServer(t *testing.T) (SchoolServiceClient, HealthServiceClient, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	if err := database.MigrateMaster(db); err != nil {
		t.Fatalf("migrating test database: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	server := grpc.NewServer()
	impl := NewServer(db, "nexacampus.tech", "school-webapp-self.vercel.app")
	RegisterSchoolServiceServer(server, impl)
	RegisterHealthServiceServer(server, impl)
	go func() { _ = server.Serve(listener) }()

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dialing: %v", err)
	}

	return NewSchoolServiceClient(conn), NewHealthServiceClient(conn), func() {
		_ = conn.Close()
		server.Stop()
		_ = db.Close()
	}
}

func TestListSchoolsReturnsCanonicalDomains(t *testing.T) {
	schools, health, cleanup := newTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	list, err := schools.ListSchools(ctx, &ListSchoolsRequest{})
	if err != nil {
		t.Fatalf("ListSchools: %v", err)
	}
	if list.GetTotal() < 2 {
		t.Fatalf("expected the seeded tenants, got %d", list.GetTotal())
	}

	for _, school := range list.GetSchools() {
		if school.GetSubdomainSlug() == "" {
			continue
		}
		want := "https://" + school.GetSubdomainSlug() + ".school-webapp-self.vercel.app"
		if school.GetAppDomain() != want {
			t.Errorf("%s: app domain = %q, want %q", school.GetSubdomainSlug(), school.GetAppDomain(), want)
		}
		if school.GetAppAdminDomain() == "" {
			t.Errorf("%s: missing admin app domain", school.GetSubdomainSlug())
		}
	}

	healthResp, err := health.Health(ctx, &HealthRequest{})
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !healthResp.GetHealthy() {
		t.Errorf("expected a healthy service, got %q", healthResp.GetStatus())
	}
	if healthResp.GetAppBaseDomain() != "school-webapp-self.vercel.app" {
		t.Errorf("unexpected app base domain: %q", healthResp.GetAppBaseDomain())
	}
}

func TestGetSchoolNotFound(t *testing.T) {
	schools, _, cleanup := newTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := schools.GetSchool(ctx, &GetSchoolRequest{SubdomainSlug: "does-not-exist"})
	if err != nil {
		t.Fatalf("GetSchool: %v", err)
	}
	if resp.GetFound() {
		t.Errorf("expected the tenant to be missing")
	}
}

func TestGetSchoolByCreator(t *testing.T) {
	schools, _, cleanup := newTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// The seeded Greenwood tenant belongs to +91 9830123456.
	resp, err := schools.GetSchoolByCreator(ctx, &GetSchoolByCreatorRequest{CreatorPhone: "+91 9830123456"})
	if err != nil {
		t.Fatalf("GetSchoolByCreator: %v", err)
	}
	if !resp.GetFound() {
		t.Fatalf("expected to find the Greenwood tenant")
	}
	if resp.GetSchool().GetSubdomainSlug() != "greenwood" {
		t.Errorf("unexpected slug: %q", resp.GetSchool().GetSubdomainSlug())
	}
	_ = metadata.MD{}
}
