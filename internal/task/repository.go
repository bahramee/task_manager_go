package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

type Repository interface {
	Create(context.Context, Input) (Task, error)
	Get(context.Context, string) (Task, error)
	List(context.Context, ListOptions) ([]Task, int, error)
	Update(context.Context, string, Input) (Task, error)
	Delete(context.Context, string) error
	Count(context.Context) (int, error)
}

type ListOptions struct {
	Status   Status
	Assignee string
	Page     int
	PageSize int
}

type PostgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) *PostgresRepository { return &PostgresRepository{db: db} }

const fields = `id::text,title,description,status,assignee,created_at,updated_at`

func scanTask(row interface{ Scan(...any) error }) (Task, error) {
	var t Task
	var status string
	err := row.Scan(&t.ID, &t.Title, &t.Description, &status, &t.Assignee, &t.CreatedAt, &t.UpdatedAt)
	t.Status = Status(status)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}
func (r *PostgresRepository) Create(ctx context.Context, in Input) (Task, error) {
	in = in.normalized()
	t, err := scanTask(r.db.QueryRowContext(ctx, `INSERT INTO tasks(title,description,status,assignee) VALUES($1,$2,$3,$4) RETURNING `+fields, in.Title, in.Description, in.Status, in.Assignee))
	return t, mapDBError(err)
}
func (r *PostgresRepository) Get(ctx context.Context, id string) (Task, error) {
	t, e := scanTask(r.db.QueryRowContext(ctx, `SELECT `+fields+` FROM tasks WHERE id=$1`, id))
	return t, mapDBError(e)
}
func (r *PostgresRepository) List(ctx context.Context, options ListOptions) ([]Task, int, error) {
	where := " WHERE TRUE"
	args := make([]any, 0, 4)
	if options.Status != "" {
		args = append(args, options.Status)
		where += fmt.Sprintf(" AND status=$%d", len(args))
	}
	if options.Assignee != "" {
		args = append(args, options.Assignee)
		where += fmt.Sprintf(" AND assignee=$%d", len(args))
	}
	var total int
	if e := r.db.QueryRowContext(ctx, `SELECT count(*) FROM tasks`+where, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	args = append(args, options.PageSize, (options.Page-1)*options.PageSize)
	rows, e := r.db.QueryContext(ctx, `SELECT `+fields+` FROM tasks`+where+fmt.Sprintf(" ORDER BY created_at DESC,id LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, e := scanTask(rows)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}
func (r *PostgresRepository) Update(ctx context.Context, id string, in Input) (Task, error) {
	in = in.normalized()
	t, e := scanTask(r.db.QueryRowContext(ctx, `UPDATE tasks SET title=$2,description=$3,status=$4,assignee=$5,updated_at=now() WHERE id=$1 RETURNING `+fields, id, in.Title, in.Description, in.Status, in.Assignee))
	return t, mapDBError(e)
}
func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	res, e := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id=$1`, id)
	if e != nil {
		return mapDBError(e)
	}
	n, e := res.RowsAffected()
	if e == nil && n == 0 {
		return ErrNotFound
	}
	return e
}
func (r *PostgresRepository) Count(ctx context.Context) (int, error) {
	var n int
	e := r.db.QueryRowContext(ctx, `SELECT count(*) FROM tasks`).Scan(&n)
	return n, e
}
func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23514" {
		return fmt.Errorf("%w: database constraint", ErrInvalid)
	}
	return err
}
