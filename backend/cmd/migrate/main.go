// Command migrate 手工执行数据库迁移；服务启动时也会自动执行 up。
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/bishop9910/hm-lesson/backend/internal/conf"
	"github.com/bishop9910/hm-lesson/backend/internal/data"
)

func main() {
	down := flag.Int("down", 0, "回滚最近 N 个版本；0 表示不执行回滚")
	printVersion := flag.Bool("version", false, "只打印当前数据库迁移版本")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := conf.Load()
	db, err := data.Open(cfg.DBDSN)
	if err != nil {
		logger.Error("打开数据库失败", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()

	if *printVersion {
		v, err := data.Version(ctx, db)
		if err != nil {
			logger.Error("读取迁移版本失败", "error", err)
			os.Exit(1)
		}
		logger.Info("当前迁移版本", "version", v)
		return
	}

	if *down > 0 {
		applied, err := data.MigrateDown(ctx, db, *down)
		if err != nil {
			logger.Error("回滚迁移失败", "error", err)
			os.Exit(1)
		}
		logger.Info("回滚完成", "versions", applied)
		return
	}

	applied, err := data.Migrate(ctx, db)
	if err != nil {
		logger.Error("执行迁移失败", "error", err)
		os.Exit(1)
	}
	logger.Info("迁移完成", "versions", applied)
}
