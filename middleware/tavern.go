package middleware

import (
	"crypto/subtle"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// TavernServiceAuth authenticates the loopback bridge, never a browser flag.
func TavernServiceAuth(required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.GetHeader("X-Tavern-Service-Key")
		if provided == "" && !required {
			c.Next()
			return
		}
		key, err := os.ReadFile(os.Getenv("TAVERN_SERVICE_KEY_FILE"))
		expected := strings.TrimSpace(string(key))
		if err != nil || len(expected) < 32 {
			c.AbortWithStatusJSON(503, gin.H{"success": false, "message": "Tavern service is unavailable"})
			return
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(403, gin.H{"success": false, "message": "Invalid tavern service"})
			return
		}
		c.Set("tavern_authenticated", true)
		c.Next()
	}
}
