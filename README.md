# NexaCampus SchoolMaker Orchestrator Backend

Autonomous Multi-Tenant School Provisioning & Subdomain Routing Engine.

# NexaCampus School Maker Backend Orchestrator & Tenant Engine

A production-hardened, decoupled dual-engine backend architecture built in **Go (Golang)** and **Turso Cloud SQLite (libSQL)**, with a pure-Go local SQLite fallback, for automated multi-tenant school onboarding, cloud infrastructure provisioning, and isolated tenant runtimes.

**Storage**: the master tenant catalogue lives in **Turso Cloud SQLite** (`TURSO_DATABASE_URL` + `TURSO_AUTH_TOKEN`). The embedded schema migrations, the free-tier quota gate and the tenant URL reconciliation all run against that cluster. Local SQLite (`MASTER_DB_PATH`) is only used when no Turso credentials are configured.

**Data transfer**: tenants move over **REST** (public contract) and **gRPC** (`nexacampus.v1.SchoolService` / `HealthService` on `GRPC_PORT`, see `proto/nexacampus/v1/nexacampus.proto`).

**Tenant URLs**: every tenant is addressed as `https://<slug>.<APP_BASE_DOMAIN>` (student) and `https://<slug>.admin.<APP_BASE_DOMAIN>` (admin); the DNS-bound custom domain (`https://<slug>.nexacampus.tech`) and the path fallback (`/school/<slug>`) are kept as alternatives.

---

## 1. Architectural Architecture & Dual-Plane Model

The system operates across two strictly decoupled tiers:

```
┌────────────────────────────────────────────────────────────────────────┐
│                   CONTROL PLANE (The School Maker)                     │
│                       cmd/orchestrator/main.go                         │
│                                                                        │
│   • Onboarding Wizard & Tenant Registration                            │
│   • Single-Use Free Tier Gatekeeper (Render Workspace Quota Tracker)   │
│   • Provider Dispatcher Strategy (Render, Vercel, AWS, Local)         │
│   • Automated Subdomain-to-Backend DNS Binder (Hostinger / Cloudflare) │
│   • Master SQLite Catalog (master.db in WAL mode)                      │
└───────────────────┬────────────────────────────────┬───────────────────┘
                    │                                │
       POST /v1/services (Render API)    CNAME api-lcgvm.nexacampus.com
                    ▼                                ▼
┌──────────────────────────────────────┐  ┌──────────────────────────────┐
│ DATA PLANE: School Tenant A (lcgvm) │  │ DATA PLANE: School Tenant B  │
│      cmd/tenant-runtime/main.go      │  │  cmd/tenant-runtime/main.go  │
│                                      │  │                              │
│ • RAM Budget: < 40 MB (512 MB max)   │  │ • Independent SQLite db      │
│ • Pure Go SQLite in WAL mode         │  │ • Dedicated JWT Secret       │
│ • SetMaxOpenConns(1) zero contention │  │ • Isolated Circulars & Leads │
│ • HMAC-SHA256 JWT Admin Security     │  │ • Bound to custom subdomain  │
└──────────────────────────────────────┘  └──────────────────────────────┘
```

---

## 2. Core Architectural Pillars

### 2.1. Single-Use Free Tier Gatekeeper & Quota Enforcement
Render workspaces strictly permit **only one free web service** (750 instance hours/month total across the entire account). Indefinitely exposing the "Free Tier" causes subsequent deployments to fail or risks account suspension.
* **Master Flag (`platform_free_tier_consumed`):** Tracked in `master.db`.
* **Atomic Provisioning Lock:** When the first tenant consumes the free plan, an atomic transaction locks the flag:
  ```go
  err := quotaManager.ConsumeFreeTierAtomic(ctx, "lcgvm")
  ```
* **API Suppression:** `GET /api/v1/deployment-options` suppresses the free tier once consumed, returning only paid starter tiers (Render Starter @ 512 MB RAM) or prompt for Bring-Your-Own (BYO) cloud credentials (`RENDER_API_KEY`, `AWS_ACCESS_KEY`).
* Any subsequent registration attempting `plan: "free"` is rejected immediately with HTTP `409 Conflict`.

### 2.2. Automated Backend-to-Subdomain DNS Binding
When a new school backend is deployed (e.g. `nexacampus-api-lcgvm.onrender.com`):
1. **Custom Domain Registration:** Calls Render's API (`POST /v1/services/{id}/custom-domains`) registering `api-lcgvm.nexacampus.com`.
2. **DNS CNAME Injection:** Calls Hostinger / Cloudflare DNS API to insert:
   - **Host:** `api-lcgvm`
   - **Points To:** `nexacampus-api-lcgvm.onrender.com`
3. **Frontend Wiring:** The school's Next.js frontend (`lcgvm.nexacampus.com`) receives the direct backend URL via the `X-Backend-Host` header.

### 2.3. Anti-Vulnerability & Threat Modeling Hardening
* **Tenant Slug Validation & Path Sanitization:**
  - Strict regex: `^[a-z0-9][a-z0-9-]{2,30}[a-z0-9]$`.
  - Blocklist of reserved slugs (`api`, `admin`, `app`, `auth`, `billing`, `root`, `www`, `maker`, etc.).
  - Filepath cleaning with `filepath.Clean()` and `filepath.Abs()` preventing directory traversal.
* **SQL Injection Prevention:**
  - Zero raw string formatting (`fmt.Sprintf`) in SQL queries.
  - 100% parameterized queries (`?` placeholders).
* **DoS Prevention on 512 MB Budget:**
  - Server timeouts: `ReadTimeout: 5s`, `ReadHeaderTimeout: 2s`, `WriteTimeout: 10s`, `IdleTimeout: 120s`.
  - Max body size: `http.MaxBytesReader(w, r.Body, 10<<20)` (10 MB maximum).
  - Per-IP rate limiting token bucket (`golang.org/x/time/rate`).
* **SSRF Protection:**
  - Whitelist of cloud provider domains (`api.render.com`, `api.vercel.com`, `amazonaws.com`, `api.hostinger.com`, `api.cloudflare.com`).
  - Blocks loopback, private RFC1918 IPs, and cloud metadata (`169.254.169.254`).
* **Zero Panics:**
  - HTTP `Recoverer` catches all runtime panics, emitting 500 JSON without crashing shared processes.

---

## 3. Directory Structure

```
schoolapp-nexacampus-maker-backend/
├── cmd/
│   ├── orchestrator/          # School Maker Main Server
│   │   └── main.go
│   └── tenant-runtime/        # Isolated School Server
│       └── main.go
├── internal/
│   ├── config/                # Environment variables loader & validator
│   ├── database/              # SQLite connection, pragmas, embedded migrations
│   ├── deployer/              # Strategy pattern cloud dispatchers (Render, Vercel, AWS, Mock)
│   ├── handlers/              # Orchestrator & Tenant HTTP controllers
│   ├── middleware/            # Panic recoverer, rate limiting, body limit, CORS, security headers, JWT auth
│   ├── models/                # Request/response and database models
│   ├── provisioner/           # Free-tier quota lock, DNS binder, tenant orchestrator
│   └── security/              # Bcrypt, JWT, token generation, slug validation, SSRF guard
├── migrations/
│   ├── master/                # Master database schema
│   │   └── 001_init.sql
│   └── tenant/                # Tenant schema
│       └── 001_init.sql
├── Dockerfile.orchestrator    # Multi-stage minimal container for School Maker
├── Dockerfile.tenant          # Ultra-minimal container (< 20MB) for isolated school backends
├── .env.example
├── go.mod
└── go.sum
```

---

## 4. API Specification

### 4.1. Orchestrator Endpoints (School Maker)

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/healthz` | System telemetry, memory allocation stats, database status |
| `GET` | `/api/v1/deployment-options` | Available hosting tiers (suppresses free tier when claimed) |
| `POST` | `/api/v1/tenants/validate-slug` | Validates slug format, reservation, and uniqueness |
| `POST` | `/api/v1/tenants/register` | Provisions school backend, binds DNS, returns credentials |
| `GET` | `/api/v1/tenants` | Lists all registered schools |
| `GET` | `/api/v1/tenants/:slug` | Details and cloud status for a specific school |
| `POST` | `/api/v1/maker/auth/send-otp` | Dispatches the creator verification code |
| `POST` | `/api/v1/maker/auth/verify-otp` | Verifies the code and issues a 256-bit creator token |
| `POST` | `/api/v1/maker/auth/validate-token` | Validates an existing creator session token |
| `POST` | `/api/v1/maker/provision` | Provisions a school tenant and its canonical URLs |
| `GET` | `/api/v1/schools` | Tenant catalogue |
| `GET` | `/api/v1/schools/search?q=` | Tenant search |
| `GET` | `/api/v1/schools/check-slug?slug=` | Slug availability plus the tenant URLs |
| `GET` | `/api/v1/schools/:slug` | A single tenant record |
| `PATCH` | `/api/v1/schools/:slug` | Updates the profile or the 15-module configuration |
| `GET` | `/api/v1/maker/my-schools?phone=` | Tenants owned by a creator phone number |
| `POST` | `/api/v1/auth/student-login` | Roster login by admission number |

### 4.1b. gRPC Data-Transfer Service (`GRPC_PORT`, default 9090)

Protobuf contract: `proto/nexacampus/v1/nexacampus.proto` — generated stubs and the server live in `internal/grpcapi/`.

| RPC | Description |
|---|---|
| `nexacampus.v1.SchoolService/ListSchools` | Tenant catalogue, optionally filtered by board or city |
| `nexacampus.v1.SchoolService/GetSchool` | A single tenant by subdomain slug |
| `nexacampus.v1.SchoolService/GetSchoolByCreator` | The tenant owned by a creator phone number |
| `nexacampus.v1.HealthService/Health` | Service, database and configuration health |

Every call must carry the `x-master-secret` metadata header (value of `MASTER_SECRET_KEY`); calls without it are rejected with `Unauthenticated`. The responses carry the same canonical tenant URLs as the REST API (`app_domain`, `app_admin_domain`, `path_fallback`).

### 4.2. Tenant Runtime Endpoints (School Instance)

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| `GET` | `/healthz` | Public | Telemetry and memory usage (<40MB) |
| `GET` | `/api/v1/meta` | Public | School profile, crest, contact info, branding colors |
| `GET` | `/api/v1/notices` | Public | Paginated circulars and announcements |
| `GET` | `/api/v1/departments` | Public | Faculty directory and department list |
| `POST` | `/api/v1/admissions` | Public | Prospective student enquiry submission |
| `POST` | `/api/v1/admin/login` | Public | Bcrypt verification, returns HMAC-SHA256 JWT |
| `POST` | `/api/v1/admin/notices` | JWT | Create school notice |
| `PUT` | `/api/v1/admin/notices/:id` | JWT | Update notice |
| `DELETE` | `/api/v1/admin/notices/:id` | JWT | Delete notice |
| `GET` | `/api/v1/admin/admissions` | JWT | Review and export admission leads |
| `PUT` | `/api/v1/admin/theme` | JWT | Update branding color codes, tagline, banners |

---

## 5. Development & Testing

### Running Tests
```bash
go test -v ./...
```

### Running the Orchestrator Locally
```bash
export MASTER_SECRET_KEY="development-master-secret-key-32-bytes-minimum!"
export ENV="development"
go run ./cmd/orchestrator
```

### Running a Tenant Runtime Locally
```bash
export TENANT_SLUG="lcgvm"
export SCHOOL_NAME="Lions Calcutta Greater Vidya Mandir"
export JWT_SECRET="development-tenant-jwt-secret-key-32-bytes-minimum!"
export ENV="development"
go run ./cmd/tenant-runtime
```

### Building Docker Images
```bash
# Minimal Tenant Runtime (< 20MB)
docker build -f Dockerfile.tenant -t ghcr.io/nexacampus/school-tenant:latest .

# Orchestrator
docker build -f Dockerfile.orchestrator -t ghcr.io/nexacampus/school-orchestrator:latest .
```

