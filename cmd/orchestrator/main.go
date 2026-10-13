package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/config"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/database"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/deployer"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/handlers"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/middleware"
	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/provisioner"
)

func main() {
	log.Println("[NexaCampus School Maker Orchestrator] Initializing control plane...")

	// 1. Load validated environment configuration
	cfg, err := config.LoadOrchestratorConfig()
	if err != nil {
		log.Fatalf("Fatal configuration error: %v", err)
	}

	// 2. Open Master SQLite database with pure Go driver in WAL mode
	db, err := database.OpenSQLite(cfg.MasterDBPath)
	if err != nil {
		log.Fatalf("Fatal master database connection failure: %v", err)
	}
	defer db.Close()

	// 3. Apply embedded schema migrations
	if err := database.MigrateMaster(db); err != nil {
		log.Fatalf("Fatal master schema migration failure: %v", err)
	}
	log.Println("[Database] Master SQLite catalog migrated successfully.")

	// 4. Initialize Quota Manager (Single-use Free Tier gatekeeper)
	quotaMgr := provisioner.NewQuotaManager(db)

	// 5. Initialize Cloud Deployers Strategy Map
	deployers := map[deployer.DeploymentTarget]deployer.Deployer{
		deployer.TargetRender: deployer.NewRenderDeployer(cfg.RenderAPIKey),
		deployer.TargetVercel: deployer.NewVercelDeployer(cfg.VercelToken),
		deployer.TargetAWS:    deployer.NewAWSDeployer(),
		deployer.TargetLocal:  deployer.NewMockDeployer(),
	}

	// 6. Initialize DNS & Custom Domain Binder
	dnsBinder := provisioner.NewDNSBinder(
		cfg.BaseDomain,
		cfg.RenderAPIKey,
		cfg.HostingerAPIToken,
		cfg.CloudflareAPIToken,
	)

	// 7. Initialize Tenant Orchestrator
	orch := provisioner.NewTenantOrchestrator(
		db,
		quotaMgr,
		deployers,
		dnsBinder,
		cfg.BaseDomain,
		cfg.TenantImage,
	)

	// 8. Construct HTTP Handlers & Routes
	h := handlers.NewOrchestratorHandler(orch, quotaMgr, db)
	mux := http.NewServeMux()

	// Public Health & Quotas
	mux.HandleFunc("/healthz", h.Healthz)
	mux.HandleFunc("/api/v1/deployment-options", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.GetDeploymentOptions(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	// Tenant Registration & Lifecycle
	mux.HandleFunc("/api/v1/tenants/validate-slug", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.ValidateSlug(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/tenants/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.RegisterTenant(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/tenants", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.ListTenants(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/tenants/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Count(r.URL.Path, "/") == 4 {
			h.GetTenant(w, r)
			return
		}
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	// 9. Build Middleware Chain (Recoverer, BodyLimit, RateLimiter, CORS, SecurityHeaders)
	rateLimiter := middleware.NewIPRateLimiter(cfg.RateLimitReqSec, cfg.RateLimitBurst)

	handler := middleware.Recoverer(
		middleware.SecurityHeaders(
			middleware.BodyLimit(10<<20)( // 10 MB maximum request payload
				rateLimiter.Handler(
					middleware.CORS([]string{"*"})(mux),
				),
			),
		),
	)

	// 10. Configure HTTP Server with strict timeouts for low-memory budget
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 11. Graceful shutdown handler
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[NexaCampus Orchestrator] Listening on http://0.0.0.0:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-shutdownSignal
	log.Println("[NexaCampus Orchestrator] Graceful shutdown signal received...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown forced: %v", err)
	}

	log.Println("[NexaCampus Orchestrator] Server exited cleanly.")
}
