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

-- Creator Verification Sessions (Issue 1)
CREATE TABLE IF NOT EXISTS creator_sessions (
    phone TEXT PRIMARY KEY,
    creator_name TEXT NOT NULL,
    otp_code TEXT NOT NULL,
    token TEXT NOT NULL,
    verified INTEGER NOT NULL DEFAULT 0,
    verified_at DATETIME,
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL
);

-- Schools Master Registry (Issue 2 & Issue 4)
CREATE TABLE IF NOT EXISTS schools (
    id TEXT PRIMARY KEY,
    creator_phone TEXT NOT NULL,
    creator_name TEXT NOT NULL,
    leadership_position TEXT NOT NULL,
    school_name TEXT NOT NULL,
    teacher_count TEXT NOT NULL,
    affiliation_board TEXT NOT NULL,
    enrolled_scholars TEXT NOT NULL,
    city TEXT NOT NULL,
    state_territory TEXT NOT NULL,
    subdomain_slug TEXT UNIQUE NOT NULL,
    school_motto TEXT,
    active_modules TEXT NOT NULL,
    student_domain TEXT NOT NULL,
    admin_domain TEXT NOT NULL,
    vercel_fallback_domain TEXT NOT NULL,
    provisioning_status TEXT NOT NULL DEFAULT 'active',
    theme_color TEXT DEFAULT '#0A192F',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_schools_slug ON schools(subdomain_slug);
CREATE INDEX IF NOT EXISTS idx_schools_creator_phone ON schools(creator_phone);

-- Seed pre-configured institutions (LCGVM demo + Greenwood Centennial)
INSERT OR IGNORE INTO schools (
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
    '{"canteen":true,"bus":true,"bus_transport":true,"library":true,"attendance":true,"diary":true,"homework_diary":true,"exams":true,"exams_reports":true,"timetable":true,"timetable_bell":true,"fees":true,"fee_ledger":true,"hallpass":true,"ehallpass":true,"clubs":true,"clubs_houses":true,"health":true,"medical_infirmary":true,"notices":true,"circulars_notices":true,"ptm":true,"store":true,"store_uniforms":true,"lostfound":true,"lost_found":true}',
    'https://lcgvm.nexacampus.tech',
    'https://lcgvm.admin.nexacampus.tech',
    'https://lcgvm.school-webapp-self.vercel.app',
    'active',
    '#0A192F',
    '2026-01-01 00:00:00',
    '2026-01-01 00:00:00'
);

INSERT OR IGNORE INTO schools (
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
    '{"canteen":true,"bus":true,"bus_transport":true,"library":true,"attendance":true,"diary":true,"homework_diary":true,"exams":true,"exams_reports":true,"timetable":true,"timetable_bell":true,"fees":true,"fee_ledger":true,"hallpass":false,"ehallpass":false,"clubs":true,"clubs_houses":true,"health":true,"medical_infirmary":true,"notices":true,"circulars_notices":true,"ptm":true,"store":false,"store_uniforms":false,"lostfound":false,"lost_found":false}',
    'https://greenwood.nexacampus.tech',
    'https://greenwood.admin.nexacampus.tech',
    'https://greenwood.school-webapp-self.vercel.app',
    'active',
    '#1B382B',
    '2026-01-01 00:00:00',
    '2026-01-01 00:00:00'
);
