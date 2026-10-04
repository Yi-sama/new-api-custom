package controller

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	imageStudioBridgePrefix  = "/api/image-studio"
	imageStudioBridgeUserKey = "image_studio_bridge_user"
)

type imageStudioBridgeUser struct {
	user     *model.UserBase
	role     string
	identity string
}

// ImageStudioBridgeAuth authenticates Studio requests with the short-lived
// HttpOnly Studio session and builds the same user context as an API token.
func ImageStudioBridgeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		config, err := loadImageStudioConfig()
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		rawToken, err := c.Cookie(imageStudioCookieName)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		identity, issuedRole, err := service.ParseImageStudioToken(rawToken)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		_, user, err := service.ValidateLoginSession(identity)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		role, ok := imageStudioRole(user.Role)
		if !ok || role != issuedRole {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		expectedIdentity, err := imageStudioEmail(config, user.Id)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if suppliedIdentity := strings.TrimSpace(c.GetHeader("X-NewAPI-Identity")); suppliedIdentity != "" && !strings.EqualFold(suppliedIdentity, expectedIdentity) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		group := strings.TrimSpace(c.GetHeader("X-NewAPI-Group"))
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
		for _, header := range []string{"Authorization", "Cookie", "X-Api-Key", "X-Goog-Api-Key", "X-NewAPI-Group", "X-NewAPI-Identity", "X-Studio-Email", "X-Studio-Name", "X-Studio-Role", "Specific-Channel-Id"} {
			c.Request.Header.Del(header)
		}
		if c.Keys != nil {
			delete(c.Keys, "specific_channel_id")
		}

		user.WriteContext(c)
		c.Set("id", user.Id)
		temporaryToken := &model.Token{UserId: user.Id, Name: "image-studio", Group: group}
		if err := middleware.SetupContextForToken(c, temporaryToken); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
		c.Set(imageStudioBridgeUserKey, imageStudioBridgeUser{user: user, role: role, identity: expectedIdentity})
		c.Set("image_studio_authenticated", true)
		c.Next()
	}
}

func imageStudioGroupAllowed(userGroup, group string) bool {
	if group == "auto" {
		return service.GroupInUserUsableGroups(userGroup, group) && len(service.GetUserAutoGroup(userGroup)) > 0
	}
	return service.IsUserSelectableGroup(userGroup, group)
}

// ImageStudioGroupAuthorization applies group policy only to endpoints that
// expose group-scoped models or submit billable requests. Context and groups
// stay available while an operator repairs an invalid default group setting.
func ImageStudioGroupAuthorization() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get(imageStudioBridgeUserKey)
		bridgeUser, valid := value.(imageStudioBridgeUser)
		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		if !ok || !valid || bridgeUser.user == nil || !imageStudioGroupAllowed(bridgeUser.user.Group, group) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func GetImageStudioContext(c *gin.Context) {
	bridgeUser, ok := c.Get(imageStudioBridgeUserKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	session, ok := bridgeUser.(imageStudioBridgeUser)
	if !ok || session.user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if _, _, err := model.ClaimImageStudioCredit(session.user.Id); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":       session.user.Id,
		"username": sanitizeStudioName(session.user.Username, session.user.Id),
		"email":    session.identity,
		"group":    session.user.Group,
		"role":     session.role,
	})
}

func GetImageStudioModels(c *gin.Context) {
	ListModels(c, constant.ChannelTypeOpenAI)
}

func ImageStudioWriteOriginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			c.Next()
			return
		}
		config, err := loadImageStudioConfig()
		if err != nil || !imageStudioOriginMatches(origin, config.url) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func imageStudioOriginMatches(origin, configuredURL string) bool {
	originURL, err := url.Parse(origin)
	if err != nil || originURL.User != nil || originURL.RawQuery != "" || originURL.Fragment != "" || (originURL.Path != "" && originURL.Path != "/") {
		return false
	}
	configured, err := url.Parse(configuredURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(originURL.Scheme, configured.Scheme) && strings.EqualFold(originURL.Host, configured.Host)
}

// ImageStudioNormalizeRelayPath lets Distribute inspect the standard /v1 path
// before ImageStudioRelay changes it to /pg to skip temporary-token billing.
func ImageStudioNormalizeRelayPath() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, imageStudioBridgePrefix)
		if !strings.HasPrefix(path, "/v1/") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Request.URL.Path = path
		c.Request.URL.RawPath = path
		c.Next()
	}
}

func ImageStudioRelay(c *gin.Context, relayFormat types.RelayFormat) {
	prepareImageStudioRelay(c)
	Relay(c, relayFormat)
}

func prepareImageStudioRelay(c *gin.Context) {
	path := c.Request.URL.Path
	c.Set("relay_mode", relayconstant.Path2RelayMode(path))
	c.Request.URL.Path = "/pg" + strings.TrimPrefix(path, "/v1")
	c.Request.URL.RawPath = c.Request.URL.Path
	// RelayInfo canonicalizes playground paths from URL.String(), which must not
	// retain an absolute-form request target from a proxy or test client.
	c.Request.URL.Scheme = ""
	c.Request.URL.Host = ""
}
