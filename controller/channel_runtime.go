package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetChannelRuntime returns process-local channel/key load and latency metrics.
// It is diagnostic data only; inflight state is intentionally not persisted.
func GetChannelRuntime(c *gin.Context) {
	common.ApiSuccess(c, gin.H{"items": model.GetChannelRuntimeSnapshots()})
}
