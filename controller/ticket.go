package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ticketID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

func GetTickets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userID := c.GetInt("id")
	var filter *int
	if c.GetInt("role") < common.RoleAdminUser {
		filter = &userID
	}
	var awaitingUser *bool
	if raw := c.Query("awaiting_user"); raw != "" {
		value, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			common.ApiError(c, errors.New("invalid awaiting_user"))
			return
		}
		awaitingUser = &value
	}
	options := model.NewTicketListOptions(c.Query("status"), c.Query("keyword"), awaitingUser, c.Query("sort_by"), c.Query("sort_order"))
	options.ReplyStatus = c.Query("reply_status")
	tickets, total, err := model.ListTickets(filter, pageInfo, options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tickets)
	common.ApiSuccess(c, pageInfo)
}

func DeleteTicket(c *gin.Context) {
	id, err := ticketID(c)
	if err != nil {
		common.ApiError(c, errors.New("invalid ticket id"))
		return
	}
	if err := model.DeleteTicket(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "ticket not found"})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func DeleteClosedTickets(c *gin.Context) {
	deleted, err := model.DeleteClosedTickets()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"deleted": deleted})
}

func CreateTicket(c *gin.Context) {
	var req dto.CreateTicketRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, errors.New("invalid request"))
		return
	}
	ticket, _, err := model.CreateTicket(c.GetInt("id"), req.Title, req.Content)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, ticket)
}

func GetTicket(c *gin.Context) {
	id, err := ticketID(c)
	if err != nil {
		common.ApiError(c, errors.New("invalid ticket id"))
		return
	}
	var ticket *model.Ticket
	if c.GetInt("role") >= common.RoleAdminUser {
		ticket, err = model.GetTicketByID(id)
	} else {
		ticket, err = model.GetTicketForUser(id, c.GetInt("id"))
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "ticket not found"})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	messages, err := model.ListTicketMessages(ticket.ID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if c.GetInt("role") < common.RoleAdminUser {
		_ = model.MarkTicketRead(ticket.ID, c.GetInt("id"))
	}
	common.ApiSuccess(c, gin.H{"ticket": ticket, "messages": messages})
}

func AddTicketMessage(c *gin.Context) {
	id, err := ticketID(c)
	if err != nil {
		common.ApiError(c, errors.New("invalid ticket id"))
		return
	}
	var req dto.TicketMessageRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, errors.New("invalid request"))
		return
	}
	var ticket *model.Ticket
	if c.GetInt("role") >= common.RoleAdminUser {
		ticket, err = model.GetTicketByID(id)
	} else {
		ticket, err = model.GetTicketForUser(id, c.GetInt("id"))
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	senderType := model.TicketSenderUser
	if c.GetInt("role") >= common.RoleAdminUser {
		senderType = model.TicketSenderAdmin
	}
	message, err := model.AddTicketMessage(ticket, c.GetInt("id"), senderType, req.Content)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, message)
}

func MarkTicketRead(c *gin.Context) {
	id, err := ticketID(c)
	if err != nil {
		common.ApiError(c, errors.New("invalid ticket id"))
		return
	}
	if err := model.MarkTicketRead(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func CloseTicket(c *gin.Context) {
	id, err := ticketID(c)
	if err != nil {
		common.ApiError(c, errors.New("invalid ticket id"))
		return
	}
	if err := model.CloseTicket(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
