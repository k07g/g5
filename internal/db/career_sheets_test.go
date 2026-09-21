package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/k07g/g5/internal/models"
)

// testRepo is shared across every test in this file, backed by a single
// MongoDB testcontainer started once in TestMain. mongo-driver v2 has no
// externally usable equivalent of database/sql's sqlmock (its own mtest
// helper lives under an internal/ package and can't be imported from
// outside the driver module), so exercising this repository's actual
// query/update logic means talking to a real MongoDB deployment.
var testRepo *CareerSheetRepository

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

// runTests starts the MongoDB container, runs the suite, and tears the
// container down — as its own function so `defer container.Terminate`
// actually executes (a deferred call in TestMain itself would be skipped
// by the os.Exit(m.Run()) pattern).
func runTests(m *testing.M) int {
	ctx := context.Background()

	container, err := mongodb.Run(ctx, "mongo:8")
	if err != nil {
		// No Docker available (e.g. a restricted sandbox or a contributor's
		// machine without it running) — skip rather than fail, so `go test
		// ./...` degrades gracefully instead of breaking everywhere.
		fmt.Fprintf(os.Stderr, "internal/db: skipping MongoDB-backed tests, failed to start testcontainer (is Docker running?): %v\n", err)
		return 0
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "internal/db: failed to terminate MongoDB testcontainer: %v\n", err)
		}
	}()

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "internal/db: failed to read MongoDB testcontainer connection string: %v\n", err)
		return 1
	}

	client, database, err := Connect(ctx, uri, "career_sheets_test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "internal/db: failed to connect to MongoDB testcontainer: %v\n", err)
		return 1
	}
	defer client.Disconnect(ctx)

	testRepo = NewCareerSheetRepository(database)

	return m.Run()
}

// uniqueSub returns a cognito sub scoped to the calling test, so tests
// sharing one container/collection never collide with each other's data.
func uniqueSub(t *testing.T) string {
	t.Helper()
	return t.Name() + "-" + time.Now().Format("150405.000000000")
}

func sampleSheet() *models.CareerSheet {
	sheet := &models.CareerSheet{
		BasicInfo: models.BasicInfo{Name: "山田太郎", Email: "yamada@example.com"},
		Summary:   "summary",
	}
	sheet.Normalize()
	return sheet
}

func TestCareerSheetRepository_Get_NotFound(t *testing.T) {
	_, err := testRepo.Get(context.Background(), uniqueSub(t))
	if !errors.Is(err, ErrCareerSheetNotFound) {
		t.Errorf("Get error = %v, want %v", err, ErrCareerSheetNotFound)
	}
}

func TestCareerSheetRepository_UpsertAndGet(t *testing.T) {
	ctx := context.Background()
	sub := uniqueSub(t)
	t.Cleanup(func() { _ = testRepo.Delete(ctx, sub) })

	sheet := sampleSheet()
	if err := testRepo.Upsert(ctx, sub, sheet); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	got, err := testRepo.Get(ctx, sub)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.BasicInfo.Name != sheet.BasicInfo.Name {
		t.Errorf("BasicInfo.Name = %q, want %q", got.BasicInfo.Name, sheet.BasicInfo.Name)
	}
	if got.WorkExperiences == nil {
		t.Error("WorkExperiences should be normalized to a non-nil empty slice")
	}
}

func TestCareerSheetRepository_Upsert_UpdatesExisting(t *testing.T) {
	ctx := context.Background()
	sub := uniqueSub(t)
	t.Cleanup(func() { _ = testRepo.Delete(ctx, sub) })

	first := sampleSheet()
	first.BasicInfo.Name = "山田太郎"
	if err := testRepo.Upsert(ctx, sub, first); err != nil {
		t.Fatalf("first Upsert returned error: %v", err)
	}

	second := sampleSheet()
	second.BasicInfo.Name = "山田次郎"
	second.Summary = "updated"
	if err := testRepo.Upsert(ctx, sub, second); err != nil {
		t.Fatalf("second Upsert returned error: %v", err)
	}

	got, err := testRepo.Get(ctx, sub)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.BasicInfo.Name != "山田次郎" || got.Summary != "updated" {
		t.Errorf("got = %+v, want the second sheet's contents", got)
	}
}

func TestCareerSheetRepository_Upsert_PreservesCreatedAt(t *testing.T) {
	ctx := context.Background()
	sub := uniqueSub(t)
	t.Cleanup(func() { _ = testRepo.Delete(ctx, sub) })

	if err := testRepo.Upsert(ctx, sub, sampleSheet()); err != nil {
		t.Fatalf("first Upsert returned error: %v", err)
	}

	var firstDoc careerSheetDocument
	if err := testRepo.collection.FindOne(ctx, bson.M{"_id": sub}).Decode(&firstDoc); err != nil {
		t.Fatalf("failed to load document: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // ensure a measurable timestamp difference

	if err := testRepo.Upsert(ctx, sub, sampleSheet()); err != nil {
		t.Fatalf("second Upsert returned error: %v", err)
	}

	var secondDoc careerSheetDocument
	if err := testRepo.collection.FindOne(ctx, bson.M{"_id": sub}).Decode(&secondDoc); err != nil {
		t.Fatalf("failed to reload document: %v", err)
	}

	if !firstDoc.CreatedAt.Equal(secondDoc.CreatedAt) {
		t.Errorf("CreatedAt changed across updates: first = %v, second = %v", firstDoc.CreatedAt, secondDoc.CreatedAt)
	}
	if !secondDoc.UpdatedAt.After(firstDoc.UpdatedAt) {
		t.Errorf("UpdatedAt did not advance: first = %v, second = %v", firstDoc.UpdatedAt, secondDoc.UpdatedAt)
	}
}

func TestCareerSheetRepository_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes an existing sheet", func(t *testing.T) {
		sub := uniqueSub(t)
		if err := testRepo.Upsert(ctx, sub, sampleSheet()); err != nil {
			t.Fatalf("Upsert returned error: %v", err)
		}

		if err := testRepo.Delete(ctx, sub); err != nil {
			t.Errorf("Delete returned error: %v", err)
		}

		if _, err := testRepo.Get(ctx, sub); !errors.Is(err, ErrCareerSheetNotFound) {
			t.Errorf("Get after delete error = %v, want %v", err, ErrCareerSheetNotFound)
		}
	})

	t.Run("returns ErrCareerSheetNotFound when nothing to delete", func(t *testing.T) {
		err := testRepo.Delete(ctx, uniqueSub(t))
		if !errors.Is(err, ErrCareerSheetNotFound) {
			t.Errorf("Delete error = %v, want %v", err, ErrCareerSheetNotFound)
		}
	})
}

func TestCareerSheetRepository_ScopedPerUser(t *testing.T) {
	ctx := context.Background()
	subA := uniqueSub(t) + "-a"
	subB := uniqueSub(t) + "-b"
	t.Cleanup(func() {
		_ = testRepo.Delete(ctx, subA)
		_ = testRepo.Delete(ctx, subB)
	})

	sheetA := sampleSheet()
	sheetA.BasicInfo.Name = "A"
	if err := testRepo.Upsert(ctx, subA, sheetA); err != nil {
		t.Fatalf("Upsert(subA) returned error: %v", err)
	}

	if _, err := testRepo.Get(ctx, subB); !errors.Is(err, ErrCareerSheetNotFound) {
		t.Errorf("Get(subB) error = %v, want %v (sheets must not leak across users)", err, ErrCareerSheetNotFound)
	}
}
