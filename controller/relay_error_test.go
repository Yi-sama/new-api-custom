package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

func TestClientFacingRelayErrorMasksNonRetryableUpstreamErrors(t *testing.T) {
	original := types.WithOpenAIError(types.OpenAIError{
		Message: "upstream balance is insufficient",
		Type:    "insufficient_quota",
		Code:    "insufficient_quota",
	}, http.StatusForbidden)

	clientError := clientFacingRelayError(original, true)

	require.Equal(t, http.StatusServiceUnavailable, clientError.StatusCode)
	openAIError := clientError.ToOpenAIError()
	require.Equal(t, "上游服务暂不可用", openAIError.Message)
	require.Equal(t, "upstream_error", openAIError.Type)
	require.Equal(t, "upstream_service_unavailable", openAIError.Code)
}

func TestClientFacingRelayErrorPreservesLocalRateLimitAndServerErrors(t *testing.T) {
	tests := []struct {
		name              string
		upstreamAttempted bool
		statusCode        int
	}{
		{name: "local error", upstreamAttempted: false, statusCode: http.StatusForbidden},
		{name: "upstream rate limit", upstreamAttempted: true, statusCode: http.StatusTooManyRequests},
		{name: "upstream server error", upstreamAttempted: true, statusCode: http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := types.WithOpenAIError(types.OpenAIError{Message: "original upstream error"}, tt.statusCode)

			require.Same(t, original, clientFacingRelayError(original, tt.upstreamAttempted))
		})
	}
}
