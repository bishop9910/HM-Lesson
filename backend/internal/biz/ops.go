package biz

import (
	"context"
	"errors"
	"strings"
)

// 内部系统账号（staff.hidden）专用的运维能力：操作记录、权限视图、只读查询控制台。
//
// 这些能力只对内网调试与排障使用，不面向业务角色开放。

// ErrSystemOnly 表示当前账号不是内部系统账号。
var ErrSystemOnly = errors.New("只有内部系统账号可以使用该功能")

// ErrInvalidQuery 表示控制台语句不在允许范围内。
var ErrInvalidQuery = errors.New("只允许执行只读查询（SELECT / SHOW / DESCRIBE / EXPLAIN），且单条语句")

// ErrQueryTooLong 表示控制台语句过长。
var ErrQueryTooLong = errors.New("语句过长，请精简到 4000 字符以内")

// 控制台硬限制：单次最多返回 200 行、最长执行 8 秒、语句 4000 字符。
const (
	ConsoleMaxRows    = 200
	ConsoleMaxSQL     = 4000
	ConsoleMaxPerPage = 200
)

// AuditEntry 是一条操作记录（audit_logs）。
type AuditEntry struct {
	ID            string
	ActorType     string
	ActorID       string
	ActorName     string
	Action        string
	TargetType    string
	TargetID      string
	BeforeSummary string
	AfterSummary  string
	CreatedAt     string
}

// StaffPermission 是权限视图里的一行：账号 + 角色 + 能力摘要。
type StaffPermission struct {
	ID        string
	Phone     string
	Name      string
	Role      string
	Status    string
	Hidden    bool
	CanManage bool
	CanReview bool
	CanExport bool
	Caveats   []string
}

// ConsoleResult 是只读查询的结果集。
type ConsoleResult struct {
	Columns      []string
	Rows         [][]string
	ElapsedMilli int64
	Truncated    bool
}

// OpsRepo 是运维控制台的数据访问接口。
type OpsRepo interface {
	ListAudit(ctx context.Context, action string, limit int) ([]AuditEntry, error)
	ListStaffPermissions(ctx context.Context) ([]StaffPermission, error)
	QueryReadOnly(ctx context.Context, sql string, maxRows int) ([]string, [][]string, bool, error)
	RecordAudit(ctx context.Context, entry AuditEntry) error
}

// OpsUsecase 编排运维控制台能力。
type OpsUsecase struct {
	repo OpsRepo
}

// NewOpsUsecase 构造运维用例。
func NewOpsUsecase(repo OpsRepo) *OpsUsecase {
	return &OpsUsecase{repo: repo}
}

// RequireSystem 校验调用者是内部系统账号。
func RequireSystem(actor *Staff) error {
	if actor == nil || !actor.Hidden {
		return ErrSystemOnly
	}
	return nil
}

// Audit 返回最近的审计记录，可按 action 过滤。
func (uc *OpsUsecase) Audit(ctx context.Context, action string, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > ConsoleMaxPerPage {
		limit = 50
	}
	return uc.repo.ListAudit(ctx, strings.TrimSpace(action), limit)
}

// Permissions 返回当前成员与角色的能力摘要，用于权限视图核对。
func (uc *OpsUsecase) Permissions(ctx context.Context) ([]StaffPermission, error) {
	items, err := uc.repo.ListStaffPermissions(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].CanManage = items[i].Role == RoleOwner
		items[i].CanReview = true
		// 导出与跟进、收费、课时调整只对 owner 开放（见 AGENTS §7）。
		items[i].CanExport = items[i].Role == RoleOwner
		if items[i].Hidden {
			items[i].Caveats = append(items[i].Caveats, "内部系统账号：不在成员管理中展示")
		}
		if items[i].Role == RoleAssistant {
			items[i].Caveats = append(items[i].Caveats, "不能访问跟进、收费、课时调整、导出、成员管理")
		}
	}
	return items, nil
}

// Record 写一条操作记录；actor 为空或 ops 未装配时静默跳过，不影响主流程。
func (uc *OpsUsecase) Record(
	ctx context.Context, actor *Staff, action, targetType, targetID, afterSummary string,
) {
	if uc == nil || uc.repo == nil || actor == nil {
		return
	}
	_ = uc.repo.RecordAudit(ctx, AuditEntry{
		ID:           NewID(),
		ActorType:    "staff",
		ActorID:      actor.ID,
		Action:       action,
		TargetType:   targetType,
		TargetID:     targetID,
		AfterSummary: afterSummary,
	})
}

// Query 执行只读查询；语句先做白名单与单语句校验。
func (uc *OpsUsecase) Query(ctx context.Context, raw string) (*ConsoleResult, error) {
	stmt := strings.TrimSpace(raw)
	stmt = strings.TrimSuffix(stmt, ";")
	stmt = strings.TrimSpace(stmt)
	if stmt == "" {
		return nil, ErrInvalidQuery
	}
	if len([]rune(stmt)) > ConsoleMaxSQL {
		return nil, ErrQueryTooLong
	}
	if strings.Contains(stmt, ";") {
		return nil, ErrInvalidQuery
	}
	upper := strings.ToUpper(stmt)
	allowed := false
	for _, prefix := range []string{"SELECT ", "SELECT\n", "SHOW ", "DESCRIBE ", "DESC ", "EXPLAIN "} {
		if strings.HasPrefix(upper, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrInvalidQuery
	}
	columns, rows, truncated, err := uc.repo.QueryReadOnly(ctx, stmt, ConsoleMaxRows)
	if err != nil {
		return nil, err
	}
	return &ConsoleResult{Columns: columns, Rows: rows, Truncated: truncated}, nil
}
