package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useTestSessionSecret(t *testing.T) {
	t.Helper()
	previous := common.SessionSecret
	common.SessionSecret = "test-session-secret-with-sufficient-entropy"
	t.Cleanup(func() { common.SessionSecret = previous })
}

func TestAccessTokenRoundTripAndPurposeIsolation(t *testing.T) {
	useTestSessionSecret(t)
	identity := AuthIdentity{UserID: 42, SessionID: "session-1", UserAuthVersion: 3, SessionVersion: 2}

	token, expiresAt, err := IssueAccessToken(identity)
	require.NoError(t, err)
	assert.Positive(t, expiresAt)

	parsed, err := ParseAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, identity, parsed)

	proof, _, err := IssueSecurityProof(identity, "2fa", []string{"channel.key.read"})
	require.NoError(t, err)
	_, err = ParseAccessToken(proof)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)
}

func TestAccessTokenRejectsTampering(t *testing.T) {
	useTestSessionSecret(t)
	identity := AuthIdentity{UserID: 42, SessionID: "session-1", UserAuthVersion: 1, SessionVersion: 1}
	token, _, err := IssueAccessToken(identity)
	require.NoError(t, err)

	tamperAt := len(token) - 2
	replacement := "x"
	if token[tamperAt] == 'x' {
		replacement = "y"
	}
	tampered := token[:tamperAt] + replacement + token[tamperAt+1:]
	_, err = ParseAccessToken(tampered)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)

	_, internal, err := ParseDashboardAccessToken(tampered)
	assert.True(t, internal)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)
}

func TestDashboardAccessTokenClassification(t *testing.T) {
	useTestSessionSecret(t)

	identity, internal, err := ParseDashboardAccessToken("opaque.key.with-dots")
	require.NoError(t, err)
	assert.False(t, internal)
	assert.Empty(t, identity)

	external := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "external-issuer",
		"aud": authTokenAudience,
		"exp": time.Now().Add(time.Minute).Unix(),
	})
	externalRaw, err := external.SignedString([]byte("external-secret"))
	require.NoError(t, err)
	_, internal, err = ParseDashboardAccessToken(externalRaw)
	require.NoError(t, err)
	assert.False(t, internal)

	unknownUse := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":       authTokenIssuer,
		"aud":       authTokenAudience,
		"token_use": "third_party",
		"exp":       time.Now().Add(time.Minute).Unix(),
	})
	unknownUseRaw, err := unknownUse.SignedString([]byte("external-secret"))
	require.NoError(t, err)
	_, internal, err = ParseDashboardAccessToken(unknownUseRaw)
	require.NoError(t, err)
	assert.False(t, internal)

	proof, _, err := IssueSecurityProof(AuthIdentity{
		UserID: 42, SessionID: "session-1", UserAuthVersion: 1, SessionVersion: 1,
	}, "2fa", []string{"channel.key.read"})
	require.NoError(t, err)
	_, internal, err = ParseDashboardAccessToken(proof)
	assert.True(t, internal)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)

	expiredClaims := authClaims{
		TokenUse:        accessTokenUse,
		SessionID:       "expired-session",
		UserAuthVersion: 1,
		SessionVersion:  1,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    authTokenIssuer,
			Subject:   "42",
			Audience:  jwt.ClaimStrings{authTokenAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			ID:        "expired-token",
		},
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(authSigningKey(accessTokenUse))
	require.NoError(t, err)
	_, internal, err = ParseDashboardAccessToken(expired)
	assert.True(t, internal)
	assert.ErrorIs(t, err, ErrAuthTokenExpired)
}

func TestSecurityProofBindsIdentityMethodAndScope(t *testing.T) {
	useTestSessionSecret(t)
	identity := AuthIdentity{UserID: 42, SessionID: "session-1", UserAuthVersion: 3, SessionVersion: 2}
	proof, _, err := IssueSecurityProof(identity, "2fa", []string{"channel.key.read"})
	require.NoError(t, err)

	method, err := VerifySecurityProof(proof, identity, "channel.key.read", []string{"2fa", "passkey"})
	require.NoError(t, err)
	assert.Equal(t, "2fa", method)

	_, err = VerifySecurityProof(proof, identity, "passkey.delete", []string{"2fa"})
	assert.ErrorIs(t, err, ErrProofScope)

	_, err = VerifySecurityProof(proof, identity, "channel.key.read", []string{"passkey"})
	assert.ErrorIs(t, err, ErrProofMethod)

	otherSession := identity
	otherSession.SessionID = "session-2"
	_, err = VerifySecurityProof(proof, otherSession, "channel.key.read", []string{"2fa"})
	assert.True(t, errors.Is(err, ErrAuthTokenInvalid))
}

func TestImageStudioTokenPurposeTamperingAndExpiry(t *testing.T) {
	useTestSessionSecret(t)
	identity := AuthIdentity{UserID: 42, SessionID: "session-1", UserAuthVersion: 3, SessionVersion: 2}

	token, expiresAt, err := IssueImageStudioToken(identity, "user", time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	assert.InDelta(t, time.Now().Add(ImageStudioTokenTTL).Unix(), expiresAt, 2)
	parsed, role, err := ParseImageStudioToken(token)
	require.NoError(t, err)
	assert.Equal(t, identity, parsed)
	assert.Equal(t, "user", role)
	_, sessionBoundExpiry, err := IssueImageStudioToken(identity, "user", time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.InDelta(t, time.Now().Add(time.Hour).Unix(), sessionBoundExpiry, 2)

	_, err = ParseAccessToken(token)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)
	_, internal, err := ParseDashboardAccessToken(token)
	assert.True(t, internal)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)

	accessToken, _, err := IssueAccessToken(identity)
	require.NoError(t, err)
	_, _, err = ParseImageStudioToken(accessToken)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)

	signatureStart := strings.LastIndexByte(token, '.') + 1
	replacement := byte('x')
	if token[signatureStart] == replacement {
		replacement = 'y'
	}
	tampered := token[:signatureStart] + string(replacement) + token[signatureStart+1:]
	_, _, err = ParseImageStudioToken(tampered)
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)

	expiredClaims := authClaims{
		TokenUse:        imageStudioTokenUse,
		SessionID:       identity.SessionID,
		UserAuthVersion: identity.UserAuthVersion,
		SessionVersion:  identity.SessionVersion,
		StudioRole:      "user",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    authTokenIssuer,
			Subject:   "42",
			Audience:  jwt.ClaimStrings{imageStudioAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			ID:        "expired-image-studio-token",
		},
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(authSigningKey(imageStudioTokenUse))
	require.NoError(t, err)
	_, _, err = ParseImageStudioToken(expired)
	assert.ErrorIs(t, err, ErrAuthTokenExpired)

	_, _, err = IssueImageStudioToken(identity, "admin\nuser", time.Now().Add(time.Hour))
	assert.ErrorIs(t, err, ErrAuthTokenInvalid)
}
