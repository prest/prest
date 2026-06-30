package controllers

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

func TestValidateDatabase_SingleDBMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test")

	err := validateDatabase("prest-test", db, true)
	require.NoError(t, err)
}

func TestValidateDatabase_SingleDBMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test")

	err := validateDatabase("other", db, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "database not registered")
}

func TestValidateDatabase_MultiDB(t *testing.T) {
	err := validateDatabase("anything", nil, false)
	require.NoError(t, err)
}

func TestValidatePathSegments(t *testing.T) {
	require.True(t, validatePathSegments("public", "users"))
	require.False(t, validatePathSegments("bad@schema"))
	require.False(t, validatePathSegments("public", "bad-table;drop"))
}
