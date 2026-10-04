package controller

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TavernCredit(c *gin.Context) {
	credit, err := model.GetTavernCredit(c.GetInt("id"))
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "Unable to read tavern credit"})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": credit})
}

func ClaimTavernCredit(c *gin.Context) {
	credit, granted, err := model.ClaimTavernCredit(c.GetInt("id"))
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "Unable to claim tavern credit"})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{
		"quota": credit.Quota, "granted_quota": credit.GrantedQuota,
		"created_time": credit.CreatedTime, "newly_granted": granted,
	}})
}
