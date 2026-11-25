package conf

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
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

	// 微信订阅消息：模板 ID 与字段映射由环境变量提供，未配置时功能整体 no-op。
	// 原始 JSON 见 SubscribeTemplates 环境变量说明；解析失败只告警，不影响服务启动。
	WXSubscribeTemplates string
	WXTemplateLang       string
	WXMiniprogramState   string

	// WXCallMode 选择调用微信接口的通道：
	//   direct（默认）—— 直连 api.weixin.qq.com，用 appid+secret 换 access_token；
	//   cloud        —— 走云托管「开放接口服务（云调用）」，不取 token，由平台网关鉴权。
	// 云托管生产环境 api.weixin.qq.com 被解析到平台内网网关，只有 cloud 模式能通。
	WXCallMode string

	// 首次启动引导：库里没有任何老师时创建第一个 owner。
	// 生产环境通过云托管环境变量注入，不要写默认密码。
	BootstrapName     string
	BootstrapPhone    string
	BootstrapPassword string

	// LessonHoursPerSession 是一次一对一课默认扣减的课时数（需求 §3.3.2，默认 1）。
	LessonHoursPerSession float64

	// SystemPassword 是内部系统账号（用户名固定 system）的初始密码。
	// 只在账号首次创建或密码为空时写入，之后本人可在小程序里自助改密。
	SystemPassword string
}

// SubscribeTemplate 是一个订阅消息模板的配置：模板 ID + 微信模板关键词到内容字段的映射。
// 关键词名（thing1/time2/phrase3…）由所选模板决定，必须由运营方在微信后台看到后填写。
type SubscribeTemplate struct {
	ID     string            `json:"id"`
	Fields map[string]string `json:"fields"`
}

// ParseSubscribeTemplates 解析 HM_WX_SUBSCRIBE_TEMPLATES。
// 形如：{"task_published":{"id":"xxx","fields":{"thing1":"title","time2":"due_at"}}}
// 空字符串表示不启用；缺 id 或缺 fields 视为配置错误，避免静默发送空内容。
func ParseSubscribeTemplates(raw string) (map[string]SubscribeTemplate, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var parsed map[string]SubscribeTemplate
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("HM_WX_SUBSCRIBE_TEMPLATES 不是合法 JSON: %w", err)
	}
	for key, tpl := range parsed {
		if strings.TrimSpace(tpl.ID) == "" {
			return nil, fmt.Errorf("订阅消息模板 %q 缺少 id", key)
		}
		if len(tpl.Fields) == 0 {
			return nil, fmt.Errorf("订阅消息模板 %q 缺少 fields 映射", key)
		}
	}
	return parsed, nil
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

		WXSubscribeTemplates: env("HM_WX_SUBSCRIBE_TEMPLATES", ""),
		// formal=正式版；trial=体验版；developer=开发版（真机调试用）。
		WXTemplateLang:     env("HM_WX_TMPL_LANG", "zh_CN"),
		WXMiniprogramState: env("HM_WX_MP_STATE", "formal"),
		WXCallMode:         normalizeCallMode(env("HM_WX_CALL_MODE", "direct")),

		BootstrapName:     env("HM_BOOTSTRAP_NAME", "老师"),
		BootstrapPhone:    env("HM_BOOTSTRAP_PHONE", ""),
		BootstrapPassword: env("HM_BOOTSTRAP_PASSWORD", ""),

		LessonHoursPerSession: envFloat("HM_LESSON_HOURS_PER_SESSION", 1),

		SystemPassword: env("HM_SYSTEM_PASSWORD", ""),
	}
}

// normalizeCallMode 只接受 direct / cloud，大小写与空值容错，其余回退 direct。
// 云托管生产必须显式配 HM_WX_CALL_MODE=cloud，否则请求会打到被劫持的地址。
func normalizeCallMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "cloud", "yun", "wxcloud":
		return "cloud"
	default:
		return "direct"
	}
}

// envFloat 解析正数浮点环境变量，非法值回退默认值。
func envFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
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
