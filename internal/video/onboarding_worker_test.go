package video

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

type mockOnboardingSender struct {
	day2Calls []string
	day7Calls []string
}

func (m *mockOnboardingSender) SendOnboardingDay2(ctx context.Context, toEmail, toName, dashboardURL string) error {
	m.day2Calls = append(m.day2Calls, toEmail)
	return nil
}

func (m *mockOnboardingSender) SendOnboardingDay7(ctx context.Context, toEmail, toName, dashboardURL string) error {
	m.day7Calls = append(m.day7Calls, toEmail)
	return nil
}

func TestProcessOnboardingDay2_SendsToEligibleUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	sender := &mockOnboardingSender{}

	mock.ExpectQuery(`SELECT id, email, name FROM users`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).
			AddRow("user-1", "alice@example.com", "Alice"))

	mock.ExpectExec(`UPDATE users SET onboarding_day2_sent_at`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	processOnboardingDay2(context.Background(), mock, sender, "https://app.sendrec.eu")

	if len(sender.day2Calls) != 1 {
		t.Fatalf("expected 1 day2 email, got %d", len(sender.day2Calls))
	}
	if sender.day2Calls[0] != "alice@example.com" {
		t.Errorf("expected email to alice@example.com, got %s", sender.day2Calls[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessOnboardingDay2_NoEligibleUsers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	sender := &mockOnboardingSender{}

	mock.ExpectQuery(`SELECT id, email, name FROM users`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}))

	processOnboardingDay2(context.Background(), mock, sender, "https://app.sendrec.eu")

	if len(sender.day2Calls) != 0 {
		t.Errorf("expected no emails, got %d", len(sender.day2Calls))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessOnboardingDay7_SendsToEligibleUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	sender := &mockOnboardingSender{}

	mock.ExpectQuery(`SELECT id, email, name FROM users`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).
			AddRow("user-1", "alice@example.com", "Alice"))

	mock.ExpectExec(`UPDATE users SET onboarding_day7_sent_at`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	processOnboardingDay7(context.Background(), mock, sender, "https://app.sendrec.eu")

	if len(sender.day7Calls) != 1 {
		t.Fatalf("expected 1 day7 email, got %d", len(sender.day7Calls))
	}
	if sender.day7Calls[0] != "alice@example.com" {
		t.Errorf("expected email to alice@example.com, got %s", sender.day7Calls[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestProcessOnboardingDay7_NoEligibleUsers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	sender := &mockOnboardingSender{}

	mock.ExpectQuery(`SELECT id, email, name FROM users`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}))

	processOnboardingDay7(context.Background(), mock, sender, "https://app.sendrec.eu")

	if len(sender.day7Calls) != 0 {
		t.Errorf("expected no emails, got %d", len(sender.day7Calls))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
