# Deployment Guide

## Render Web Service Deployment

### 1. Build & Runtime Configuration
- **Runtime**: Docker (Multi-stage Alpine Linux with auto-toolchain)
- **Dockerfile Path**: `./Dockerfile`
- **Exposed Port**: `8080` (managed automatically via `PORT` environment variable)

### 2. Environment Variables Configuration
Configure the following environment variables in your Render Dashboard under **Environment**:

| Environment Variable | Description | Default / Example Value |
|---|---|---|
| `PORT` | HTTP port for the orchestrator control plane | `8080` |
| `GRPC_PORT` | gRPC port for the protobuf data-transfer service | `9090` |
| `ENV` | Environment identifier | `production` |
| `MASTER_SECRET_KEY` | 32+ character cryptographic key for JWT and session signing. If omitted, falls back to internal default. | `nexacampus_secret_master_key_super_secure_32bytes_long` |
| `TURSO_DATABASE_URL` | Remote persistent Turso Cloud SQLite (libSQL) connection URL — **the primary store** | `libsql://nexa-campus-parthasdey2304.aws-ap-south-1.turso.io` |
| `TURSO_AUTH_TOKEN` | Bearer token for authenticating to Turso Cloud cluster | `eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...` |
| `VERCEL_TOKEN` | Vercel API token for provisioning dynamic subdomains | Optional |
| `VERCEL_PROJECT_ID` | Vercel project ID for multi-tenant domain aliases | Optional |
| `BASE_DOMAIN` | Apex domain for tenant routing (DNS-bound custom domains) | `nexacampus.tech` |
| `APP_BASE_DOMAIN` | Host that serves the multi-tenant webapp. Every tenant is addressed as `<slug>.<APP_BASE_DOMAIN>` (student) and `<slug>.admin.<APP_BASE_DOMAIN>` (admin). | `school-webapp-self.vercel.app` |

### 2b. Database (Turso Cloud SQLite)
Turso is the **primary** store for the master catalogue:
- `TURSO_DATABASE_URL` + `TURSO_AUTH_TOKEN` present → the orchestrator connects to the remote libSQL cluster (`internal/database.OpenWithTurso`), applies the embedded master schema there and reconciles every tenant URL on boot.
- Both missing → it falls back to a local pure-Go SQLite file (`MASTER_DB_PATH`, default `data/master.db`) for development.
- Only one of the two present → the orchestrator refuses to start with a clear error, so a half-configured cluster can never silently drop writes to a local file.

### 2c. gRPC data-transfer service
- The orchestrator serves `nexacampus.v1.SchoolService` and `nexacampus.v1.HealthService` (protobuf definitions in `proto/nexacampus/v1/nexacampus.proto`) on `GRPC_PORT` in parallel with the REST API.
- Every call must carry the `x-master-secret` metadata header with the value of `MASTER_SECRET_KEY`; requests without it are rejected with `Unauthenticated`.
- The gRPC surface reads the same catalogue as REST, so the tenant URLs it returns (`app_domain`, `app_admin_domain`) always match the REST responses.

### 3. Startup Verification
The orchestrator validates connectivity to Turso Cloud SQLite on startup (`PingContext`), runs master schema migrations, reconciles the canonical tenant URLs (`https://<slug>.<APP_BASE_DOMAIN>`) and registers health check routes at `GET /healthz`.

