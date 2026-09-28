package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestGetAutoEnableGroupsOmitsGroupsWithoutRealModels(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "default-model", EnableGroup: []string{"default"}},
		{ModelName: "Auto", EnableGroup: []string{"default", "empty", "vip"}},
	}

	assert.Equal(t, []string{"default"}, getAutoEnableGroups(
		pricing,
		[]string{"vip", "empty", "default"},
	))
}

func TestGetAutoEnableGroupsExpandsAllGroupModels(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "shared-model", EnableGroup: []string{"all"}},
	}

	assert.Equal(t, []string{"default", "vip"}, getAutoEnableGroups(
		pricing,
		[]string{"vip", "default"},
	))
}
