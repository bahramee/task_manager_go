package task

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPostgresRepositoryCreateAndGetNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO tasks(title,description,status,assignee) VALUES($1,$2,$3,$4) RETURNING `+fields)).WithArgs("Plan", "", "pending", "").WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "status", "assignee", "created_at", "updated_at"}).AddRow("id-1", "Plan", "", "pending", "", now, now))
	got, err := repo.Create(context.Background(), Input{Title: "Plan"})
	if err != nil || got.ID != "id-1" || got.Status != Pending {
		t.Fatalf("Create = %+v, %v", got, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + fields + ` FROM tasks WHERE id=$1`)).WithArgs("missing").WillReturnError(sql.ErrNoRows)
	if _, err = repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing error = %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestPostgresRepositoryListUpdateDeleteAndCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now()
	options := ListOptions{Status: InProgress, Assignee: "Alex", Page: 2, PageSize: 10}
	countQuery := `SELECT count(*) FROM tasks WHERE TRUE AND status=$1 AND assignee=$2`
	listQuery := `SELECT ` + fields + ` FROM tasks WHERE TRUE AND status=$1 AND assignee=$2 ORDER BY created_at DESC,id LIMIT $3 OFFSET $4`
	mock.ExpectQuery(regexp.QuoteMeta(countQuery)).WithArgs(InProgress, "Alex").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(11))
	mock.ExpectQuery(regexp.QuoteMeta(listQuery)).WithArgs(InProgress, "Alex", 10, 10).WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "status", "assignee", "created_at", "updated_at"}).AddRow("id-2", "Task", "", "in_progress", "Alex", now, now))
	items, total, err := repo.List(context.Background(), options)
	if err != nil || total != 11 || len(items) != 1 || items[0].ID != "id-2" {
		t.Fatalf("List = %+v, %d, %v", items, total, err)
	}

	updateQuery := `UPDATE tasks SET title=$2,description=$3,status=$4,assignee=$5,updated_at=now() WHERE id=$1 RETURNING ` + fields
	mock.ExpectQuery(regexp.QuoteMeta(updateQuery)).WithArgs("id-2", "Updated", "", Completed, "").WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "status", "assignee", "created_at", "updated_at"}).AddRow("id-2", "Updated", "", "completed", "", now, now))
	if updated, err := repo.Update(context.Background(), "id-2", Input{Title: "Updated", Status: Completed}); err != nil || updated.Status != Completed {
		t.Fatalf("Update = %+v, %v", updated, err)
	}

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM tasks WHERE id=$1`)).WithArgs("id-2").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.Delete(context.Background(), "id-2"); err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM tasks WHERE id=$1`)).WithArgs("missing").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := repo.Delete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete missing error = %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM tasks`)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	if n, err := repo.Count(context.Background()); err != nil || n != 3 {
		t.Fatalf("Count = %d, %v", n, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMapDBErrors(t *testing.T) {
	if err := mapDBError(nil); err != nil {
		t.Fatalf("mapDBError(nil) = %v", err)
	}
	invalid := mapDBError(&pgconn.PgError{Code: "23514"})
	if !errors.Is(invalid, ErrInvalid) {
		t.Fatalf("constraint error = %v", invalid)
	}
	other := errors.New("connection failed")
	if got := mapDBError(other); !errors.Is(got, other) {
		t.Fatalf("unexpected mapped error: %v", got)
	}
}
