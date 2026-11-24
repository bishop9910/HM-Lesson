package biz

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeTaskRepo struct {
	task        *Task
	assignments []TaskAssignment
	created     *Task
	published   int
	publishIDs  []string
	reviewArgs  []any
	submitArgs  []any
}

func (f *fakeTaskRepo) ListTasks(context.Context, string, string, int, int) ([]Task, int, error) {
	return nil, 0, nil
}

func (f *fakeTaskRepo) CreateTask(_ context.Context, t *Task) error {
	f.created = t
	f.task = t
	return nil
}

func (f *fakeTaskRepo) GetTask(context.Context, string) (*Task, error) {
	if f.task == nil {
		return nil, ErrTaskNotFound
	}
	return f.task, nil
}

func (f *fakeTaskRepo) SetTaskStatus(context.Context, string, string, string) error { return nil }

func (f *fakeTaskRepo) DeleteTask(context.Context, string) error { return nil }

func (f *fakeTaskRepo) PublishTask(_ context.Context, _ string, ids []string) (int, error) {
	f.publishIDs = ids
	f.published = len(ids)
	return len(ids), nil
}

func (f *fakeTaskRepo) ListAssignments(context.Context, string) ([]TaskAssignment, error) {
	return f.assignments, nil
}

func (f *fakeTaskRepo) ReviewQueue(context.Context, int) ([]TaskAssignment, error) {
	return nil, nil
}

func (f *fakeTaskRepo) ReviewAssignment(_ context.Context, id, reviewer string, score *float64, comment, reason string) error {
	f.reviewArgs = []any{id, reviewer, score, comment, reason}
	return nil
}

func (f *fakeTaskRepo) PortalTasks(context.Context, string) ([]TaskAssignment, error) {
	return nil, nil
}

func (f *fakeTaskRepo) PortalTask(context.Context, string, string) (*TaskAssignment, error) {
	return nil, nil
}

func (f *fakeTaskRepo) PortalArchive(context.Context, string) ([]TaskAssignment, error) {
	return nil, nil
}

func (f *fakeTaskRepo) Submit(_ context.Context, guardian, assignment, text string, ids []string) (*Submission, error) {
	f.submitArgs = []any{guardian, assignment, text, ids}
	return &Submission{ID: "s1", Version: 1, Text: text, FileIDs: ids, SubmittedAt: time.Now()}, nil
}

func (f *fakeTaskRepo) ListTemplates(context.Context) ([]TaskTemplate, error) { return nil, nil }

func (f *fakeTaskRepo) CreateTemplate(context.Context, *TaskTemplate) error { return nil }

func (f *fakeTaskRepo) UpdateTemplate(context.Context, *TaskTemplate) error { return nil }

func (f *fakeTaskRepo) DeleteTemplate(context.Context, string) error { return nil }

func future() *time.Time {
	t := time.Now().Add(time.Hour)
	return &t
}

func past() *time.Time {
	t := time.Now().Add(-time.Hour)
	return &t
}

func TestTaskCreateValidation(t *testing.T) {
	cases := []struct {
		name  string
		title string
		typ   string
		due   *time.Time
		want  error
	}{
		{"empty title", "", "homework", future(), ErrInvalidTask},
		{"bad type", "作业", "unknown", future(), ErrInvalidTask},
		{"no due", "作业", "homework", nil, ErrInvalidTask},
		{"ok", "作业", "homework", future(), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeTaskRepo{}
			_, err := NewTaskUsecase(repo).Create(context.Background(), "staff-1", tc.title, "", tc.typ, tc.due)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
			if tc.want == nil && (repo.created == nil || repo.created.Status != TaskDraft) {
				t.Fatal("expected draft task")
			}
		})
	}
}

func TestDeleteDraftOrClosed(t *testing.T) {
	repo := &fakeTaskRepo{task: &Task{ID: "t1", Status: TaskDraft}}
	uc := NewTaskUsecase(repo)
	if err := uc.Delete(context.Background(), "t1"); err != nil {
		t.Fatalf("draft delete: %v", err)
	}
	repo.task = &Task{ID: "t2", Status: TaskPublished}
	if err := uc.Delete(context.Background(), "t2"); !errors.Is(err, ErrTaskState) {
		t.Fatalf("published delete want state error, got %v", err)
	}
	repo.task = &Task{ID: "t3", Status: TaskClosed}
	if err := uc.Delete(context.Background(), "t3"); err != nil {
		t.Fatalf("closed delete: %v", err)
	}
}

func TestTaskPublish(t *testing.T) {
	repo := &fakeTaskRepo{}
	uc := NewTaskUsecase(repo)
	ctx := context.Background()

	if _, err := uc.Publish(ctx, "missing", []string{"s1"}); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("want not found, got %v", err)
	}

	repo.task = &Task{ID: "t1", Status: TaskPublished, DueAt: future()}
	if _, err := uc.Publish(ctx, "t1", []string{"s1"}); !errors.Is(err, ErrTaskState) {
		t.Fatalf("want state error, got %v", err)
	}

	repo.task = &Task{ID: "t1", Status: TaskDraft, DueAt: past()}
	if _, err := uc.Publish(ctx, "t1", []string{"s1"}); !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("want invalid due, got %v", err)
	}

	repo.task = &Task{ID: "t1", Status: TaskDraft, DueAt: future()}
	if _, err := uc.Publish(ctx, "t1", nil); !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("want empty students, got %v", err)
	}

	n, err := uc.Publish(ctx, "t1", []string{"s1", "s1", " ", "s2"})
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(repo.publishIDs) != 2 || repo.publishIDs[0] != "s1" || repo.publishIDs[1] != "s2" {
		t.Fatalf("ids=%v", repo.publishIDs)
	}
}

func TestTaskReviewAndSubmitValidation(t *testing.T) {
	repo := &fakeTaskRepo{}
	uc := NewTaskUsecase(repo)
	ctx := context.Background()
	high := 101.0
	if err := uc.Review(ctx, "a1", "staff-1", &high, "", ""); !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("want invalid score, got %v", err)
	}
	ok := 88.0
	if err := uc.Review(ctx, "a1", "staff-1", &ok, "不错", ""); err != nil {
		t.Fatalf("review: %v", err)
	}
	if len(repo.reviewArgs) != 5 {
		t.Fatalf("review args=%v", repo.reviewArgs)
	}

	if _, err := uc.Submit(ctx, "g1", "a1", "", nil); !errors.Is(err, ErrNothingToSubmit) {
		t.Fatalf("want nothing, got %v", err)
	}
	if _, err := uc.Submit(ctx, "g1", "a1", "text", make([]string, 10)); !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("want too many files, got %v", err)
	}
	sub, err := uc.Submit(ctx, "g1", "a1", "答案", []string{"cloud://a"})
	if err != nil || sub.ID != "s1" {
		t.Fatalf("submit=%v err=%v", sub, err)
	}
}

func TestTaskDetailStats(t *testing.T) {
	due := past()
	repo := &fakeTaskRepo{
		task: &Task{ID: "t1"},
		assignments: []TaskAssignment{
			{Status: AssignmentPending, DueAt: due},
			{Status: AssignmentSubmitted},
			{Status: AssignmentReviewed},
			{Status: AssignmentReturned},
		},
	}
	detail, err := NewTaskUsecase(repo).Detail(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Stats.Assigned != 4 || detail.Stats.Submitted != 3 || detail.Stats.Reviewed != 1 ||
		detail.Stats.Returned != 1 || detail.Stats.Pending != 1 || detail.Stats.PendingReview != 1 || detail.Stats.Overdue != 1 {
		t.Fatalf("stats=%+v", detail.Stats)
	}
	if detail.Stats.Rate != 0.75 {
		t.Fatalf("rate=%v", detail.Stats.Rate)
	}
}
