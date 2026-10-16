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
| `ENV` | Environment identifier | `production` |
| `MASTER_SECRET_KEY` | 32+ character cryptographic key for JWT and session signing. If omitted, falls back to internal default. | `nexacampus_secret_master_key_super_secure_32bytes_long` |
| `TURSO_DATABASE_URL` | Remote persistent Turso Cloud SQLite (libSQL) connection URL | `libsql://nexa-campus-parthasdey2304.aws-ap-south-1.turso.io` |
| `TURSO_AUTH_TOKEN` | Bearer token for authenticating to Turso Cloud cluster | `eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...` |
| `VERCEL_TOKEN` | Vercel API token for provisioning dynamic subdomains | Optional |
| `VERCEL_PROJECT_ID` | Vercel project ID for multi-tenant domain aliases | Optional |
| `BASE_DOMAIN` | Apex domain for tenant routing | `nexacampus.tech` |

### 3. Startup Verification
The orchestrator validates connectivity to Turso Cloud SQLite on startup (`PingContext`), runs master schema migrations, and registers health check routes at `GET /healthz`.

