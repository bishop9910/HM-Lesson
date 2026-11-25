package conf

import (
	"log/slog"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}
	for _, c := range cases {
		cfg := Config{LogLevelName: c.in}
		if got := cfg.LogLevel(); got != c.want {
			t.Fatalf("LogLevel(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestEnvFallback(t *testing.T) {
	t.Setenv("HM_ENV", "")
	if got := Load().Env; got != "dev" {
		t.Fatalf("Env = %q, want dev", got)
	}
	t.Setenv("HM_ENV", "prod")
	if got := Load().Env; got != "prod" {
		t.Fatalf("Env = %q, want prod", got)
	}
}

func TestCloudEnvFallback(t *testing.T) {
	t.Setenv("HM_DB_DSN", "")
	t.Setenv("MYSQL_ADDRESS", "10.0.0.8:3306")
	t.Setenv("MYSQL_USERNAME", "hm")
	t.Setenv("MYSQL_PASSWORD", "p@ss:word")
	t.Setenv("MYSQL_DATABASE", "hm_lesson")
	t.Setenv("HM_DB_NAME", "")
	t.Setenv("COS_BUCKET", "hm-1250000000")
	t.Setenv("COS_REGION", "ap-shanghai")

	cfg := Load()
	parsed, err := mysqlDriver.ParseDSN(cfg.DBDSN)
	if err != nil {
		t.Fatalf("ParseDSN(%q): %v", cfg.DBDSN, err)
	}
	if parsed.User != "hm" || parsed.Passwd != "p@ss:word" {
		t.Fatalf("credentials = %q/%q", parsed.User, parsed.Passwd)
	}
	if parsed.Addr != "10.0.0.8:3306" || parsed.DBName != "hm_lesson" {
		t.Fatalf("addr/db = %q/%q", parsed.Addr, parsed.DBName)
	}
	if !parsed.ParseTime || parsed.Loc.String() != "UTC" {
		t.Fatalf("parseTime/loc = %v/%v", parsed.ParseTime, parsed.Loc)
	}
	if cfg.StorageDriver != "cloud" || cfg.StorageBucket != "hm-1250000000" || cfg.StorageRegion != "ap-shanghai" {
		t.Fatalf("storage = %q/%q/%q", cfg.StorageDriver, cfg.StorageBucket, cfg.StorageRegion)
	}
}

func TestHMDBNameWins(t *testing.T) {
	t.Setenv("HM_DB_DSN", "")
	t.Setenv("MYSQL_ADDRESS", "10.0.0.8:3306")
	t.Setenv("MYSQL_DATABASE", "golang_demo")
	t.Setenv("HM_DB_NAME", "hm_lesson")
	parsed, err := mysqlDriver.ParseDSN(Load().DBDSN)
	if err != nil {
		t.Fatalf("ParseDSN: %v", err)
	}
	if parsed.DBName != "hm_lesson" {
		t.Fatalf("DBName = %q, want hm_lesson", parsed.DBName)
	}
}

func TestExplicitDSNWins(t *testing.T) {
	t.Setenv("HM_DB_DSN", "user:pass@tcp(localhost:3307)/custom?parseTime=true")
	t.Setenv("MYSQL_ADDRESS", "10.0.0.8:3306")
	if got := Load().DBDSN; got != "user:pass@tcp(localhost:3307)/custom?parseTime=true" {
		t.Fatalf("DBDSN = %q", got)
	}
}

func TestNormalizeCallMode(t *testing.T) {
	cases := map[string]string{
		"":        "direct",
		"direct":  "direct",
		"cloud":   "cloud",
		" Cloud ": "cloud",
		"yun":     "cloud",
		"weird":   "direct",
	}
	for in, want := range cases {
		if got := normalizeCallMode(in); got != want {
			t.Fatalf("normalizeCallMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadCallModeFromEnv(t *testing.T) {
	t.Setenv("HM_WX_CALL_MODE", "cloud")
	if got := Load().WXCallMode; got != "cloud" {
		t.Fatalf("HM_WX_CALL_MODE=cloud 解析为 %q", got)
	}
	t.Setenv("HM_WX_CALL_MODE", "")
	if got := Load().WXCallMode; got != "direct" {
		t.Fatalf("未配置时应回退 direct，实际 %q", got)
	}
}

func TestParseSubscribeTemplates(t *testing.T) {
	tpls, err := ParseSubscribeTemplates("")
	if err != nil || tpls != nil {
		t.Fatalf("空配置应返回 nil/nil，得到 %v/%v", tpls, err)
	}

	raw := `{"task_published":{"id":"tmpl-1","fields":{"thing1":"title","time2":"due_at"}}}`
	tpls, err = ParseSubscribeTemplates(raw)
	if err != nil {
		t.Fatalf("ParseSubscribeTemplates: %v", err)
	}
	if got := tpls["task_published"]; got.ID != "tmpl-1" || got.Fields["thing1"] != "title" || got.Fields["time2"] != "due_at" {
		t.Fatalf("模板解析错误: %+v", got)
	}

	badCases := []string{
		`not-json`,
		`{"task_published":{"fields":{"thing1":"title"}}}`,
		`{"task_published":{"id":"tmpl-1"}}`,
	}
	for _, in := range badCases {
		if _, err := ParseSubscribeTemplates(in); err == nil {
			t.Fatalf("期望报错: %s", in)
		}
	}
}
