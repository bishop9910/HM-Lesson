package data

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/migrations"
)

var migrationName = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.(up|down)\.sql$`)

type migration struct {
	version int64
	name    string
	up      []byte
	down    []byte
}

// Migrate 按版本顺序执行未应用的 up 迁移，返回本次应用的版本号。
// 使用 MySQL GET_LOCK 串行化多实例启动；DDL 在 MySQL 中隐式提交，失败时需人工修复后重试。
func Migrate(ctx context.Context, db *gorm.DB) ([]int64, error) {
	return runMigrations(ctx, db, true, 0)
}

// MigrateDown 回滚最近的 steps 个版本；steps<=0 表示回滚全部。
func MigrateDown(ctx context.Context, db *gorm.DB, steps int) ([]int64, error) {
	return runMigrations(ctx, db, false, steps)
}

// Version 返回数据库当前已应用的最高迁移版本。
func Version(ctx context.Context, db *gorm.DB) (int64, error) {
	var v int64
	if err := db.WithContext(ctx).Raw(
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&v).Error; err != nil {
		return 0, err
	}
	return v, nil
}

func runMigrations(ctx context.Context, db *gorm.DB, up bool, steps int) ([]int64, error) {
	all, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	var applied []int64
	err = db.Connection(func(tx *gorm.DB) error {
		tx = tx.WithContext(ctx)
		var locked int
		if err := tx.Raw("SELECT GET_LOCK(?, 30)", migrationLock).Scan(&locked).Error; err != nil {
			return fmt.Errorf("获取迁移锁: %w", err)
		}
		if locked != 1 {
			return errors.New("获取迁移锁超时，可能有其他实例正在迁移")
		}
		defer func() { _ = tx.Exec("SELECT RELEASE_LOCK(?)", migrationLock).Error }()

		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT NOT NULL PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error; err != nil {
			return fmt.Errorf("创建 schema_migrations: %w", err)
		}

		current, err := versionTx(tx)
		if err != nil {
			return err
		}

		if up {
			for _, m := range all {
				if m.version <= current {
					continue
				}
				if err := apply(tx, m, true); err != nil {
					return err
				}
				if err := tx.Exec(
					"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
					m.version, m.name,
				).Error; err != nil {
					return fmt.Errorf("记录迁移 %d: %w", m.version, err)
				}
				applied = append(applied, m.version)
			}
			return nil
		}

		// 回滚：从高到低；steps<=0 表示全部。
		for i := len(all) - 1; i >= 0; i-- {
			m := all[i]
			if m.version > current {
				continue
			}
			if err := apply(tx, m, false); err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM schema_migrations WHERE version = ?", m.version).Error; err != nil {
				return fmt.Errorf("删除迁移记录 %d: %w", m.version, err)
			}
			applied = append(applied, m.version)
			if steps > 0 && len(applied) >= steps {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return applied, nil
}

func versionTx(tx *gorm.DB) (int64, error) {
	var v int64
	if err := tx.Raw("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&v).Error; err != nil {
		return 0, fmt.Errorf("读取迁移版本: %w", err)
	}
	return v, nil
}

func apply(tx *gorm.DB, m migration, up bool) error {
	label := "down"
	body := m.down
	if up {
		label = "up"
		body = m.up
	}
	for _, stmt := range splitSQL(string(body)) {
		if err := tx.Exec(stmt).Error; err != nil {
			return fmt.Errorf("迁移 %d_%s.%s 失败: %w", m.version, m.name, label, err)
		}
	}
	return nil
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, err
	}
	byVersion := make(map[int64]*migration)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		mm := migrationName.FindStringSubmatch(e.Name())
		if mm == nil {
			continue
		}
		version, err := strconv.ParseInt(mm[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("迁移版本 %q 非法: %w", e.Name(), err)
		}
		body, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			return nil, err
		}
		m, ok := byVersion[version]
		if !ok {
			m = &migration{version: version, name: mm[2]}
			byVersion[version] = m
		}
		if mm[3] == "up" {
			m.up = body
		} else {
			m.down = body
		}
	}
	out := make([]migration, 0, len(byVersion))
	for _, m := range byVersion {
		if len(m.up) == 0 || len(m.down) == 0 {
			return nil, fmt.Errorf("迁移 %d_%s 缺少 up 或 down 文件", m.version, m.name)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}
