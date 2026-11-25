package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
	"github.com/bishop9910/hm-lesson/backend/internal/conf"
	"github.com/bishop9910/hm-lesson/backend/internal/data"
	"github.com/bishop9910/hm-lesson/backend/internal/service"
	"github.com/bishop9910/hm-lesson/backend/internal/version"
	"github.com/bishop9910/hm-lesson/backend/pkg/password"
	"github.com/bishop9910/hm-lesson/backend/pkg/token"
	"github.com/bishop9910/hm-lesson/backend/pkg/wx"
)

func main() {
	cfg := conf.Load()
	logger := newLogger(cfg.LogLevel())
	slog.SetDefault(logger)

	authSrv, subs, db := buildServices(cfg, logger)

	srv := khttp.NewServer(
		khttp.Address(cfg.HTTPAddr),
		khttp.Timeout(15*time.Second),
		khttp.Middleware(recovery.Recovery()),
		khttp.ErrorEncoder(func(w http.ResponseWriter, r *http.Request, err error) {
			if w.Header().Get("X-HM-Handled") != "" {
				return
			}
			khttp.DefaultErrorEncoder(w, r, err)
		}),
	)
	router := srv.Route("/")
	router.GET("/healthz", func(ctx khttp.Context) error {
		return ctx.JSON(200, map[string]any{"status": "ok", "version": version.Version})
	})
	router.GET("/readyz", func(ctx khttp.Context) error {
		if db != nil {
			sqlDB, err := db.DB()
			if err != nil || sqlDB.PingContext(ctx) != nil {
				return ctx.JSON(503, map[string]any{"status": "not_ready"})
			}
		}
		return ctx.JSON(200, map[string]any{"status": "ready"})
	})

	// 多老师账号：登录公开；成员管理仅 owner。
	router.POST("/api/v1/auth/login", authSrv.Login)
	router.POST("/api/v1/auth/staff/login", authSrv.Login)
	router.POST("/api/v1/auth/bind-request", authSrv.RequestBind)
	router.GET("/api/v1/auth/me", authSrv.Me)
	router.GET("/api/v1/staff", authSrv.ListStaff)
	router.POST("/api/v1/staff", authSrv.CreateStaff)
	router.PATCH("/api/v1/staff/{id}", authSrv.UpdateStaff)
	router.GET("/api/v1/dashboard/summary", authSrv.DashboardSummary)

	// M2：客户档案、跟进、家长审批/绑定、家长门户。
	router.GET("/api/v1/students", authSrv.ListStudents)
	router.POST("/api/v1/students", authSrv.CreateStudent)
	router.GET("/api/v1/students/{id}", authSrv.GetStudent)
	router.PATCH("/api/v1/students/{id}", authSrv.UpdateStudent)
	router.GET("/api/v1/students/{id}/follow-ups", authSrv.ListFollowUps)
	router.POST("/api/v1/students/{id}/follow-ups", authSrv.CreateFollowUp)
	router.POST("/api/v1/students/{id}/binding-codes", authSrv.CreateBindingCode)
	router.GET("/api/v1/guardians/pending", authSrv.PendingGuardians)
	router.POST("/api/v1/guardians/{id}/approve", authSrv.ApproveGuardian)
	router.POST("/api/v1/guardians/{id}/reject", authSrv.RejectGuardian)
	router.POST("/api/v1/auth/guardian/login", authSrv.GuardianLogin)
	router.GET("/api/v1/portal/home", authSrv.PortalHome)
	router.GET("/api/v1/portal/notifications", authSrv.PortalNotifications)
	router.POST("/api/v1/portal/notifications/read-all", authSrv.ReadAllNotifications)
	router.POST("/api/v1/portal/notifications/{id}/read", authSrv.ReadNotification)
	router.GET("/api/v1/portal/students/{id}", authSrv.PortalStudent)

	// M2：Excel 学员导入。
	router.GET("/api/v1/imports/students/template", authSrv.ImportTemplate)
	router.POST("/api/v1/imports/students/preview", authSrv.PreviewStudentsImport)
	router.GET("/api/v1/imports/{id}", authSrv.GetImportBatch)
	router.POST("/api/v1/imports/{id}/commit", authSrv.CommitStudentsImport)
	router.POST("/api/v1/imports/{id}/rollback", authSrv.RollbackImportBatch)

	// M3：任务下发、提交、批阅与作业档案。
	router.GET("/api/v1/tasks", authSrv.ListTasks)
	router.POST("/api/v1/tasks", authSrv.CreateTask)
	router.GET("/api/v1/tasks/{id}", authSrv.GetTask)
	router.POST("/api/v1/tasks/{id}/publish", authSrv.PublishTask)
	router.POST("/api/v1/tasks/{id}/close", authSrv.CloseTask)
	router.DELETE("/api/v1/tasks/{id}", authSrv.DeleteTask)
	router.GET("/api/v1/reviews", authSrv.ReviewQueue)
	router.GET("/api/v1/task-templates", authSrv.ListTemplates)
	router.POST("/api/v1/task-templates", authSrv.CreateTemplate)
	router.PATCH("/api/v1/task-templates/{id}", authSrv.UpdateTemplate)
	router.DELETE("/api/v1/task-templates/{id}", authSrv.DeleteTemplate)
	router.POST("/api/v1/assignments/{id}/review", authSrv.ReviewAssignment)
	router.GET("/api/v1/portal/tasks", authSrv.PortalTasks)
	router.GET("/api/v1/portal/tasks/{id}", authSrv.PortalTask)
	router.POST("/api/v1/portal/tasks/{id}/submit", authSrv.PortalSubmit)
	router.GET("/api/v1/portal/archive", authSrv.PortalArchive)

	// M3 增强：微信订阅消息（一次性订阅授权 + 进程内投递）。
	router.GET("/api/v1/portal/subscriptions", authSrv.PortalSubscriptions)
	router.POST("/api/v1/portal/subscriptions", authSrv.GrantSubscriptions)

	// M4 一对一预约排课：时段与预约（教师端）。
	router.GET("/api/v1/slots", authSrv.ListSlots)
	router.POST("/api/v1/slots", authSrv.CreateSlot)
	router.POST("/api/v1/slots/batch", authSrv.BatchSlots)
	router.PATCH("/api/v1/slots/{id}", authSrv.UpdateSlot)
	router.DELETE("/api/v1/slots/{id}", authSrv.CancelSlot)
	router.GET("/api/v1/bookings", authSrv.ListBookings)
	router.POST("/api/v1/bookings/{id}/approve", authSrv.ApproveBooking)
	router.POST("/api/v1/bookings/{id}/reject", authSrv.RejectBooking)
	router.POST("/api/v1/bookings/{id}/complete", authSrv.CompleteBooking)
	router.POST("/api/v1/bookings/{id}/no-show", authSrv.NoShowBooking)
	router.GET("/api/v1/calendar/bookings", authSrv.CalendarBookings)

	// M4 家长端预约。
	router.GET("/api/v1/portal/slots", authSrv.PortalSlots)
	router.GET("/api/v1/portal/bookings", authSrv.PortalBookings)
	router.POST("/api/v1/portal/bookings", authSrv.PortalCreateBooking)
	router.POST("/api/v1/portal/bookings/{id}/cancel", authSrv.PortalCancelBooking)

	// M4 课时台账。
	router.GET("/api/v1/lesson-accounts", authSrv.ListLessonAccounts)
	router.GET("/api/v1/lesson-accounts/{studentID}", authSrv.GetLessonAccount)
	router.POST("/api/v1/lesson-accounts/{studentID}/purchase", authSrv.PurchaseLessonHours)
	router.POST("/api/v1/lesson-accounts/{studentID}/adjust", authSrv.AdjustLessonHours)
	router.GET("/api/v1/lesson-ledger", authSrv.ListLessonLedger)
	router.GET("/api/v1/portal/lesson-account", authSrv.PortalLessonAccount)

	app := kratos.New(
		kratos.Name("hm-api"),
		kratos.Version(version.Version),
		kratos.Logger(logger),
		kratos.Server(srv),
		// 订阅消息 worker 跟随应用生命周期：AfterStart 起 goroutine，Stop 取消 ctx 后退出。
		kratos.AfterStart(func(ctx context.Context) error {
			if subs != nil {
				go subs.RunWorker(ctx)
			}
			return nil
		}),
	)
	if err := app.Run(); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func buildServices(cfg conf.Config, logger *slog.Logger) (*service.Server, *biz.SubscriptionUsecase, *gorm.DB) {
	if cfg.DBDSN == "" {
		logger.Warn("HM_DB_DSN 未配置，仅健康检查可用；登录/成员接口返回 503")
		return service.New(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil), nil, nil
	}
	db, err := data.Open(cfg.DBDSN)
	if err != nil {
		logger.Error("打开数据库失败", "error", err)
		os.Exit(1)
	}
	if applied, err := data.Migrate(context.Background(), db); err != nil {
		logger.Error("执行数据库迁移失败", "error", err)
		os.Exit(1)
	} else if len(applied) > 0 {
		logger.Info("数据库迁移完成", "versions", applied)
	}
	issuer, err := token.NewIssuer(cfg.JWTSecret, 2*time.Hour)
	if err != nil {
		logger.Error("JWT 配置无效", "error", err)
		os.Exit(1)
	}
	uc := biz.NewAuthUsecase(data.NewStaffRepo(db), password.Hasher{}, issuer)
	_, seeded, err := uc.Bootstrap(context.Background(), cfg.BootstrapName, cfg.BootstrapPhone, cfg.BootstrapPassword)
	if err != nil {
		logger.Error("创建初始负责人失败", "error", err)
		os.Exit(1)
	}
	if seeded {
		logger.Info("已创建初始负责人", "phone", cfg.BootstrapPhone)
	} else if cfg.BootstrapPhone == "" {
		logger.Warn("库中没有老师账号，且未配置 HM_BOOTSTRAP_PHONE/HM_BOOTSTRAP_PASSWORD；请配置后重启")
	}
	dashboard := biz.NewDashboardUsecase(data.NewDashboardRepo(db))
	bind := biz.NewBindUsecase(data.NewGuardianRepo(db))
	crm := biz.NewCRMUsecase(data.NewCRMRepo(db))
	importUC := biz.NewImportUsecase(data.NewImportRepo(db))
	tasks := biz.NewTaskUsecase(data.NewTaskRepo(db))
	subs := buildSubscriptions(cfg, db, logger)
	bookings := biz.NewBookingUsecase(data.NewBookingRepo(db), cfg.LessonHoursPerSession)
	lessons := biz.NewLessonUsecase(data.NewLessonRepo(db))
	return service.New(uc, dashboard, bind, crm, importUC, tasks, subs, bookings, lessons, issuer), subs, db
}

// buildSubscriptions 组装一次性订阅消息用例。
// 模板来自 HM_WX_SUBSCRIBE_TEMPLATES；缺 AppID/AppSecret 或模板时 sender 为 nil，整条链路 no-op。
func buildSubscriptions(cfg conf.Config, db *gorm.DB, logger *slog.Logger) *biz.SubscriptionUsecase {
	parsed, err := conf.ParseSubscribeTemplates(cfg.WXSubscribeTemplates)
	if err != nil {
		logger.Warn("订阅消息模板配置无效，微信订阅消息不会发送", "error", err)
		parsed = nil
	}
	templates := make(map[string]biz.SubscribeTemplate, len(parsed))
	for key, tpl := range parsed {
		if !biz.IsTemplateKey(key) {
			logger.Warn("忽略未知订阅消息模板键", "template_key", key)
			continue
		}
		templates[key] = biz.SubscribeTemplate{ID: tpl.ID, Fields: tpl.Fields}
	}
	var sender biz.SubscribeSender
	if cfg.WXAppID != "" && cfg.WXAppSecret != "" && len(templates) > 0 {
		sender = wxSubscribeSender{client: wx.NewClient(cfg.WXAppID, cfg.WXAppSecret)}
	} else {
		logger.Info("微信订阅消息未启用：需要 HM_WX_APP_ID、HM_WX_APP_SECRET 与 HM_WX_SUBSCRIBE_TEMPLATES；家长仍可在小程序内看到站内通知")
	}
	return biz.NewSubscriptionUsecase(data.NewSubscriptionRepo(db), sender, templates, cfg.WXMiniprogramState, logger)
}

// wxSubscribeSender 把 pkg/wx 适配成 biz.SubscribeSender；retryable=false 表示重试无意义。
type wxSubscribeSender struct{ client *wx.Client }

func (s wxSubscribeSender) SendSubscribe(ctx context.Context, msg biz.SubscribeMessage) (bool, error) {
	err := s.client.SendSubscribe(ctx, wx.SubscribeMessage{
		ToUser:           msg.OpenID,
		TemplateID:       msg.TemplateID,
		Page:             msg.Page,
		MiniprogramState: msg.MiniprogramState,
		Data:             msg.Data,
	})
	if err == nil {
		return false, nil
	}
	return !wx.IsPermanent(err), err
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
