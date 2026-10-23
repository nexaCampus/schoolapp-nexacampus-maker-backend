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
    'https://lcgvm.nexacampus.school',
    'https://lcgvm.admin.nexacampus.school',
    'https://lcgvm.nexacampus.school',
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
    'https://greenwood.nexacampus.school',
    'https://greenwood.admin.nexacampus.school',
    'https://greenwood.nexacampus.school',
    'active',
    '#1B382B',
    '2026-01-01 00:00:00',
    '2026-01-01 00:00:00'
);


-- Relational Subdomain Bindings
CREATE TABLE IF NOT EXISTS school_subdomains (
    id TEXT PRIMARY KEY,
    school_id TEXT NOT NULL,
    subdomain_slug TEXT NOT NULL,
    domain_type TEXT NOT NULL,
    full_domain TEXT NOT NULL,
    target_cname TEXT NOT NULL DEFAULT 'cname.vercel-dns.com',
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL,
    FOREIGN KEY(school_id) REFERENCES schools(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_subdomains_slug ON school_subdomains(subdomain_slug);
CREATE INDEX IF NOT EXISTS idx_subdomains_school_id ON school_subdomains(school_id);

-- Relational School Modules Configuration
CREATE TABLE IF NOT EXISTS school_modules (
    id TEXT PRIMARY KEY,
    school_id TEXT NOT NULL,
    module_key TEXT NOT NULL,
    is_enabled INTEGER NOT NULL DEFAULT 1,
    updated_at DATETIME NOT NULL,
    UNIQUE(school_id, module_key),
    FOREIGN KEY(school_id) REFERENCES schools(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_school_modules_school ON school_modules(school_id);

-- Maker Audit Trail
CREATE TABLE IF NOT EXISTS maker_audit_logs (
    id TEXT PRIMARY KEY,
    school_id TEXT,
    creator_phone TEXT NOT NULL,
    action TEXT NOT NULL,
    details TEXT,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_creator ON maker_audit_logs(creator_phone);
CREATE INDEX IF NOT EXISTS idx_audit_school ON maker_audit_logs(school_id);
