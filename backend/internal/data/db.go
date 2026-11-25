// Package data 实现仓储与数据库初始化。
package data

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 打开 MySQL 5.7.18；DSN 需带 parseTime=true&loc=UTC&charset=utf8mb4。
func Open(dsn string) (*gorm.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("HM_DB_DSN 未配置")
	}
	db, err := gorm.Open(mysql.Open(withUTCSession(dsn)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

// withUTCSession 强制每条连接的会话时区为 UTC。
// 云托管 MySQL 的系统时区是 +08:00，若不管，NOW(3)/CURRENT_TIMESTAMP 写进 datetime 的是
// 本地墙上时间，而 Go 侧写入的是 UTC，两者混在一列里会差 8 小时（2026-11-25 线上发现：
// bookings.reviewed_at 比 ledger.occurred_at 快 8 小时）。time_zone 是 MySQL 系统变量，
// go-sql-driver 会在握手时执行 SET time_zone='+00:00'。
func withUTCSession(dsn string) string {
	if strings.Contains(dsn, "time_zone=") {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "time_zone=%27%2B00%3A00%27"
}

const migrationLock = "hm_migrate"

// splitSQL 按“行尾分号”切分 DDL，忽略 -- 注释与空行。
func splitSQL(s string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			out = append(out, strings.TrimSpace(b.String()))
			b.Reset()
		}
	}
	if rest := strings.TrimSpace(b.String()); rest != "" {
		out = append(out, rest)
	}
	return out
}
