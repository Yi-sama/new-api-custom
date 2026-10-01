package service

import (
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	videoStudioTokenUse = "video_studio"
	videoStudioAudience = "new-api-video-studio"
)

// IssueVideoStudioToken creates a short-lived bearer credential for the local
// Video Studio iframe. It is deliberately distinct from dashboard and API keys.
func IssueVideoStudioToken(identity AuthIdentity, sessionExpiresAt time.Time) (string, int64, error) {
	if identity.UserID <= 0 || identity.SessionID == "" || identity.UserAuthVersion <= 0 || identity.SessionVersion <= 0 {
		return "", 0, ErrAuthTokenInvalid
	}
	now := time.Now()
	expiresAt := now.Add(ImageStudioTokenTTL)
	if sessionExpiresAt.Before(expiresAt) {
		expiresAt = sessionExpiresAt
	}
	if !expiresAt.After(now) {
		return "", 0, ErrAuthTokenExpired
	}
	claims := authClaims{
		TokenUse:        videoStudioTokenUse,
		SessionID:       identity.SessionID,
		UserAuthVersion: identity.UserAuthVersion,
		SessionVersion:  identity.SessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    authTokenIssuer,
			Subject:   strconv.Itoa(identity.UserID),
			Audience:  jwt.ClaimStrings{videoStudioAudience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(authSigningKey(videoStudioTokenUse))
	return signed, expiresAt.Unix(), err
}

func ParseVideoStudioToken(raw string) (AuthIdentity, error) {
	claims, err := parseAuthClaims(raw, videoStudioTokenUse, videoStudioAudience, authSigningKey(videoStudioTokenUse))
	if err != nil {
		return AuthIdentity{}, err
	}
	userID, err := strconv.Atoi(claims.Subject)
	if err != nil || userID <= 0 || claims.SessionID == "" || claims.UserAuthVersion <= 0 || claims.SessionVersion <= 0 {
		return AuthIdentity{}, ErrAuthTokenInvalid
	}
	return AuthIdentity{
		UserID:          userID,
		SessionID:       claims.SessionID,
		UserAuthVersion: claims.UserAuthVersion,
		SessionVersion:  claims.SessionVersion,
	}, nil
}
