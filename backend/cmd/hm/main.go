package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
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
	router.DELETE("/api/v1/students/{id}", authSrv.DeleteStudent)
	router.POST("/api/v1/students/batch-delete", authSrv.BatchDeleteStudents)
	router.GET("/api/v1/students/{id}/follow-ups", authSrv.ListFollowUps)
	router.POST("/api/v1/students/{id}/follow-ups", authSrv.CreateFollowUp)
	router.POST("/api/v1/students/{id}/binding-codes", authSrv.CreateBindingCode)
	router.GET("/api/v1/guardians/pending", authSrv.PendingGuardians)
	router.GET("/api/v1/guardians", authSrv.ListGuardians)
	router.POST("/api/v1/guardians/{id}/approve", authSrv.ApproveGuardian)
	router.POST("/api/v1/guardians/{id}/reject", authSrv.RejectGuardian)
	router.POST("/api/v1/guardians/{id}/password", authSrv.ResetGuardianPassword)
	router.DELETE("/api/v1/guardians/{id}", authSrv.DeleteGuardian)
	router.POST("/api/v1/guardians/batch-delete", authSrv.BatchDeleteGuardians)
	router.POST("/api/v1/auth/password", authSrv.ChangePassword)
	router.POST("/api/v1/auth/guardian/password", authSrv.ChangeGuardianPassword)
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

	// M5 资料库：分类矩阵 + 资料 + 下载记录（教师端）。
	router.GET("/api/v1/material-categories", authSrv.ListMaterialCategories)
	router.POST("/api/v1/material-categories", authSrv.CreateMaterialCategory)
	router.PATCH("/api/v1/material-categories/{id}", authSrv.UpdateMaterialCategory)
	router.PUT("/api/v1/material-categories/{id}/visibility", authSrv.SetMaterialCategoryVisibility)
	router.GET("/api/v1/materials", authSrv.ListMaterials)
	router.POST("/api/v1/materials", authSrv.CreateMaterial)
	router.GET("/api/v1/materials/{id}", authSrv.GetMaterial)
	router.PATCH("/api/v1/materials/{id}", authSrv.UpdateMaterial)
	router.POST("/api/v1/materials/{id}/publish", authSrv.PublishMaterial)
	router.POST("/api/v1/materials/{id}/archive", authSrv.ArchiveMaterial)
	router.DELETE("/api/v1/materials/{id}", authSrv.DeleteMaterial)
	router.GET("/api/v1/material-downloads", authSrv.ListMaterialDownloads)

	// M5 家长端资料。
	router.GET("/api/v1/portal/materials", authSrv.PortalMaterials)
	router.POST("/api/v1/portal/materials/{id}/download", authSrv.PortalDownloadMaterial)

	// 内部系统账号（system）专用的运维接口：操作记录、权限视图、只读控制台。
	router.GET("/api/v1/system/audit", authSrv.SystemAudit)
	router.GET("/api/v1/system/permissions", authSrv.SystemPermissions)
	router.POST("/api/v1/system/query", authSrv.SystemQuery)

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
		return service.New(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil), nil, nil
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
	// 内部系统账号（用户名 system）：只在创建时或密码为空时从环境变量取初始密码。
	system, sysCreated, serr := uc.EnsureSystem(context.Background(), cfg.SystemPassword)
	switch {
	case serr != nil:
		logger.Warn("系统账号维护失败，不影响业务", "error", serr)
	case sysCreated:
		logger.Info("已创建内部系统账号", "phone", biz.SystemPhone, "hidden", true)
	case system == nil:
		logger.Warn("未配置 HM_SYSTEM_PASSWORD，且系统账号尚未创建；配置后重启即可创建")
	case system.PasswordHash == "":
		logger.Warn("内部系统账号还没有密码，请配置 HM_SYSTEM_PASSWORD 后重启", "phone", biz.SystemPhone)
	default:
		logger.Info("内部系统账号已存在", "phone", biz.SystemPhone, "hidden", true)
	}
	dashboard := biz.NewDashboardUsecase(data.NewDashboardRepo(db))
	bind := biz.NewBindUsecase(data.NewGuardianRepo(db))
	crm := biz.NewCRMUsecase(data.NewCRMRepo(db), password.Hasher{})
	importUC := biz.NewImportUsecase(data.NewImportRepo(db))
	tasks := biz.NewTaskUsecase(data.NewTaskRepo(db))
	subs := buildSubscriptions(cfg, db, logger)
	bookings := biz.NewBookingUsecase(data.NewBookingRepo(db), cfg.LessonHoursPerSession)
	lessons := biz.NewLessonUsecase(data.NewLessonRepo(db))
	materials := biz.NewMaterialUsecase(data.NewMaterialRepo(db))
	// 资料分类与分组矩阵按蓝本表 7 幂等补齐（owner 改过的不覆盖）。
	if created, merr := materials.EnsureDefaults(context.Background()); merr != nil {
		logger.Warn("资料分类初始化失败，不影响其它功能", "error", merr)
	} else if created > 0 {
		logger.Info("已初始化资料分类", "created", created)
	}
	return service.New(uc, dashboard, bind, crm, importUC, tasks, subs, bookings, lessons, materials,
		biz.NewOpsUsecase(data.NewOpsRepo(db)), issuer), subs, db
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
	cloudMode := cfg.WXCallMode == "cloud"
	if cfg.WXAppID != "" && len(templates) > 0 && (cloudMode || cfg.WXAppSecret != "") {
		var client *wx.Client
		if cloudMode {
			// 云调用：平台网关鉴权，不需要 AppSecret，也不取 access_token。
			client = wx.NewCloudClient(cfg.WXAppID)
		} else {
			client = wx.NewClient(cfg.WXAppID, cfg.WXAppSecret)
		}
		sender = wxSubscribeSender{client: client}
		go probeEgress(logger)
		if cloudMode {
			logger.Info("微信订阅消息使用云调用通道（HM_WX_CALL_MODE=cloud），跳过 access_token 探测",
				"app_id", cfg.WXAppID, "templates", len(templates))
		} else {
			go probeWXToken(client, cfg.WXAppID, logger)
		}
	} else {
		logger.Info("微信订阅消息未启用：需要 HM_WX_APP_ID、HM_WX_SUBSCRIBE_TEMPLATES 与（直连模式下的）HM_WX_APP_SECRET；家长仍可在小程序内看到站内通知")
	}
	return biz.NewSubscriptionUsecase(data.NewSubscriptionRepo(db), sender, templates, cfg.WXMiniprogramState, logger)
}

// probeWXToken 启动时验证 AppID/AppSecret 能否换到 access_token，让配置错误在日志里暴露，
// 而不是等到第一次真实发通知才 500/40013。失败只记日志：订阅消息是增强能力，不阻塞启动。
func probeWXToken(client *wx.Client, appID string, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.AccessToken(ctx); err != nil {
		logger.Warn("微信 access_token 获取失败，订阅消息将无法发送",
			"app_id", appID, "error", err.Error(), "hint", wxFailureHint(err))
		return
	}
	logger.Info("微信 access_token 获取成功，订阅消息可发送", "app_id", appID)
}

// wxFailureHint 给运维一句可执行的方向，区分“密钥不对”和“网络出不去”。
func wxFailureHint(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return "容器连不上 api.weixin.qq.com：确认云托管已开启公网出口（出站），或检查 DNS/代理"
	}
	var apiErr *wx.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case 40013:
			return "AppID 不正确（HM_WX_APP_ID）"
		case 40125, 40001:
			return "AppSecret 不正确（HM_WX_APP_SECRET）；若填的是占位文案需换成微信后台真实密钥"
		case 40164:
			return "调用 IP 不在白名单：微信后台 → 开发管理 → 开发设置 → IP 白名单"
		}
	}
	return "查看上面的 error 与微信错误码说明"
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
