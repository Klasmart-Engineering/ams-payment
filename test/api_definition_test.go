package test_test

import (
	"context"
	"testing"

	"bitbucket.org/calmisland/go-server-api/openapi/openapi3"
	"github.com/calmisland/go-testify/assert"
)

const (
	apiDefinitionPath = "../api-v1.json"
)

func TestValidateAPIDefinition(t *testing.T) {
	api, err := openapi3.Load(apiDefinitionPath)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	err = api.Validate(ctx)
	assert.NoError(t, err)
}
