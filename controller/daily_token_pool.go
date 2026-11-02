package controller

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetDailyTokenPool(c *gin.Context) {
	config, err := model.GetDailyTokenPoolConfig()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Shared token pool is unavailable"})
		return
	}
	if !config.PublicVisible {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"enabled": false}})
		return
	}
	status, err := model.GetDailyTokenPoolStatus(config, time.Now())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Shared token pool is unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

func GetDailyTokenPoolAdmin(c *gin.Context) {
	config, err := model.GetDailyTokenPoolConfig()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Unable to load shared token pool settings"})
		return
	}
	var channels []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := model.DB.Model(&model.Channel{}).Select("id", "name").Order("id").Find(&channels).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Unable to load channels"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"config": config, "channels": channels}})
}

func UpdateDailyTokenPool(c *gin.Context) {
	current, err := model.GetDailyTokenPoolConfig()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Unable to load shared token pool settings"})
		return
	}
	// Older settings clients omit this new field; omission must not turn free
	// requests back into paid requests.
	config := model.DailyTokenPoolConfig{FreeUsage: current.FreeUsage}
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid shared token pool settings"})
		return
	}
	if err := config.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.SaveDailyTokenPoolConfig(config); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Unable to save shared token pool settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}
