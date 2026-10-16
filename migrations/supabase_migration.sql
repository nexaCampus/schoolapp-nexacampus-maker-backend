-- ============================================================================
-- NexaCampus PostgreSQL / Supabase Database Migration
-- Multi-Tenant Institutional School Cloud & Maker Catalog
-- Compatible with Supabase SQL Editor and Standard PostgreSQL 14+
-- ============================================================================

-- 1. Enable UUID Extension if not already active
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 2. Master Platform Metadata
CREATE TABLE IF NOT EXISTS platform_meta (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO platform_meta (key, value)
VALUES ('platform_free_tier_consumed', 'false')
ON CONFLICT (key) DO NOTHING;

-- 3. Tenants / Cloud Runtime Instances
CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    slug VARCHAR(64) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    plan VARCHAR(32) NOT NULL DEFAULT 'starter',
    provider VARCHAR(32) NOT NULL DEFAULT 'render',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    service_id VARCHAR(128),
    backend_url VARCHAR(255),
    custom_domain VARCHAR(255),
    admin_username VARCHAR(128) NOT NULL DEFAULT 'admin',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenants_slug ON tenants(slug);
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);

-- 4. Deployment Execution Logs
CREATE TABLE IF NOT EXISTS deployments (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    tenant_slug VARCHAR(64) NOT NULL REFERENCES tenants(slug) ON DELETE CASCADE,
    provider VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    service_id VARCHAR(128),
    logs TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deployments_tenant_slug ON deployments(tenant_slug);

-- 5. Creator Phone & Verification Sessions (Issue 1)
CREATE TABLE IF NOT EXISTS creator_sessions (
    phone VARCHAR(32) PRIMARY KEY,
    creator_name VARCHAR(255) NOT NULL,
    otp_code VARCHAR(16) NOT NULL,
    token TEXT NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_creator_sessions_token ON creator_sessions(token);

-- 6. Unified Multi-Tenant Schools Master Registry (Issue 2 & Issue 4)
CREATE TABLE IF NOT EXISTS schools (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    creator_phone VARCHAR(32) NOT NULL,
    creator_name VARCHAR(255) NOT NULL,
    leadership_position VARCHAR(128) NOT NULL,
    school_name VARCHAR(255) NOT NULL,
    teacher_count VARCHAR(32) NOT NULL,
    affiliation_board VARCHAR(128) NOT NULL,
    enrolled_scholars VARCHAR(32) NOT NULL,
    city VARCHAR(128) NOT NULL,
    state_territory VARCHAR(128) NOT NULL,
    subdomain_slug VARCHAR(64) UNIQUE NOT NULL,
    school_motto TEXT,
    active_modules JSONB NOT NULL DEFAULT '{}'::jsonb,
    student_domain VARCHAR(255) NOT NULL,
    admin_domain VARCHAR(255) NOT NULL,
    vercel_fallback_domain VARCHAR(255) NOT NULL,
    provisioning_status VARCHAR(32) NOT NULL DEFAULT 'active',
    theme_color VARCHAR(32) DEFAULT '#0A192F',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_schools_slug ON schools(subdomain_slug);
CREATE INDEX IF NOT EXISTS idx_schools_creator_phone ON schools(creator_phone);

-- 7. Seed Flagship Institutions (LCGVM Demo + Greenwood Centennial)
INSERT INTO schools (
    id, creator_phone, creator_name, leadership_position,
    school_name, teacher_count, affiliation_board, enrolled_scholars,
    city, state_territory, subdomain_slug, school_motto,
    active_modules, student_domain, admin_domain, vercel_fallback_domain,
    provisioning_status, theme_color, created_at, updated_at
) VALUES (
    'sch-lcgvm',
    '+91 6289652321',
    'Dr. Rajesh Sharma',
    'Principal / Head of Institution',
    'Lions Calcutta Greater Vidya Mandir',
    '124',
    'CISCE / ICSE & ISC',
    '3,200',
    'Kolkata',
    'West Bengal',
    'lcgvm',
    'Tamaso Ma Jyotirgamaya (Knowledge is Light)',
    '{"canteen":true,"bus":true,"bus_transport":true,"library":true,"attendance":true,"diary":true,"homework_diary":true,"exams":true,"exams_reports":true,"timetable":true,"timetable_bell":true,"fees":true,"fee_ledger":true,"hallpass":true,"ehallpass":true,"clubs":true,"clubs_houses":true,"health":true,"medical_infirmary":true,"notices":true,"circulars_notices":true,"ptm":true,"store":true,"store_uniforms":true,"lostfound":true,"lost_found":true}'::jsonb,
    'https://lcgvm.nexacampus.tech',
    'https://lcgvm.admin.nexacampus.tech',
    'https://school-webapp-self.vercel.app/s/lcgvm',
    'active',
    '#0A192F',
    NOW(),
    NOW()
)
ON CONFLICT (subdomain_slug) DO UPDATE
SET school_name = EXCLUDED.school_name,
    active_modules = EXCLUDED.active_modules,
    updated_at = NOW();

INSERT INTO schools (
    id, creator_phone, creator_name, leadership_position,
    school_name, teacher_count, affiliation_board, enrolled_scholars,
    city, state_territory, subdomain_slug, school_motto,
    active_modules, student_domain, admin_domain, vercel_fallback_domain,
    provisioning_status, theme_color, created_at, updated_at
) VALUES (
    'sch-greenwood',
    '+91 9830123456',
    'Dean Arthur Pendelton',
    'Founder / Co-Founder',
    'Greenwood Centennial Academy',
    '48',
    'CBSE (Central Board)',
    '1,850',
    'Kolkata',
    'West Bengal',
    'greenwood',
    'Veritas et Virtus (Truth & Virtue)',
    '{"canteen":true,"bus":true,"bus_transport":true,"library":true,"attendance":true,"diary":true,"homework_diary":true,"exams":true,"exams_reports":true,"timetable":true,"timetable_bell":true,"fees":true,"fee_ledger":true,"hallpass":false,"ehallpass":false,"clubs":true,"clubs_houses":true,"health":true,"medical_infirmary":true,"notices":true,"circulars_notices":true,"ptm":true,"store":false,"store_uniforms":false,"lostfound":false,"lost_found":false}'::jsonb,
    'https://greenwood.nexacampus.tech',
    'https://greenwood.admin.nexacampus.tech',
    'https://school-webapp-self.vercel.app/s/greenwood',
    'active',
    '#1B382B',
    NOW(),
    NOW()
)
ON CONFLICT (subdomain_slug) DO UPDATE
SET school_name = EXCLUDED.school_name,
    active_modules = EXCLUDED.active_modules,
    updated_at = NOW();
