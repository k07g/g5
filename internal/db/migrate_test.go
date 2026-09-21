package db

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestMigrate(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	// 現状の埋め込みマイグレーションは0001の1件のみ。CREATE TABLE文が
	// 実行されることを確認する(内容の完全一致ではなく、対象テーブルを
	// 含む何らかのSQLが1回実行されることを検証する)。
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))

	if err := Migrate(context.Background(), mockDB); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigrate_ExecError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	mock.ExpectExec("CREATE TABLE").WillReturnError(errors.New("exec failed"))

	if err := Migrate(context.Background(), mockDB); err == nil {
		t.Fatal("Migrate returned nil error, want the underlying exec error to propagate")
	}
}
