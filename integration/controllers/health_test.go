package controllers_test

import (
	"context"
	"testing"

	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/stretchr/testify/require"
)

func TestCheckDBHealth(t *testing.T) {
	helpers.LoadTestConfig(t)
	require.Nil(t, controllers.CheckDBHealth(context.Background()))
}
