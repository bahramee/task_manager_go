package task

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRepo struct {
	createInput Input
	getTask     Task
	err         error
	listCalls   int
	listOptions ListOptions
}

func (f *fakeRepo) Create(_ context.Context, in Input) (Task, error) {
	f.createInput = in
	return Task{Title: in.Title, Status: in.Status}, f.err
}
func (f *fakeRepo) Get(context.Context, string) (Task, error) { return f.getTask, f.err }
func (f *fakeRepo) List(_ context.Context, options ListOptions) ([]Task, int, error) {
	f.listCalls++
	f.listOptions = options
	return []Task{f.getTask}, 1, f.err
}
func (f *fakeRepo) Update(_ context.Context, _ string, in Input) (Task, error) {
	return Task{Title: in.Title, Status: in.Status}, f.err
}
func (f *fakeRepo) Delete(context.Context, string) error { return f.err }
func (f *fakeRepo) Count(context.Context) (int, error)   { return 1, f.err }

func TestCreateDefaultsAndTrims(t *testing.T) {
	r := &fakeRepo{}
	got, err := NewService(r).Create(context.Background(), Input{Title: "  plan  "})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "plan" || got.Status != Pending {
		t.Fatalf("unexpected task: %+v", got)
	}
}

func TestCreateRejectsInvalid(t *testing.T) {
	for _, input := range []Input{
		{},
		{Title: "x", Status: "unknown"},
		{Title: " x ", Assignee: strings.Repeat("a", 201)},
		{Title: strings.Repeat("界", 201)},
		{Title: "x", Description: strings.Repeat("界", 5001)},
	} {
		if _, err := NewService(&fakeRepo{}).Create(context.Background(), input); !errors.Is(err, ErrInvalid) {
			t.Errorf("Create(%+v) error = %v", input, err)
		}
	}
}

func TestOperationsForwardRepositoryResults(t *testing.T) {
	want := Task{ID: "a", Title: "work"}
	repo := &fakeRepo{getTask: want}
	svc := NewService(repo)
	if got, _ := svc.Get(context.Background(), "a"); got != want {
		t.Fatalf("Get = %+v", got)
	}
	if got, total, err := svc.List(context.Background(), ListOptions{}); err != nil || total != 1 || len(got) != 1 {
		t.Fatalf("List = %+v, %d, %v", got, total, err)
	}
	if repo.listOptions.Page != 1 || repo.listOptions.PageSize != 20 {
		t.Fatalf("list defaults = %+v", repo.listOptions)
	}
	if got, err := svc.Update(context.Background(), "a", Input{Title: "next"}); err != nil || got.Status != Pending {
		t.Fatalf("Update = %+v, %v", got, err)
	}
	if err := svc.Delete(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.Count(context.Background()); err != nil || n != 1 {
		t.Fatalf("Count = %d, %v", n, err)
	}
}

func TestRepositoryErrorsPropagate(t *testing.T) {
	repo := &fakeRepo{err: ErrNotFound}
	svc := NewService(repo)
	if _, err := svc.Get(context.Background(), "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, _, err := svc.List(context.Background(), ListOptions{}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestListValidatesFiltersAndPagination(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	for _, options := range []ListOptions{
		{Page: -1, PageSize: 20},
		{Page: 1, PageSize: 101},
		{Page: 1, PageSize: 20, Status: "unknown"},
		{Page: 1, PageSize: 20, Assignee: strings.Repeat("a", 201)},
	} {
		if _, _, err := svc.List(context.Background(), options); !errors.Is(err, ErrInvalid) {
			t.Errorf("List(%+v) error = %v", options, err)
		}
	}
	if repo.listCalls != 0 {
		t.Fatalf("invalid options reached repository %d times", repo.listCalls)
	}
}
