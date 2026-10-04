package controller

import (
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/publicsuffix"
)

const (
	imageStudioCookieName        = "new_api_image_studio"
	defaultStudioIdentityFormat  = "new-api-{id}@{domain}"
	imageStudioUnauthorizedCache = "no-store, no-cache, must-revalidate, private, max-age=0"
)

type imageStudioConfig struct {
	url            string
	studioHost     string
	identityDomain string
	emailFormat    string
	cookieDomain   string
	secureCookie   bool
}

func ImageStudioEnabled() bool {
	_, err := loadImageStudioConfig()
	return err == nil
}

func clearImageStudioCookie(c *gin.Context) {
	config, err := loadImageStudioConfig()
	if err != nil {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     imageStudioCookieName,
		Value:    "",
		Path:     "/",
		Domain:   config.cookieDomain,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   config.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func CreateImageStudioSession(c *gin.Context) {
	config, err := loadImageStudioConfig()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": "IMAGE_STUDIO_DISABLED", "message": "image studio is not configured"})
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
	role, ok := imageStudioRole(user.Role)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "IMAGE_STUDIO_ROLE_INVALID", "message": "image studio role is invalid"})
		return
	}
	requestHost := requestHostname(c.Request.Host)
	invalidHostOnlyScope := config.cookieDomain == "" && requestHost != config.studioHost
	invalidDomainScope := config.cookieDomain != "" && (!domainMatches(requestHost, config.cookieDomain) || !domainMatches(config.studioHost, config.cookieDomain))
	if requestHost == "" || invalidHostOnlyScope || invalidDomainScope {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": "IMAGE_STUDIO_COOKIE_DOMAIN_INVALID", "message": "image studio cookie domain is invalid"})
		return
	}
	token, expiresAt, err := service.IssueImageStudioToken(identity, role, time.Unix(session.ExpiresAt, 0))
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     imageStudioCookieName,
		Value:    token,
		Path:     "/",
		Domain:   config.cookieDomain,
		MaxAge:   max(1, int(time.Until(time.Unix(expiresAt, 0)).Seconds())),
		HttpOnly: true,
		Secure:   config.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"url": strings.TrimSuffix(config.url, "/") + "/auth"}})
}

func GetImageStudioIdentity(c *gin.Context) {
	c.Header("Cache-Control", imageStudioUnauthorizedCache)
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	config, err := loadImageStudioConfig()
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	rawToken, err := c.Cookie(imageStudioCookieName)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	identity, issuedRole, err := service.ParseImageStudioToken(rawToken)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	_, user, err := service.ValidateLoginSession(identity)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	email, err := imageStudioEmail(config, user.Id)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	role, ok := imageStudioRole(user.Role)
	if !ok || role != issuedRole {
		c.Status(http.StatusUnauthorized)
		return
	}
	if _, _, err := model.ClaimImageStudioCredit(user.Id); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header("X-Studio-Email", email)
	c.Header("X-Studio-Name", url.PathEscape(sanitizeStudioName(user.Username, user.Id)))
	c.Header("X-Studio-Role", role)
	c.Status(http.StatusNoContent)
}

func GetImageStudioBilling(c *gin.Context) {
	c.Header("Cache-Control", imageStudioUnauthorizedCache)
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	rawToken, err := c.Cookie(imageStudioCookieName)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	identity, issuedRole, err := service.ParseImageStudioToken(rawToken)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	_, user, err := service.ValidateLoginSession(identity)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	role, ok := imageStudioRole(user.Role)
	if !ok || role != issuedRole {
		c.Status(http.StatusUnauthorized)
		return
	}
	credit, _, err := model.ClaimImageStudioCredit(user.Id)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	remaining, err := model.GetUserQuota(user.Id, false)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	used, err := model.GetUserUsedQuota(user.Id)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"remaining_quota":           remaining,
		"used_quota":                used,
		"studio_gift_quota":         credit.Quota,
		"studio_gift_granted_quota": credit.GrantedQuota,
		"display_type":              operation_setting.GetQuotaDisplayType(),
		"quota_per_unit":            common.QuotaPerUnit,
		"usd_exchange_rate":         operation_setting.USDExchangeRate,
	})
}

func loadImageStudioConfig() (imageStudioConfig, error) {
	rawURL := strings.TrimSpace(os.Getenv("IMAGE_STUDIO_URL"))
	parsedURL, err := url.Parse(rawURL)
	if err != nil || rawURL == "" || !parsedURL.IsAbs() || parsedURL.Hostname() == "" || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || parsedURL.Path != "" && parsedURL.Path != "/" {
		return imageStudioConfig{}, fmt.Errorf("invalid IMAGE_STUDIO_URL")
	}
	host := strings.ToLower(strings.TrimSuffix(parsedURL.Hostname(), "."))
	secure := parsedURL.Scheme == "https"
	if !secure && !(parsedURL.Scheme == "http" && isLocalhost(host)) {
		return imageStudioConfig{}, fmt.Errorf("IMAGE_STUDIO_URL must use HTTPS or localhost HTTP")
	}
	identityDomain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(os.Getenv("IMAGE_STUDIO_IDENTITY_DOMAIN")), "."))
	if !validDNSName(identityDomain) {
		return imageStudioConfig{}, fmt.Errorf("invalid IMAGE_STUDIO_IDENTITY_DOMAIN")
	}
	emailFormat := strings.TrimSpace(os.Getenv("IMAGE_STUDIO_IDENTITY_EMAIL_FORMAT"))
	if emailFormat == "" {
		emailFormat = defaultStudioIdentityFormat
	}
	if strings.Count(emailFormat, "{id}") != 1 || strings.Count(emailFormat, "{domain}") != 1 {
		return imageStudioConfig{}, fmt.Errorf("invalid IMAGE_STUDIO_IDENTITY_EMAIL_FORMAT")
	}
	cookieDomain := strings.ToLower(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(os.Getenv("IMAGE_STUDIO_COOKIE_DOMAIN")), "."), "."))
	if cookieDomain != "" {
		publicSuffix, _ := publicsuffix.PublicSuffix(cookieDomain)
		if !validDNSName(cookieDomain) || net.ParseIP(cookieDomain) != nil || isLocalhost(cookieDomain) || publicSuffix == cookieDomain {
			return imageStudioConfig{}, fmt.Errorf("invalid IMAGE_STUDIO_COOKIE_DOMAIN")
		}
	}
	config := imageStudioConfig{
		url:            rawURL,
		studioHost:     host,
		identityDomain: identityDomain,
		emailFormat:    emailFormat,
		cookieDomain:   cookieDomain,
		secureCookie:   secure,
	}
	if _, err := imageStudioEmail(config, 1); err != nil {
		return imageStudioConfig{}, err
	}
	return config, nil
}

func imageStudioEmail(config imageStudioConfig, userID int) (string, error) {
	if userID <= 0 {
		return "", fmt.Errorf("invalid user id")
	}
	email := strings.ReplaceAll(config.emailFormat, "{id}", strconv.Itoa(userID))
	email = strings.ReplaceAll(email, "{domain}", config.identityDomain)
	if strings.ContainsAny(email, "\r\n") {
		return "", fmt.Errorf("invalid image studio identity email")
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return "", fmt.Errorf("invalid image studio identity email")
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || !strings.EqualFold(email[at+1:], config.identityDomain) {
		return "", fmt.Errorf("image studio identity email domain mismatch")
	}
	return email, nil
}

func imageStudioRole(role int) (string, bool) {
	switch role {
	case common.RoleRootUser, common.RoleAdminUser:
		return "admin", true
	case common.RoleCommonUser:
		return "user", true
	default:
		return "", false
	}
}

func sanitizeStudioName(name string, userID int) string {
	name = strings.ToValidUTF8(name, "")
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, name)
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return fmt.Sprintf("User %d", userID)
	}
	return name
}

func requestHostname(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	parsed, err := url.Parse("//" + hostport)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
}

func domainMatches(host, domain string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func isLocalhost(host string) bool {
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func validDNSName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}
