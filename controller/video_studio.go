package controller

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const videoStudioPrefix = "/api/video-studio"

const videoStudioGroupHeader = "X-New-Api-Group"

const videoStudioBridgeUserKey = "video_studio_bridge_user"

type videoStudioBridgeUser struct {
	user *model.UserBase
}

func VideoStudioEnabled() bool {
	return strings.TrimSpace(os.Getenv("VIDEO_STUDIO_DIST_DIR")) != ""
}

func CreateVideoStudioSession(c *gin.Context) {
	if !VideoStudioEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": "VIDEO_STUDIO_DISABLED", "message": "video studio is not configured"})
		return
	}
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_SESSION_REQUIRED", "message": "a dashboard login session is required"})
		return
	}
	session, user, err := service.ValidateLoginSession(identity)
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	token, expiresAt, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"user_id": user.Id, "username": user.Username, "token": token, "expires_at": expiresAt,
	}})
}

// VideoStudioBridgeAuth validates the scoped iframe bearer and rebuilds relay
// context for the real user without exposing or requiring an API key.
func VideoStudioBridgeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !VideoStudioEnabled() {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": "VIDEO_STUDIO_DISABLED", "message": "video studio is not configured"})
			return
		}
		rawToken := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(rawToken), "bearer ") {
			rawToken = strings.TrimSpace(rawToken[7:])
		}
		identity, err := service.ParseVideoStudioToken(rawToken)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		_, user, err := service.ValidateLoginSession(identity)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		group := strings.TrimSpace(c.GetHeader(videoStudioGroupHeader))
		if group != "" {
			group, err = url.PathUnescape(group)
			if err != nil {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			group = strings.TrimSpace(group)
		}
		if group == "" {
			group = user.Group
		}
		for _, header := range []string{"Authorization", "Cookie", "Api-Key", "X-Api-Key", "X-Goog-Api-Key", videoStudioGroupHeader, "Specific-Channel-Id"} {
			c.Request.Header.Del(header)
		}
		if c.Keys != nil {
			delete(c.Keys, "specific_channel_id")
		}
		user.WriteContext(c)
		c.Set("id", user.Id)
		temporaryToken := &model.Token{UserId: user.Id, Name: "video-studio", Group: group}
		if err := middleware.SetupContextForToken(c, temporaryToken); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
		c.Set(videoStudioBridgeUserKey, videoStudioBridgeUser{user: user})
		c.Next()
	}
}

// VideoStudioGroupAuthorization protects group-scoped model reads and all
// submissions. Task reads remain owner-scoped even after access changes.
func VideoStudioGroupAuthorization() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get(videoStudioBridgeUserKey)
		bridgeUser, valid := value.(videoStudioBridgeUser)
		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		if !ok || !valid || bridgeUser.user == nil || !service.IsUserSelectableGroup(bridgeUser.user.Group, group) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func GetVideoStudioContext(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"user_id":  c.GetInt("id"),
		"username": common.GetContextKeyString(c, constant.ContextKeyUserName),
		"group":    common.GetContextKeyString(c, constant.ContextKeyUserGroup),
		"quota":    common.GetContextKeyInt(c, constant.ContextKeyUserQuota),
	}})
}

func GetVideoStudioModels(c *gin.Context) {
	value, ok := c.Get(videoStudioBridgeUserKey)
	bridgeUser, valid := value.(videoStudioBridgeUser)
	if !ok || !valid || bridgeUser.user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	catalog, err := service.GetVideoStudioCatalog(bridgeUser.user.Group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	models := make([]gin.H, 0)
	for _, entry := range catalog {
		if entry.ID != group {
			continue
		}
		for _, entryModel := range entry.Models {
			for _, capability := range entryModel.Capabilities {
				if capability == "video" {
					models = append(models, gin.H{"id": entryModel.ID, "object": "model", "created": 0, "owned_by": "new-api"})
					break
				}
			}
		}
		break
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": models, "object": "list"})
}

func GetVideoStudioCatalog(c *gin.Context) {
	value, ok := c.Get(videoStudioBridgeUserKey)
	bridgeUser, valid := value.(videoStudioBridgeUser)
	if !ok || !valid || bridgeUser.user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	groups, err := service.GetVideoStudioCatalog(bridgeUser.user.Group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"groups": groups}})
}

// VideoStudioVideoModelAuthorization is deliberately after Distribute: that
// middleware reads the request model but does not select a channel or charge.
func VideoStudioVideoModelAuthorization() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get(videoStudioBridgeUserKey)
		bridgeUser, valid := value.(videoStudioBridgeUser)
		if !ok || !valid || bridgeUser.user == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		allowed, err := service.IsVideoStudioGroupVideoModel(bridgeUser.user.Group, group, modelName)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if !allowed {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "model is not available for video generation", "type": "invalid_request_error"}})
			c.Abort()
			return
		}
		c.Next()
	}
}

func VideoStudioNormalizeRelayPath() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, videoStudioPrefix)
		if !strings.HasPrefix(path, "/v1/") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Request.URL.Path = path
		c.Request.URL.RawPath = path
		c.Request.RequestURI = path
		if c.Request.URL.RawQuery != "" {
			c.Request.RequestURI += "?" + c.Request.URL.RawQuery
		}
		c.Next()
	}
}

func prepareVideoStudioRelay(c *gin.Context, relayMode int) {
	if relayMode == relayconstant.RelayModeUnknown {
		relayMode = c.GetInt("relay_mode")
	}
	c.Set("relay_mode", relayMode)
	c.Request.URL.Path = "/pg" + strings.TrimPrefix(c.Request.URL.Path, "/v1")
	c.Request.URL.RawPath = c.Request.URL.Path
	c.Request.URL.Scheme = ""
	c.Request.URL.Host = ""
}

func VideoStudioRelay(c *gin.Context, relayFormat types.RelayFormat) {
	prepareVideoStudioRelay(c, relayconstant.RelayModeUnknown)
	Relay(c, relayFormat)
}

func VideoStudioRelayTask(c *gin.Context) {
	prepareVideoStudioRelay(c, relayconstant.RelayModeVideoSubmit)
	RelayTask(c)
}

func VideoStudioRelayTaskFetch(c *gin.Context) {
	prepareVideoStudioRelay(c, relayconstant.RelayModeVideoFetchByID)
	RelayTaskFetch(c)
}

func VideoStudioVideoProxy(c *gin.Context) {
	VideoProxy(c)
}
