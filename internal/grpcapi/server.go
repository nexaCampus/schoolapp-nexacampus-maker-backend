package grpcapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the NexaCampus control-plane gRPC services on top of the
// same SQLite/Turso catalogue the REST API uses.
type Server struct {
	UnimplementedSchoolServiceServer
	UnimplementedHealthServiceServer

	db            *sql.DB
	appBaseDomain string
	baseDomain    string
	startedAt     time.Time
}

// NewServer builds the gRPC server.
func NewServer(db *sql.DB, baseDomain, appBaseDomain string) *Server {
	if baseDomain == "" {
		baseDomain = "nexacampus.tech"
	}
	if appBaseDomain == "" {
		appBaseDomain = "school-webapp-self.vercel.app"
	}
	return &Server{
		db:            db,
		baseDomain:    strings.ToLower(strings.TrimSpace(baseDomain)),
		appBaseDomain: strings.ToLower(strings.TrimSpace(appBaseDomain)),
		startedAt:     time.Now().UTC(),
	}
}

const schoolColumns = `
	id, creator_phone, creator_name, leadership_position,
	school_name, teacher_count, affiliation_board, enrolled_scholars,
	city, state_territory, subdomain_slug, COALESCE(school_motto, ''),
	active_modules, student_domain, admin_domain, vercel_fallback_domain,
	provisioning_status, COALESCE(theme_color, '#0A192F'), created_at, updated_at
`

func (s *Server) scanSchool(row interface{ Scan(...any) error }) (*School, error) {
	var (
		school         School
		fallbackDomain string
		modulesRaw     string
		createdAt      time.Time
		updatedAt      time.Time
	)
	if err := row.Scan(
		&school.Id, &school.CreatorPhone, &school.CreatorName, &school.LeadershipPosition,
		&school.SchoolName, &school.TeacherCount, &school.AffiliationBoard, &school.EnrolledScholars,
		&school.City, &school.StateTerritory, &school.SubdomainSlug, &school.SchoolMotto,
		&modulesRaw, &school.StudentDomain, &school.AdminDomain, &fallbackDomain,
		&school.ProvisioningStatus, &school.ThemeColor, &createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}

	modules := map[string]bool{}
	if err := json.Unmarshal([]byte(modulesRaw), &modules); err != nil {
		modules = map[string]bool{}
	}
	school.ActiveModules = modules
	school.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	school.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	// Canonical URLs: the tenant lives on its own subdomain of the webapp host.
	slug := strings.ToLower(strings.TrimSpace(school.SubdomainSlug))
	if slug != "" {
		school.AppDomain = fmt.Sprintf("https://%s.%s", slug, s.appBaseDomain)
		school.AppAdminDomain = fmt.Sprintf("https://%s.admin.%s", slug, s.appBaseDomain)
		school.PathFallback = fmt.Sprintf("https://%s/school/%s", s.appBaseDomain, slug)
		if school.StudentDomain == "" {
			school.StudentDomain = fmt.Sprintf("https://%s.%s", slug, s.baseDomain)
		}
		if school.AdminDomain == "" {
			school.AdminDomain = fmt.Sprintf("https://%s.admin.%s", slug, s.baseDomain)
		}
	}

	return &school, nil
}

// ListSchools returns the tenant catalogue, optionally filtered.
func (s *Server) ListSchools(ctx context.Context, req *ListSchoolsRequest) (*ListSchoolsResponse, error) {
	query := "SELECT " + schoolColumns + " FROM schools"
	args := []any{}
	where := []string{}

	if b := strings.TrimSpace(req.GetBoard()); b != "" {
		where = append(where, "affiliation_board LIKE ?")
		args = append(args, "%"+b+"%")
	}
	if c := strings.TrimSpace(req.GetCity()); c != "" {
		where = append(where, "city LIKE ?")
		args = append(args, "%"+c+"%")
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC"

	if limit := req.GetLimit(); limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing schools: %v", err)
	}
	defer rows.Close()

	schools := []*School{}
	for rows.Next() {
		school, err := s.scanSchool(rows)
		if err != nil {
			continue
		}
		schools = append(schools, school)
	}
	if err := rows.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "reading schools: %v", err)
	}

	return &ListSchoolsResponse{Schools: schools, Total: int32(len(schools))}, nil
}

// GetSchool returns a single tenant by slug.
func (s *Server) GetSchool(ctx context.Context, req *GetSchoolRequest) (*GetSchoolResponse, error) {
	slug := strings.ToLower(strings.TrimSpace(req.GetSubdomainSlug()))
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "subdomain_slug is required")
	}

	row := s.db.QueryRowContext(ctx, "SELECT "+schoolColumns+" FROM schools WHERE subdomain_slug = ?", slug)
	school, err := s.scanSchool(row)
	if err == sql.ErrNoRows {
		return &GetSchoolResponse{Found: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "reading school: %v", err)
	}
	return &GetSchoolResponse{School: school, Found: true}, nil
}

// GetSchoolByCreator returns the tenant owned by a creator phone number.
func (s *Server) GetSchoolByCreator(ctx context.Context, req *GetSchoolByCreatorRequest) (*GetSchoolResponse, error) {
	phone := strings.TrimSpace(req.GetCreatorPhone())
	if phone == "" {
		return nil, status.Error(codes.InvalidArgument, "creator_phone is required")
	}

	row := s.db.QueryRowContext(
		ctx,
		"SELECT "+schoolColumns+" FROM schools WHERE creator_phone = ? ORDER BY created_at DESC LIMIT 1",
		phone,
	)
	school, err := s.scanSchool(row)
	if err == sql.ErrNoRows {
		return &GetSchoolResponse{Found: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "reading school: %v", err)
	}
	return &GetSchoolResponse{School: school, Found: true}, nil
}

// Health reports the health of the gRPC surface and its database.
func (s *Server) Health(ctx context.Context, _ *HealthRequest) (*HealthResponse, error) {
	database := "unknown"
	healthy := true

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.db.PingContext(pingCtx); err != nil {
		healthy = false
		database = "unreachable: " + err.Error()
	} else {
		database = "connected"
	}

	statusText := "SERVING"
	if !healthy {
		statusText = "NOT_SERVING"
	}

	return &HealthResponse{
		Healthy:       healthy,
		Status:        statusText,
		Database:      database,
		AppBaseDomain: s.appBaseDomain,
		UptimeSeconds: int64(time.Since(s.startedAt).Seconds()),
	}, nil
}
