package controllers

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

func TestValidateDatabase_SingleDBMatch(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("prest-test").Return(true)
	db.EXPECT().GetDatabase().Return("prest-test")

	err := validateDatabase("prest-test", db, true)
	require.NoError(t, err)
}

func TestValidateDatabase_SingleDBMismatch(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("other").Return(true)
	db.EXPECT().GetDatabase().Return("prest-test")

	err := validateDatabase("other", db, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "database not registered")
}

func TestValidateDatabase_MultiDB(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("anything").Return(true)

	err := validateDatabase("anything", db, false)
	require.NoError(t, err)
}

func TestValidateDatabase_RegistryUnknown(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("unknown").Return(false)

	err := validateDatabase("unknown", db, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "database not registered")
}

func TestValidatePathSegments(t *testing.T) {
	t.Parallel()

	require.True(t, validatePathSegments("public", "users"))
	require.False(t, validatePathSegments("bad@schema"))
	require.False(t, validatePathSegments("public", "bad-table;drop"))
}
