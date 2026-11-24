// Package migrations 以 embed 方式打包版本化 SQL 迁移。
//
// 文件命名：<version>_<name>.up.sql / <version>_<name>.down.sql。
// 迁移由 internal/data.Migrate 在启动时按版本顺序执行，并记录到 schema_migrations。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
