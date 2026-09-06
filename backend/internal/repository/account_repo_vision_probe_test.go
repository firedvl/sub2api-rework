package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestConsumeOpenAIVisionProbeCandidateUsesBoundedAtomicCounter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := newAccountRepositoryWithSQL(nil, db, nil)
	groupID := int64(4)
	mock.ExpectQuery(`(?s)UPDATE accounts.*jsonb_typeof\(credentials -> \$3\) = 'number'.*BETWEEN 1 AND 10.*RETURNING id`).
		WithArgs(int64(31), groupID, service.OpenAIVisionProbeCandidateCredentialKey).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(31)))

	claimed, err := repo.ConsumeOpenAIVisionProbeCandidate(context.Background(), 31, &groupID)
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("expected candidate claim")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConsumeOpenAIVisionProbeCandidateRejectsMissingState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := newAccountRepositoryWithSQL(nil, db, nil)
	mock.ExpectQuery(`(?s)UPDATE accounts.*RETURNING id`).
		WithArgs(int64(31), nil, service.OpenAIVisionProbeCandidateCredentialKey).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	claimed, err := repo.ConsumeOpenAIVisionProbeCandidate(context.Background(), 31, nil)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("missing candidate state should not claim")
	}
}
