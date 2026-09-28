package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOnlineTopUpAmountBounds(t *testing.T) {
	testCases := []struct {
		name    string
		amount  int64
		wantErr bool
	}{
		{name: "minimum amount", amount: 1},
		{name: "maximum amount", amount: 1000},
		{name: "zero amount", amount: 0, wantErr: true},
		{name: "amount above maximum", amount: 1001, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOnlineTopUpAmount(tc.amount)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
