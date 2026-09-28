package router

import (
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

const imageWorkbenchUnavailable = "image workbench is not configured"

var errInvalidImageWorkbenchURL = errors.New("IMAGE_WORKBENCH_HISTORY_URL must be a local HTTP origin")

func SetImageWorkbenchRouter(router *gin.Engine) {
	distDir := strings.TrimSpace(os.Getenv("IMAGE_WORKBENCH_DIST_DIR"))
	historyURL, historyErr := localImageWorkbenchURL(os.Getenv("IMAGE_WORKBENCH_HISTORY_URL"))
	var historyProxy *httputil.ReverseProxy
	if historyErr == nil {
		historyProxy = httputil.NewSingleHostReverseProxy(historyURL)
	}

	router.GET("/studio", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/studio/")
	})
	router.GET("/studio/*path", func(c *gin.Context) {
		if distDir == "" {
			c.String(http.StatusServiceUnavailable, imageWorkbenchUnavailable)
			return
		}
		if c.Param("path") == "/" {
			c.File(filepath.Join(distDir, "studio.html"))
			return
		}
		http.StripPrefix("/studio", http.FileServer(http.Dir(distDir))).ServeHTTP(c.Writer, c.Request)
	})

	router.Any("/studio-api/*path", func(c *gin.Context) {
		if historyErr != nil {
			c.String(http.StatusServiceUnavailable, imageWorkbenchUnavailable)
			return
		}
		historyProxy.ServeHTTP(c.Writer, c.Request)
	})
}

func localImageWorkbenchURL(rawURL string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || target.Scheme != "http" || target.Host == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" || (target.Path != "" && target.Path != "/") {
		return nil, &url.Error{Op: "parse", URL: rawURL, Err: errInvalidImageWorkbenchURL}
	}

	hostname := target.Hostname()
	ip := net.ParseIP(hostname)
	if !strings.EqualFold(hostname, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return nil, &url.Error{Op: "parse", URL: rawURL, Err: errInvalidImageWorkbenchURL}
	}
	target.Path = ""
	return target, nil
}
