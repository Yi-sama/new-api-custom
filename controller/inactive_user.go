package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetInactiveUserSettings(c *gin.Context) {
	days, err := model.GetInactiveUserDays()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"days": days}})
}

func SaveInactiveUserSettings(c *gin.Context) {
	var request struct {
		Days int `json:"days"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SaveInactiveUserDays(request.Days); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "inactive_users.settings", map[string]interface{}{"days": request.Days})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"days": request.Days}})
}

func GetInactiveUsers(c *gin.Context) {
	days, daysErr := strconv.Atoi(c.Query("days"))
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if daysErr != nil || pageErr != nil || sizeErr != nil {
		common.ApiErrorMsg(c, "Invalid inactivity days or pagination")
		return
	}
	data, err := model.ListInactiveUsers(c.Request.Context(), days, page, pageSize, common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func DeleteInactiveUsers(c *gin.Context) {
	var request struct {
		Days      int   `json:"days"`
		Cutoff    int64 `json:"cutoff"`
		Confirmed bool  `json:"confirmed"`
		IDs       []int `json:"ids"`
		All       bool  `json:"all"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ValidateInactiveUserDays(request.Days); err != nil {
		common.ApiError(c, err)
		return
	}
	if !request.Confirmed || request.Cutoff <= 0 ||
		request.Cutoff > common.GetTimestamp()-int64(request.Days)*86400 ||
		(request.All && len(request.IDs) > 0) || (!request.All && len(request.IDs) == 0) ||
		len(request.IDs) > 1000 {
		common.ApiErrorMsg(c, "Invalid cleanup confirmation or cutoff")
		return
	}
	for _, id := range request.IDs {
		if id <= 0 {
			common.ApiErrorMsg(c, "Invalid account ID")
			return
		}
	}
	ids := request.IDs
	if request.All {
		var err error
		ids, err = model.InactiveUserIDs(c.Request.Context(), request.Cutoff)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	deleted, skipped, failed := 0, 0, 0
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if c.Request.Context().Err() != nil {
			break
		}
		if id == c.GetInt("id") {
			skipped++
			continue
		}
		err := model.DeleteInactiveUser(id, request.Cutoff)
		switch {
		case err == nil:
			deleted++
			recordManageAuditFor(c, id, "user.delete.inactive", map[string]interface{}{
				"id": id, "days": request.Days, "cutoff": request.Cutoff,
			})
		case errors.Is(err, model.ErrUserNotInactive), errors.Is(err, gorm.ErrRecordNotFound):
			skipped++
		default:
			failed++
			common.SysError("inactive account cleanup failed for user " + strconv.Itoa(id) + ": " + err.Error())
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"deleted": deleted, "skipped": skipped, "failed": failed,
	}})
}
