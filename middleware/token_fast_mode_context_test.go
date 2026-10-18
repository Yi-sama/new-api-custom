package middleware

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupContextForTokenSetsFastMode(t *testing.T) {
	for _, fastMode := range []bool{false, true} {
		t.Run("fast_mode="+strconv.FormatBool(fastMode), func(t *testing.T) {
			ctx := newTokenAutoGroupsContext()
			require.NoError(t, SetupContextForToken(ctx, &model.Token{Id: 1, UserId: 2, FastMode: fastMode}))
			value, ok := common.GetContextKey(ctx, constant.ContextKeyTokenFastMode)
			require.True(t, ok)
			assert.Equal(t, fastMode, value)
		})
	}
}
