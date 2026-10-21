package task

import (
	"context"
	"unicode/utf8"
)

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{repo: r} }
func (s *Service) Create(c context.Context, in Input) (Task, error) {
	if e := in.Validate(); e != nil {
		return Task{}, e
	}
	return s.repo.Create(c, in.normalized())
}
func (s *Service) Get(c context.Context, id string) (Task, error) { return s.repo.Get(c, id) }
func (s *Service) List(c context.Context, options ListOptions) ([]Task, int, error) {
	if options.Page == 0 {
		options.Page = 1
	}
	if options.PageSize == 0 {
		options.PageSize = 20
	}
	if options.Page < 1 || options.Page > 1000000 || options.PageSize < 1 || options.PageSize > 100 || utf8.RuneCountInString(options.Assignee) > 200 {
		return nil, 0, ErrInvalid
	}
	if options.Status != "" && options.Status != Pending && options.Status != InProgress && options.Status != Completed {
		return nil, 0, ErrInvalid
	}
	return s.repo.List(c, options)
}
func (s *Service) Update(c context.Context, id string, in Input) (Task, error) {
	if e := in.Validate(); e != nil {
		return Task{}, e
	}
	return s.repo.Update(c, id, in.normalized())
}
func (s *Service) Delete(c context.Context, id string) error { return s.repo.Delete(c, id) }
func (s *Service) Count(c context.Context) (int, error)      { return s.repo.Count(c) }
