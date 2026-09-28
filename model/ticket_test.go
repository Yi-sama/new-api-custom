package model

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTicketTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Ticket{}, &TicketMessage{}, &TicketEvent{}))
	DB.Exec("DELETE FROM ticket_events")
	DB.Exec("DELETE FROM ticket_messages")
	DB.Exec("DELETE FROM tickets")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM ticket_events")
		DB.Exec("DELETE FROM ticket_messages")
		DB.Exec("DELETE FROM tickets")
	})
}

func TestCreateTicketAndAppendMessage(t *testing.T) {
	setupTicketTest(t)
	ticket, first, err := CreateTicket(41, "Login issue", "I cannot sign in")
	require.NoError(t, err)
	assert.Equal(t, TicketStatusOpen, ticket.Status)
	assert.Equal(t, TicketSenderUser, first.SenderType)

	second, err := AddTicketMessage(ticket, 41, TicketSenderUser, "Additional browser details")
	require.NoError(t, err)
	assert.Equal(t, ticket.ID, second.TicketID)

	messages, err := ListTicketMessages(ticket.ID)
	require.NoError(t, err)
	assert.Len(t, messages, 2)
}

func TestClosedTicketRejectsMessagesAndUserIsolation(t *testing.T) {
	setupTicketTest(t)
	ticket, _, err := CreateTicket(41, "Billing", "Please check my balance")
	require.NoError(t, err)
	require.NoError(t, CloseTicket(ticket.ID, 99))
	_, err = AddTicketMessage(ticket, 41, TicketSenderUser, "follow-up")
	assert.Error(t, err)

	_, err = GetTicketForUser(ticket.ID, 42)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestListTicketsFiltersAwaitingUserAndSortsByRecentUpdate(t *testing.T) {
	setupTicketTest(t)
	first, _, err := CreateTicket(41, "first", "question")
	require.NoError(t, err)
	second, _, err := CreateTicket(42, "second", "question")
	require.NoError(t, err)
	_, err = AddTicketMessage(first, 99, TicketSenderAdmin, "please provide logs")
	require.NoError(t, err)
	awaiting := true
	page := &common.PageInfo{Page: 1, PageSize: 20}
	tickets, total, err := ListTickets(nil, page, NewTicketListOptions("", "", &awaiting, "updated_at", "desc"))
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, tickets, 1)
	assert.Equal(t, first.ID, tickets[0].ID)
	assert.True(t, tickets[0].AwaitingUser)
	assert.NotEqual(t, second.ID, tickets[0].ID)
}

func TestListTicketsReplyStatuses(t *testing.T) {
	setupTicketTest(t)
	page := &common.PageInfo{Page: 1, PageSize: 20}

	_, _, err := CreateTicket(41, "unreplied", "question")
	require.NoError(t, err)

	replied, _, err := CreateTicket(42, "replied", "question")
	require.NoError(t, err)
	_, err = AddTicketMessage(replied, 99, TicketSenderAdmin, "answer")
	require.NoError(t, err)

	followupUnreplied, _, err := CreateTicket(43, "followup-unreplied", "question")
	require.NoError(t, err)
	_, err = AddTicketMessage(followupUnreplied, 99, TicketSenderAdmin, "answer")
	require.NoError(t, err)
	_, err = AddTicketMessage(followupUnreplied, 43, TicketSenderUser, "more details")
	require.NoError(t, err)

	followupReplied, _, err := CreateTicket(44, "followup-replied", "question")
	require.NoError(t, err)
	_, err = AddTicketMessage(followupReplied, 99, TicketSenderAdmin, "answer")
	require.NoError(t, err)
	_, err = AddTicketMessage(followupReplied, 44, TicketSenderUser, "more details")
	require.NoError(t, err)
	_, err = AddTicketMessage(followupReplied, 99, TicketSenderAdmin, "follow-up answer")
	require.NoError(t, err)

	closed, _, err := CreateTicket(45, "closed", "question")
	require.NoError(t, err)
	require.NoError(t, CloseTicket(closed.ID, 99))

	cases := []struct {
		name   string
		status string
	}{
		{name: "unreplied", status: TicketReplyStatusUnreplied},
		{name: "replied", status: TicketReplyStatusReplied},
		{name: "followup-unreplied", status: TicketReplyStatusFollowupUnreplied},
		{name: "followup-replied", status: TicketReplyStatusFollowupReplied},
		{name: "closed", status: TicketReplyStatusClosed},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tickets, total, listErr := ListTickets(nil, page, TicketListOptions{ReplyStatus: testCase.status})
			require.NoError(t, listErr)
			require.Equal(t, int64(1), total)
			require.Len(t, tickets, 1)
			assert.Equal(t, testCase.status, tickets[0].ReplyStatus)
		})
	}
}

func TestListTicketsSortsByUpdatedTimeWithoutAwaitingPriority(t *testing.T) {
	setupTicketTest(t)
	awaiting, _, err := CreateTicket(41, "awaiting", "question")
	require.NoError(t, err)
	_, err = AddTicketMessage(awaiting, 99, TicketSenderAdmin, "please reply")
	require.NoError(t, err)

	recent, _, err := CreateTicket(42, "recent", "question")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&Ticket{}).Where("id = ?", awaiting.ID).UpdateColumn("updated_at", int64(100)).Error)
	require.NoError(t, DB.Model(&Ticket{}).Where("id = ?", recent.ID).UpdateColumn("updated_at", int64(200)).Error)

	page := &common.PageInfo{Page: 1, PageSize: 20}
	tickets, total, err := ListTickets(nil, page, TicketListOptions{SortBy: "updated_at", SortOrder: "desc"})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, tickets, 2)
	assert.Equal(t, recent.ID, tickets[0].ID)
}

func TestAutoCloseStaleTicketsOnlyClosesWhenAdminMessageIsLatest(t *testing.T) {
	setupTicketTest(t)
	cutoff := common.GetTimestamp() - int64((24 * time.Hour).Seconds())
	stale, _, err := CreateTicket(41, "stale", "question")
	require.NoError(t, err)
	adminMessage, err := AddTicketMessage(stale, 99, TicketSenderAdmin, "please reply")
	require.NoError(t, err)
	require.NoError(t, DB.Model(adminMessage).Update("created_at", cutoff-1).Error)

	active, _, err := CreateTicket(42, "active", "question")
	require.NoError(t, err)
	_, err = AddTicketMessage(active, 99, TicketSenderAdmin, "please reply")
	require.NoError(t, err)
	_, err = AddTicketMessage(active, 42, TicketSenderUser, "here are the details")
	require.NoError(t, err)

	closed, err := AutoCloseStaleTickets(context.Background(), cutoff)
	require.NoError(t, err)
	assert.Equal(t, int64(1), closed)

	var staleReloaded, activeReloaded Ticket
	require.NoError(t, DB.First(&staleReloaded, stale.ID).Error)
	require.NoError(t, DB.First(&activeReloaded, active.ID).Error)
	assert.Equal(t, TicketStatusClosed, staleReloaded.Status)
	assert.Equal(t, TicketStatusOpen, activeReloaded.Status)
}

func TestDeleteClosedTicketsRemovesMessagesAndEvents(t *testing.T) {
	setupTicketTest(t)
	ticket, _, err := CreateTicket(41, "closed", "question")
	require.NoError(t, err)
	require.NoError(t, CloseTicket(ticket.ID, 99))
	deleted, err := DeleteClosedTickets()
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	assert.ErrorIs(t, DB.First(&Ticket{}, ticket.ID).Error, gorm.ErrRecordNotFound)
	var messageCount, eventCount int64
	require.NoError(t, DB.Model(&TicketMessage{}).Where("ticket_id = ?", ticket.ID).Count(&messageCount).Error)
	require.NoError(t, DB.Model(&TicketEvent{}).Where("ticket_id = ?", ticket.ID).Count(&eventCount).Error)
	assert.Zero(t, messageCount)
	assert.Zero(t, eventCount)
}
