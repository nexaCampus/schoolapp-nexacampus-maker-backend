package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/config"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/database"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/handlers"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/middleware"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/security"
)

func main() {
	log.Println("[NexaCampus Tenant Runtime] Booting isolated school engine...")

	// 1. Load validated tenant configuration
	cfg, err := config.LoadTenantConfig()
	if err != nil {
		log.Fatalf("Fatal tenant configuration error: %v", err)
	}

	log.Printf("[Tenant Engine] Active Tenant Slug: '%s'", cfg.TenantSlug)

	// 2. Open embedded SQLite database in WAL mode with connection limits
	db, err := database.OpenSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("Fatal tenant database connection error: %v", err)
	}
	defer db.Close()

	// 3. Run embedded schema migrations
	if err := database.MigrateTenant(db); err != nil {
		log.Fatalf("Fatal tenant schema migration error: %v", err)
	}
	log.Println("[Database] Tenant schema verified.")

	// 4. Seed initial admin user if configured
	adminPassHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if adminPassHash == "" {
		// Generate fallback default in dev/test
		adminPassHash, _ = security.HashPassword("Admin@NexaCampus2026!")
	}
	_ = database.SeedTenantAdmin(db, uuid.New().String(), "admin", adminPassHash)
	_ = database.SeedSchoolMeta(db, cfg.SchoolName, "#3b49df", "#12131a")

	// 5. Construct HTTP Handlers & Routes
	jwtSecretBytes := []byte(cfg.JWTSecret)
	h := handlers.NewTenantHandler(db, cfg.TenantSlug, jwtSecretBytes)
	mux := http.NewServeMux()

	// Public Endpoints
	mux.HandleFunc("/healthz", h.Healthz)

	mux.HandleFunc("/api/v1/meta", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.GetMeta(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/notices", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.GetNotices(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/departments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.GetDepartments(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/admissions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.SubmitAdmission(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	// Public Admin Login
	mux.HandleFunc("/api/v1/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.AdminLogin(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	// Protected Admin Endpoints (Guarded by JWTAuth)
	adminAuth := middleware.JWTAuth(jwtSecretBytes)

	adminNoticesHandler := adminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.AdminCreateNotice(w, r)
		case http.MethodPut:
			h.AdminUpdateNotice(w, r)
		case http.MethodDelete:
			h.AdminDeleteNotice(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.Handle("/api/v1/admin/notices", adminNoticesHandler)
	mux.Handle("/api/v1/admin/notices/", adminNoticesHandler)

	adminAdmissionsHandler := adminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.AdminGetAdmissions(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}))
	mux.Handle("/api/v1/admin/admissions", adminAdmissionsHandler)

	adminThemeHandler := adminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			h.AdminUpdateTheme(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}))
	mux.Handle("/api/v1/admin/theme", adminThemeHandler)

	// 6. Build Middleware Pipeline
	rateLimiter := middleware.NewIPRateLimiter(cfg.RateLimitReqSec, cfg.RateLimitBurst)

	handler := middleware.Recoverer(
		middleware.SecurityHeaders(
			middleware.BodyLimit(10<<20)(
				rateLimiter.Handler(
					middleware.CORS(cfg.AllowedOrigins)(mux),
				),
			),
		),
	)

	// Inject X-Backend-Host header into responses
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if host == "" {
			host = cfg.TenantSlug + ".nexacampus.com"
		}
		w.Header().Set("X-Backend-Host", host)
		w.Header().Set("X-Tenant-Slug", cfg.TenantSlug)
		handler.ServeHTTP(w, r)
	})

	// 7. Configure HTTP Server
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           wrappedHandler,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 8. Graceful shutdown
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[Tenant Engine: %s] Serving HTTP traffic on port %s", cfg.TenantSlug, cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-shutdownChan
	log.Printf("[Tenant Engine: %s] Shutting down gracefully...", cfg.TenantSlug)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Forced shutdown error: %v", err)
	}

	log.Printf("[Tenant Engine: %s] Stopped cleanly.", cfg.TenantSlug)
}
