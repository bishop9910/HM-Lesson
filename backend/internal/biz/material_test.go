package biz

import (
	"context"
	"errors"
	"testing"
)

// stubMaterialRepo 只覆写用例用到的少量方法，其余调用会 panic。
type stubMaterialRepo struct {
	MaterialRepo

	created  *Material
	updated  *Material
	status   string
	deleted  string
	visSet   map[string]bool
	visGrace string
}

func (r *stubMaterialRepo) CreateMaterial(_ context.Context, m *Material) (*Material, error) {
	r.created = m
	return m, nil
}

func (r *stubMaterialRepo) UpdateMaterial(_ context.Context, m *Material) (*Material, error) {
	r.updated = m
	return m, nil
}

func (r *stubMaterialRepo) GetMaterial(_ context.Context, id string) (*Material, error) {
	return &Material{ID: id, Status: MaterialDraft}, nil
}

func (r *stubMaterialRepo) SetMaterialStatus(_ context.Context, id, status string) (*Material, error) {
	r.status = status
	return &Material{ID: id, Status: status}, nil
}

func (r *stubMaterialRepo) DeleteMaterial(_ context.Context, id string) error {
	r.deleted = id
	return nil
}

func (r *stubMaterialRepo) SetCategoryVisibility(_ context.Context, categoryID string, visibility map[string]bool) (*MaterialCategory, error) {
	r.visSet = visibility
	r.visGrace = categoryID
	return &MaterialCategory{ID: categoryID, Visibility: visibility}, nil
}

func TestMaterialInputNormalize(t *testing.T) {
	validFiles := []MaterialFile{{Name: "讲义", FileID: "cloud://a.pdf"}}
	cases := []struct {
		name  string
		input MaterialInput
		want  error
	}{
		{
			name:  "正常资料",
			input: MaterialInput{CategoryID: "c1", Title: "第一讲", Grade: "g1", Files: validFiles},
			want:  nil,
		},
		{
			name:  "缺分类",
			input: MaterialInput{Title: "第一讲"},
			want:  ErrInvalidMaterial,
		},
		{
			name:  "标题为空",
			input: MaterialInput{CategoryID: "c1", Title: "   "},
			want:  ErrInvalidMaterial,
		},
		{
			name:  "年级非法",
			input: MaterialInput{CategoryID: "c1", Title: "第一讲", Grade: "g9"},
			want:  ErrInvalidMaterial,
		},
		{
			name:  "文件超过 9 个",
			input: MaterialInput{CategoryID: "c1", Title: "第一讲", Files: make([]MaterialFile, 10)},
			want:  ErrInvalidMaterial,
		},
		{
			name:  "附件缺 fileID",
			input: MaterialInput{CategoryID: "c1", Title: "第一讲", Files: []MaterialFile{{Name: "空"}}},
			want:  ErrInvalidMaterial,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.input
			if err := input.normalize(); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestCreateMaterialStartsAsDraft(t *testing.T) {
	repo := &stubMaterialRepo{}
	uc := NewMaterialUsecase(repo)
	actor := &Staff{ID: "staff-1", Role: RoleOwner}
	material, err := uc.CreateMaterial(context.Background(), actor, MaterialInput{
		CategoryID: "c1", Title: "  第一讲  ",
	})
	if err != nil {
		t.Fatalf("CreateMaterial: %v", err)
	}
	if material.Status != MaterialDraft {
		t.Fatalf("want draft, got %q", material.Status)
	}
	if material.Title != "第一讲" {
		t.Fatalf("title not trimmed: %q", material.Title)
	}
	if material.CreatedBy != "staff-1" {
		t.Fatalf("want created_by staff-1, got %q", material.CreatedBy)
	}
	if material.ID == "" {
		t.Fatal("id not generated")
	}
}

func TestSetCategoryVisibilityValidatesGroups(t *testing.T) {
	repo := &stubMaterialRepo{}
	uc := NewMaterialUsecase(repo)
	owner := &Staff{ID: "staff-1", Role: RoleOwner}
	assistant := &Staff{ID: "staff-2", Role: RoleAssistant}

	if _, err := uc.SetCategoryVisibility(context.Background(), assistant, "c1", map[string]bool{GroupIntent: true}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("assistant should be forbidden, got %v", err)
	}
	if _, err := uc.SetCategoryVisibility(context.Background(), owner, "c1", map[string]bool{"unknown": true}); !errors.Is(err, ErrInvalidMaterial) {
		t.Fatalf("unknown group should be rejected, got %v", err)
	}

	category, err := uc.SetCategoryVisibility(context.Background(), owner, "c1", map[string]bool{GroupIntent: true})
	if err != nil {
		t.Fatalf("SetCategoryVisibility: %v", err)
	}
	// 未提供的分组一律按 false 补齐，避免矩阵出现空洞。
	for _, group := range MaterialGroups {
		want := group == GroupIntent
		if category.Visibility[group] != want {
			t.Fatalf("group %s: want %v, got %v", group, want, category.Visibility[group])
		}
	}
}

func TestArchiveRequiresPublished(t *testing.T) {
	repo := &stubMaterialRepo{}
	uc := NewMaterialUsecase(repo)
	actor := &Staff{ID: "staff-1", Role: RoleOwner}
	// stub 的 GetMaterial 固定返回 draft。
	if _, err := uc.Archive(context.Background(), actor, "m1"); !errors.Is(err, ErrMaterialState) {
		t.Fatalf("archive draft should conflict, got %v", err)
	}
}

func TestDeleteMaterialOwnerOnly(t *testing.T) {
	repo := &stubMaterialRepo{}
	uc := NewMaterialUsecase(repo)
	if err := uc.DeleteMaterial(context.Background(), &Staff{ID: "s2", Role: RoleAssistant}, "m1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("assistant should be forbidden, got %v", err)
	}
	if err := uc.DeleteMaterial(context.Background(), &Staff{ID: "s1", Role: RoleOwner}, "m1"); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if repo.deleted != "m1" {
		t.Fatalf("repo not called, got %q", repo.deleted)
	}
}

func TestDefaultCategoriesCoverBlueBook(t *testing.T) {
	categories := DefaultCategories()
	if len(categories) != 6 {
		t.Fatalf("want 6 categories, got %d", len(categories))
	}
	byCode := make(map[string]MaterialCategory, len(categories))
	for _, c := range categories {
		byCode[c.Code] = c
	}
	// 蓝本表 7 的关键约束：班课讲义只给班课在读，一对一只给一对一在读。
	if !byCode[CategoryClassHandout].Visibility[GroupClassActive] || byCode[CategoryClassHandout].Visibility[GroupOneOnOne] {
		t.Fatal("class_handout matrix mismatch")
	}
	if !byCode[CategoryOneOnOne].Visibility[GroupOneOnOne] || byCode[CategoryOneOnOne].Visibility[GroupClassActive] {
		t.Fatal("one_on_one matrix mismatch")
	}
	for _, group := range MaterialGroups {
		if !byCode[CategoryTrial].Visibility[group] || !byCode[CategoryEnrollment].Visibility[group] {
			t.Fatalf("trial/enrollment should be visible to %s", group)
		}
		if byCode[CategoryRecording].Visibility[group] {
			t.Fatalf("recording should be hidden from %s", group)
		}
	}
}
