package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

func SetImageStudioRouter(router *gin.Engine) {
	setImageStudioRouter(router, middleware.SystemPerformanceCheck())
}

func setImageStudioRouter(router *gin.Engine, performanceCheck gin.HandlerFunc) {
	studio := router.Group("/api/image-studio")
	studio.Use(middleware.RouteTag("api"))
	studio.GET("/context", controller.ImageStudioBridgeAuth(), middleware.DisableCache(), controller.GetImageStudioContext)
	studio.GET("/groups", controller.ImageStudioBridgeAuth(), middleware.DisableCache(), controller.GetUserGroups)

	// Account metadata must remain readable when relay admission is suspended.
	studio.Use(middleware.RouteTag("relay"), performanceCheck, controller.ImageStudioBridgeAuth())
	studio.GET("/v1/models", middleware.DisableCache(), controller.ImageStudioGroupAuthorization(), controller.GetImageStudioModels)

	relay := studio.Group("/v1")
	relay.Use(controller.ImageStudioGroupAuthorization(), controller.ImageStudioWriteOriginGuard(), controller.ImageStudioNormalizeRelayPath(), middleware.Distribute())
	relay.POST("/chat/completions", func(c *gin.Context) {
		controller.ImageStudioRelay(c, types.RelayFormatOpenAI)
	})
	relay.POST("/responses", func(c *gin.Context) {
		controller.ImageStudioRelay(c, types.RelayFormatOpenAIResponses)
	})
	relay.POST("/images/generations", func(c *gin.Context) {
		controller.ImageStudioRelay(c, types.RelayFormatOpenAIImage)
	})
	relay.POST("/images/edits", func(c *gin.Context) {
		controller.ImageStudioRelay(c, types.RelayFormatOpenAIImage)
	})
	relay.POST("/audio/speech", func(c *gin.Context) {
		controller.ImageStudioRelay(c, types.RelayFormatOpenAIAudio)
	})
	relay.POST("/audio/transcriptions", func(c *gin.Context) {
		controller.ImageStudioRelay(c, types.RelayFormatOpenAIAudio)
	})
	relay.POST("/audio/translations", func(c *gin.Context) {
		controller.ImageStudioRelay(c, types.RelayFormatOpenAIAudio)
	})
}
