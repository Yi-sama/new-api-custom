package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoStudioTokenIsPurposeScoped(t *testing.T) {
	useTestSessionSecret(t)
	identity := AuthIdentity{UserID: 42, SessionID: "video-session", UserAuthVersion: 2, SessionVersion: 3}
	token, expiresAt, err := IssueVideoStudioToken(identity, time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	assert.InDelta(t, time.Now().Add(ImageStudioTokenTTL).Unix(), expiresAt, 2)
	parsed, err := ParseVideoStudioToken(token)
	require.NoError(t, err)
	assert.Equal(t, identity, parsed)

	_, err = ParseAccessToken(token)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)
	_, internal, err := ParseDashboardAccessToken(token)
	assert.True(t, internal)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)

	imageToken, _, err := IssueImageStudioToken(identity, "user", time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = ParseVideoStudioToken(imageToken)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)
}
