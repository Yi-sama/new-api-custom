package router

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

const videoStudioUnavailable = "video studio is not configured"

func SetVideoStudioRouter(router *gin.Engine) {
	router.GET("/video-canvas", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/video-canvas/")
	})
	router.GET("/video-canvas/*path", serveVideoStudioStatic)

	studio := router.Group("/api/video-studio")
	studio.Use(middleware.RouteTag("api"))
	studio.POST("/session", middleware.UserAuth(), middleware.DisableCache(), controller.CreateVideoStudioSession)
	studio.Use(controller.VideoStudioBridgeAuth())
	studio.GET("/context", middleware.DisableCache(), controller.GetVideoStudioContext)
	studio.GET("/catalog", middleware.DisableCache(), controller.GetVideoStudioCatalog)
	studio.GET("/v1/models", middleware.DisableCache(), controller.VideoStudioGroupAuthorization(), controller.GetVideoStudioModels)

	// Keep account metadata and completed-task retrieval available while relay
	// admission is suspended; only submissions enter the billable relay policy.
	billable := studio.Group("/v1")
	billable.Use(middleware.RouteTag("relay"), middleware.SystemPerformanceCheck(), controller.VideoStudioGroupAuthorization(), middleware.ModelRequestRateLimit())
	billable.POST("/chat/completions", controller.VideoStudioNormalizeRelayPath(), middleware.Distribute(), func(c *gin.Context) {
		controller.VideoStudioRelay(c, types.RelayFormatOpenAI)
	})
	billable.POST("/responses", controller.VideoStudioNormalizeRelayPath(), middleware.Distribute(), func(c *gin.Context) {
		controller.VideoStudioRelay(c, types.RelayFormatOpenAIResponses)
	})
	billable.POST("/images/generations", controller.VideoStudioNormalizeRelayPath(), middleware.Distribute(), func(c *gin.Context) {
		controller.VideoStudioRelay(c, types.RelayFormatOpenAIImage)
	})
	billable.POST("/images/edits", controller.VideoStudioNormalizeRelayPath(), middleware.Distribute(), func(c *gin.Context) {
		controller.VideoStudioRelay(c, types.RelayFormatOpenAIImage)
	})
	billable.POST("/audio/speech", controller.VideoStudioNormalizeRelayPath(), middleware.Distribute(), func(c *gin.Context) {
		controller.VideoStudioRelay(c, types.RelayFormatOpenAIAudio)
	})
	billable.POST("/videos", controller.VideoStudioNormalizeRelayPath(), middleware.Distribute(), controller.VideoStudioVideoModelAuthorization(), controller.VideoStudioRelayTask)

	read := studio.Group("/v1")
	read.Use(middleware.RouteTag("relay"))
	read.GET("/videos/:task_id", controller.VideoStudioNormalizeRelayPath(), controller.VideoStudioRelayTaskFetch)
	read.GET("/videos/:task_id/content", controller.VideoStudioVideoProxy)
}

func serveVideoStudioStatic(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	distDir := strings.TrimSpace(os.Getenv("VIDEO_STUDIO_DIST_DIR"))
	if distDir == "" {
		c.String(http.StatusServiceUnavailable, videoStudioUnavailable)
		return
	}
	requestPath := strings.TrimPrefix(c.Param("path"), "/")
	cleanPath := path.Clean("/" + requestPath)
	if strings.Contains(requestPath, "\\") || strings.Contains(cleanPath, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	filePath := filepath.Join(distDir, filepath.FromSlash(strings.TrimPrefix(cleanPath, "/")))
	if info, err := os.Stat(filePath); err == nil && info.Mode().IsRegular() {
		c.File(filePath)
		return
	}
	c.File(filepath.Join(distDir, "index.html"))
}
