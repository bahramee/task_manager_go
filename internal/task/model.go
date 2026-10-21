package task

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type Status string

const (
	Pending    Status = "pending"
	InProgress Status = "in_progress"
	Completed  Status = "completed"
)

var (
	ErrNotFound = errors.New("task not found")
	ErrInvalid  = errors.New("invalid task")
)

type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Assignee    string    `json:"assignee,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Input struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      Status `json:"status"`
	Assignee    string `json:"assignee"`
}

func (in Input) Validate() error {
	if strings.TrimSpace(in.Title) == "" || utf8.RuneCountInString(in.Title) > 200 || utf8.RuneCountInString(in.Description) > 5000 || utf8.RuneCountInString(in.Assignee) > 200 {
		return ErrInvalid
	}
	if in.Status == "" {
		return nil
	}
	if in.Status != Pending && in.Status != InProgress && in.Status != Completed {
		return ErrInvalid
	}
	return nil
}
func (in Input) normalized() Input {
	in.Title = strings.TrimSpace(in.Title)
	if in.Status == "" {
		in.Status = Pending
	}
	return in
}
