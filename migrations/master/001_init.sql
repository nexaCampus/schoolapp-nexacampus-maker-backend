-- Master Orchestrator Schema
CREATE TABLE IF NOT EXISTS platform_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Seed single-use free tier flag if not present
INSERT OR IGNORE INTO platform_meta (key, value) VALUES ('platform_free_tier_consumed', 'false');

CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    plan TEXT NOT NULL,
    provider TEXT NOT NULL,
    status TEXT NOT NULL,
    service_id TEXT,
    backend_url TEXT,
    custom_domain TEXT,
    admin_username TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tenants_slug ON tenants(slug);
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);

CREATE TABLE IF NOT EXISTS deployments (
    id TEXT PRIMARY KEY,
    tenant_slug TEXT NOT NULL,
    provider TEXT NOT NULL,
    status TEXT NOT NULL,
    service_id TEXT,
    logs TEXT,
    created_at DATETIME NOT NULL,
    FOREIGN KEY(tenant_slug) REFERENCES tenants(slug) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_deployments_tenant_slug ON deployments(tenant_slug);
