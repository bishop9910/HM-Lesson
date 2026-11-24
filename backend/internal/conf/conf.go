package conf

import (
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// Config 是 M1 的最小配置；M2 接入 Kratos config 后保持字段名不变。
type Config struct {
	Env           string
	HTTPAddr      string
	DBDriver      string
	DBDSN         string
	JWTSecret     string
	StorageDriver string
	StorageBucket string
	StorageRegion string
	WXAppID       string
	WXAppSecret   string
	LogLevelName  string
	Timezone      string

	// 首次启动引导：库里没有任何老师时创建第一个 owner。
	// 生产环境通过云托管环境变量注入，不要写默认密码。
	BootstrapName     string
	BootstrapPhone    string
	BootstrapPassword string
}

func Load() Config {
	storageDriver := env("HM_STORAGE_DRIVER", "")
	if storageDriver == "" && os.Getenv("COS_BUCKET") != "" {
		storageDriver = "cloud"
	}
	if storageDriver == "" {
		storageDriver = "minio"
	}

	return Config{
		Env: env("HM_ENV", "dev"),
		// 本地默认 :8000；云托管通过 Dockerfile/环境变量覆盖为 :80。
		HTTPAddr:      env("HM_HTTP_ADDR", "0.0.0.0:8000"),
		DBDriver:      env("HM_DB_DRIVER", "mysql"),
		DBDSN:         buildDSN(),
		JWTSecret:     env("HM_JWT_SECRET", ""),
		StorageDriver: storageDriver,
		StorageBucket: env("HM_STORAGE_BUCKET", env("COS_BUCKET", "hm-dev")),
		StorageRegion: env("HM_STORAGE_REGION", env("COS_REGION", "ap-shanghai")),
		WXAppID:       env("HM_WX_APP_ID", ""),
		WXAppSecret:   env("HM_WX_APP_SECRET", ""),
		LogLevelName:  env("HM_LOG_LEVEL", "info"),
		Timezone:      env("HM_TIMEZONE", "Asia/Shanghai"),

		BootstrapName:     env("HM_BOOTSTRAP_NAME", "老师"),
		BootstrapPhone:    env("HM_BOOTSTRAP_PHONE", ""),
		BootstrapPassword: env("HM_BOOTSTRAP_PASSWORD", ""),
	}
}

// buildDSN 优先使用显式 HM_DB_DSN；否则用云托管自动注入的 MYSQL_* 变量拼装。
// 云托管不会注入数据库名；优先显式 HM_DB_NAME，其次 MYSQL_DATABASE，最后 hm_lesson。
func buildDSN() string {
	if dsn := env("HM_DB_DSN", ""); dsn != "" {
		return dsn
	}
	addr := env("MYSQL_ADDRESS", "")
	if addr == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "3306")
	}

	cfg := mysqlDriver.NewConfig()
	cfg.User = env("MYSQL_USERNAME", "")
	cfg.Passwd = env("MYSQL_PASSWORD", "")
	cfg.Net = "tcp"
	cfg.Addr = addr
	cfg.DBName = env("HM_DB_NAME", env("MYSQL_DATABASE", "hm_lesson"))
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	return cfg.FormatDSN()
}

func (c Config) LogLevel() slog.Level {
	switch strings.ToLower(c.LogLevelName) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
