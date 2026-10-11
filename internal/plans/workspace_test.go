package plans

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v5"
)

func TestHasWorkspaceBusiness(t *testing.T) {
	ctx := context.Background()

	t.Run("workspace on business needs no lookup", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		ok, err := HasWorkspaceBusiness(ctx, mock, "org-1", "business")
		if err != nil || !ok {
			t.Fatalf("got %v, %v; want true", ok, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	// A personal Business subscription covers the workspaces its holder owns.
	for _, owner := range []bool{true, false} {
		mock, _ := pgxmock.NewPool()
		mock.ExpectQuery(`SELECT EXISTS`).WithArgs("org-1").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(owner))
		ok, err := HasWorkspaceBusiness(ctx, mock, "org-1", "free")
		if err != nil || ok != owner {
			t.Fatalf("owner business=%v: got %v, %v", owner, ok, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		mock.Close()
	}
}
