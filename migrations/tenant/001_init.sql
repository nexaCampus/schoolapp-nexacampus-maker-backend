-- Tenant School Runtime Schema
CREATE TABLE IF NOT EXISTS school_meta (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL,
    tagline TEXT,
    crest_url TEXT,
    affiliation TEXT,
    contact_email TEXT,
    phone TEXT,
    address TEXT,
    theme_primary_color TEXT DEFAULT '#3b49df',
    theme_secondary_color TEXT DEFAULT '#12131a',
    banner_url TEXT,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS notices (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    category TEXT NOT NULL,
    attachment_url TEXT,
    is_pinned BOOLEAN DEFAULT FALSE,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notices_created_at ON notices(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notices_category ON notices(category);

CREATE TABLE IF NOT EXISTS departments (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    head_of_department TEXT,
    faculty_count INTEGER DEFAULT 0,
    description TEXT
);

CREATE TABLE IF NOT EXISTS admissions (
    id TEXT PRIMARY KEY,
    student_name TEXT NOT NULL,
    parent_name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT NOT NULL,
    grade_applying TEXT NOT NULL,
    previous_school TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    notes TEXT,
    submitted_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_admissions_status ON admissions(status);
CREATE INDEX IF NOT EXISTS idx_admissions_submitted ON admissions(submitted_at DESC);

CREATE TABLE IF NOT EXISTS admins (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL
);
