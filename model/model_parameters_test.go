package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioModelParametersApplyGroupOverridesPerField(t *testing.T) {
	parameters, err := ParseStudioModelParameters(`{
        "video":{"durations":[4,8],"resolutions":["720p"],"modes":["frames"]},
        "groups":{"premium":{"video":{"durations":[6],"modes":["reference"]}}}
    }`)
	require.NoError(t, err)

	profile := parameters.Profile("video", "premium")
	assert.Equal(t, []int{6}, profile.Durations)
	assert.Equal(t, []string{"720p"}, profile.Resolutions)
	assert.Equal(t, []string{"reference"}, profile.Modes)
}

func TestStudioModelParametersRejectNonPositiveBillingChoices(t *testing.T) {
	_, err := ParseStudioModelParameters(`{"image":{"counts":[0]}}`)
	require.Error(t, err)
}
