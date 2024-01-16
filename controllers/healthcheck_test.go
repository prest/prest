package controllers

import (
	"context"
	"errors"
	"testing"

	_ "github.com/lib/pq"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/prest/prest/adapters/mockgen"
)

func Test_CheckDBHealth(t *testing.T) {
	// Test case 1: Healthy database
	t.Run("Healthy database", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		adapter := mockgen.NewMockAdapter(ctrl)

		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(";").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		tx, err := db.Begin()
		require.NoError(t, err)

		adapter.EXPECT().GetTransactionCtx(gomock.Any()).
			Return(tx, nil)

		err = CheckDBHealth(context.Background(), adapter)
		require.Nil(t, err)
	})

	// Test case 2: Unhealthy database
	t.Run("Unhealthy database", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		adapter := mockgen.NewMockAdapter(ctrl)

		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		adapter.EXPECT().GetTransactionCtx(gomock.Any()).
			Return(nil, errors.New("could not connect to the database"))

		err = CheckDBHealth(context.Background(), adapter)
		require.Error(t, err)
	})
}
