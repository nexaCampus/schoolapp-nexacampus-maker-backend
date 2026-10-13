package migrations

import _ "embed"

//go:embed master/001_init.sql
var MasterSchemaSQL string

//go:embed tenant/001_init.sql
var TenantSchemaSQL string
