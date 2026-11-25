package biz

import (
	"context"
	"strings"
	"testing"
)

func TestRequireSystem(t *testing.T) {
	if err := RequireSystem(nil); err == nil {
		t.Fatal("nil 账号应当被拒绝")
	}
	if err := RequireSystem(&Staff{Role: RoleOwner}); err == nil {
		t.Fatal("普通 owner 不能使用运维接口")
	}
	if err := RequireSystem(&Staff{Role: RoleOwner, Hidden: true}); err != nil {
		t.Fatalf("系统账号应当通过：%v", err)
	}
}

type stubOpsRepo struct {
	OpsRepo
	lastSQL string
}

func (s *stubOpsRepo) QueryReadOnly(
	_ context.Context, statement string, _ int,
) ([]string, [][]string, bool, error) {
	s.lastSQL = statement
	return []string{"id"}, [][]string{{"1"}}, false, nil
}

func TestOpsQueryValidation(t *testing.T) {
	repo := &stubOpsRepo{}
	uc := NewOpsUsecase(repo)
	ctx := context.Background()

	cases := []struct {
		name string
		sql  string
		ok   bool
	}{
		{"空语句", "   ", false},
		{"多条语句", "SELECT 1; DROP TABLE staff", false},
		{"写操作", "UPDATE staff SET name = 'x'", false},
		{"删除", "DELETE FROM staff", false},
		{"select 正常", "SELECT id FROM staff LIMIT 1", true},
		{"select 小写", "select id from staff", true},
		{"show", "SHOW TABLES", true},
		{"describe", "DESCRIBE staff", true},
		{"explain", "EXPLAIN SELECT 1", true},
		{"末尾分号可接受", "SELECT 1;", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := uc.Query(ctx, tc.sql)
			if tc.ok {
				if err != nil {
					t.Fatalf("期望通过，实际报错：%v", err)
				}
				if result == nil || len(result.Columns) == 0 {
					t.Fatal("期望返回列信息")
				}
				return
			}
			if err == nil {
				t.Fatal("期望被拒绝，实际通过")
			}
		})
	}
}

func TestOpsQueryRejectsLongStatement(t *testing.T) {
	uc := NewOpsUsecase(&stubOpsRepo{})
	stmt := "SELECT '" + strings.Repeat("x", ConsoleMaxSQL) + "'"
	if _, err := uc.Query(context.Background(), stmt); err == nil {
		t.Fatal("超长语句应当被拒绝")
	}
}

func TestOpsPermissionsDeriveCapabilities(t *testing.T) {
	repo := &stubPermRepo{}
	uc := NewOpsUsecase(repo)
	items, err := uc.Permissions(context.Background())
	if err != nil {
		t.Fatalf("权限视图失败：%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 行，实际 %d", len(items))
	}
	owner, assistant := items[0], items[1]
	if !owner.CanManage || !owner.CanExport {
		t.Fatal("owner 应当可以管理成员与导出")
	}
	if assistant.CanManage || assistant.CanExport {
		t.Fatal("助理不应当可以管理成员或导出")
	}
	if len(assistant.Caveats) == 0 {
		t.Fatal("助理应当带权限说明")
	}
}

type stubPermRepo struct {
	OpsRepo
}

func (s *stubPermRepo) ListStaffPermissions(_ context.Context) ([]StaffPermission, error) {
	return []StaffPermission{
		{ID: "1", Phone: "188", Name: "老师", Role: RoleOwner, Status: StatusActive},
		{ID: "2", Phone: "199", Name: "助教", Role: RoleAssistant, Status: StatusActive},
	}, nil
}
