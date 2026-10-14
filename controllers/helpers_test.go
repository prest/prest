package controllers

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

func TestValidateDatabase_Registered(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("prest-test").Return(true)

	err := validateDatabase("prest-test", db)
	require.NoError(t, err)
}

func TestValidateDatabase_MultiDB(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("anything").Return(true)

	err := validateDatabase("anything", db)
	require.NoError(t, err)
}

// An allow-listed name is accepted from IsRegistered alone; no default-database lookup.
func TestValidateDatabase_AllowListedAccepted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("extra").Return(true)

	require.NoError(t, validateDatabase("extra", db))
}

func TestValidateDatabase_RegistryUnknown(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("unknown").Return(false)

	err := validateDatabase("unknown", db)
	require.Error(t, err)
	require.Contains(t, err.Error(), "database not registered")
}

func TestValidatePathSegments(t *testing.T) {
	t.Parallel()

	require.True(t, validatePathSegments("public", "users"))
	require.False(t, validatePathSegments("bad@schema"))
	require.False(t, validatePathSegments("public", "bad-table;drop"))
}
