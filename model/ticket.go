package model

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TicketStatusOpen   = "open"
	TicketStatusClosed = "closed"
	TicketTitleMax     = 120
	TicketContentMax   = 10000
)

const (
	TicketReplyStatusUnreplied         = "unreplied"
	TicketReplyStatusReplied           = "replied"
	TicketReplyStatusFollowupUnreplied = "followup_unreplied"
	TicketReplyStatusFollowupReplied   = "followup_replied"
	TicketReplyStatusClosed            = "closed"
)

type Ticket struct {
	ID           int64  `json:"id" gorm:"primaryKey"`
	TicketNo     string `json:"ticket_no" gorm:"type:varchar(40);uniqueIndex"`
	UserID       int    `json:"user_id" gorm:"index"`
	Title        string `json:"title" gorm:"type:varchar(120)"`
	Status       string `json:"status" gorm:"type:varchar(16);index"`
	LastReadAt   int64  `json:"last_read_at"`
	ClosedAt     int64  `json:"closed_at"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"index"`
	AwaitingUser bool   `json:"awaiting_user" gorm:"column:awaiting_user;->"`
	Username     string `json:"username" gorm:"column:username;->"`
	ReplyStatus  string `json:"reply_status" gorm:"column:reply_status;->"`
}

func (Ticket) TableName() string { return "tickets" }

func (ticket *Ticket) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if ticket.CreatedAt == 0 {
		ticket.CreatedAt = now
	}
	if ticket.UpdatedAt == 0 {
		ticket.UpdatedAt = now
	}
	if ticket.Status == "" {
		ticket.Status = TicketStatusOpen
	}
	return nil
}

func (ticket *Ticket) BeforeUpdate(_ *gorm.DB) error {
	ticket.UpdatedAt = common.GetTimestamp()
	return nil
}

func GenerateTicketNo() (string, error) {
	key, err := common.GenerateRandomCharsKey(16)
	if err != nil {
		return "", err
	}
	return "T-" + key, nil
}

func CreateTicket(userID int, title string, content string) (*Ticket, *TicketMessage, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if userID <= 0 || title == "" || content == "" {
		return nil, nil, errors.New("title and content are required")
	}
	if len([]rune(title)) > TicketTitleMax || len([]rune(content)) > TicketContentMax {
		return nil, nil, errors.New("ticket content is too long")
	}
	no, err := GenerateTicketNo()
	if err != nil {
		return nil, nil, err
	}
	ticket := &Ticket{TicketNo: no, UserID: userID, Title: title, Status: TicketStatusOpen}
	message := &TicketMessage{SenderID: userID, SenderType: TicketSenderUser, Content: content}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		message.TicketID = ticket.ID
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		return tx.Create(&TicketEvent{TicketID: ticket.ID, EventType: TicketEventCreated, ActorID: userID}).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return ticket, message, nil
}

type TicketListOptions struct {
	Status       string
	ReplyStatus  string
	Keyword      string
	AwaitingUser *bool
	SortBy       string
	SortOrder    string
}

func ticketReplyStatusSQL() string {
	latestSender := "(SELECT sender_type FROM ticket_messages WHERE id = (SELECT MAX(id) FROM ticket_messages WHERE ticket_id = tickets.id))"
	firstAdmin := "(SELECT MIN(id) FROM ticket_messages WHERE ticket_id = tickets.id AND sender_type = 'admin')"
	followup := "EXISTS (SELECT 1 FROM ticket_messages followup WHERE followup.ticket_id = tickets.id AND followup.sender_type = 'user' AND followup.id > " + firstAdmin + ")"
	return "CASE WHEN tickets.status = 'closed' THEN 'closed' WHEN NOT EXISTS (SELECT 1 FROM ticket_messages admin_message WHERE admin_message.ticket_id = tickets.id AND admin_message.sender_type = 'admin') THEN 'unreplied' WHEN " + followup + " AND " + latestSender + " = 'user' THEN 'followup_unreplied' WHEN " + followup + " THEN 'followup_replied' ELSE 'replied' END"
}

func NewTicketListOptions(status, keyword string, awaitingUser *bool, sortBy, sortOrder string) TicketListOptions {
	return TicketListOptions{
		Status: status, Keyword: strings.TrimSpace(keyword), AwaitingUser: awaitingUser,
		SortBy: strings.ToLower(strings.TrimSpace(sortBy)), SortOrder: strings.ToLower(strings.TrimSpace(sortOrder)),
	}
}

func ListTickets(userID *int, pageInfo *common.PageInfo, options ...TicketListOptions) ([]Ticket, int64, error) {
	var tickets []Ticket
	query := DB.Model(&Ticket{})
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	var opts TicketListOptions
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.Status == TicketStatusOpen || opts.Status == TicketStatusClosed {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.ReplyStatus == TicketReplyStatusUnreplied || opts.ReplyStatus == TicketReplyStatusReplied || opts.ReplyStatus == TicketReplyStatusFollowupUnreplied || opts.ReplyStatus == TicketReplyStatusFollowupReplied || opts.ReplyStatus == TicketReplyStatusClosed {
		query = query.Where(ticketReplyStatusSQL()+" = ?", opts.ReplyStatus)
	}
	if opts.Keyword != "" {
		pattern := "%" + opts.Keyword + "%"
		query = query.Where("(title LIKE ? OR ticket_no LIKE ?)", pattern, pattern)
	}
	latestSenderSQL := "(SELECT sender_type FROM ticket_messages WHERE id = (SELECT MAX(id) FROM ticket_messages WHERE ticket_id = tickets.id))"
	awaitingCondition := fmt.Sprintf("tickets.status = ? AND %s = ?", latestSenderSQL)
	if opts.AwaitingUser != nil {
		if *opts.AwaitingUser {
			query = query.Where(awaitingCondition, TicketStatusOpen, TicketSenderAdmin)
		} else {
			query = query.Where("NOT ("+awaitingCondition+")", TicketStatusOpen, TicketSenderAdmin)
		}
	}
	query = query.Joins("LEFT JOIN users ON users.id = tickets.user_id").Select("tickets.*, users.username AS username, "+ticketReplyStatusSQL()+" AS reply_status, CASE WHEN tickets.status = ? AND "+latestSenderSQL+" = ? THEN 1 ELSE 0 END AS awaiting_user", TicketStatusOpen, TicketSenderAdmin)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	sortBy := opts.SortBy
	if sortBy != "created_at" && sortBy != "updated_at" && sortBy != "awaiting_user" {
		sortBy = "awaiting_user"
	}
	sortOrder := "desc"
	if opts.SortOrder == "asc" {
		sortOrder = "asc"
	}
	order := sortBy + " " + sortOrder
	if err := query.Order(order).Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&tickets).Error; err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

func DeleteTicket(id int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var ticket Ticket
		if err := tx.First(&ticket, "id = ?", id).Error; err != nil {
			return err
		}
		if ticket.Status != TicketStatusClosed {
			return errors.New("only closed tickets can be deleted")
		}
		if err := tx.Where("ticket_id = ?", id).Delete(&TicketMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("ticket_id = ?", id).Delete(&TicketEvent{}).Error; err != nil {
			return err
		}
		return tx.Delete(&ticket).Error
	})
}

func DeleteClosedTickets() (int64, error) {
	var deleted int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var ids []int64
		if err := tx.Model(&Ticket{}).Where("status = ?", TicketStatusClosed).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Where("ticket_id IN ?", ids).Delete(&TicketMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("ticket_id IN ?", ids).Delete(&TicketEvent{}).Error; err != nil {
			return err
		}
		result := tx.Where("id IN ? AND status = ?", ids, TicketStatusClosed).Delete(&Ticket{})
		deleted = result.RowsAffected
		return result.Error
	})
	return deleted, err
}

func AutoCloseStaleTickets(ctx context.Context, cutoff int64) (int64, error) {
	if cutoff <= 0 {
		return 0, errors.New("cutoff timestamp is required")
	}
	latestMessageID := DB.Model(&TicketMessage{}).Select("MAX(id)").Where("ticket_id = tickets.id")
	query := DB.WithContext(ctx).Model(&Ticket{}).
		Where("status = ?", TicketStatusOpen).
		Where("(SELECT sender_type FROM ticket_messages WHERE id = (?)) = ?", latestMessageID, TicketSenderAdmin).
		Where("(SELECT created_at FROM ticket_messages WHERE id = (?)) <= ?", latestMessageID, cutoff)
	var tickets []Ticket
	if err := query.Find(&tickets).Error; err != nil {
		return 0, err
	}
	var closed int64
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, ticket := range tickets {
			result := tx.Model(&Ticket{}).Where("id = ? AND status = ?", ticket.ID, TicketStatusOpen).Updates(map[string]any{
				"status": TicketStatusClosed, "closed_at": common.GetTimestamp(), "updated_at": common.GetTimestamp(),
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}
			if err := tx.Create(&TicketEvent{TicketID: ticket.ID, EventType: TicketEventClosed, ActorID: 0}).Error; err != nil {
				return err
			}
			closed++
		}
		return nil
	})
	return closed, err
}

func GetTicketByID(id int64) (*Ticket, error) {
	var ticket Ticket
	if err := DB.Model(&Ticket{}).Joins("LEFT JOIN users ON users.id = tickets.user_id").Select("tickets.*, users.username AS username, "+ticketReplyStatusSQL()+" AS reply_status").Where("tickets.id = ?", id).First(&ticket).Error; err != nil {
		return nil, err
	}
	return &ticket, nil
}

func GetTicketForUser(id int64, userID int) (*Ticket, error) {
	var ticket Ticket
	if err := DB.Model(&Ticket{}).Joins("LEFT JOIN users ON users.id = tickets.user_id").Select("tickets.*, users.username AS username, "+ticketReplyStatusSQL()+" AS reply_status").Where("tickets.id = ? AND tickets.user_id = ?", id, userID).First(&ticket).Error; err != nil {
		return nil, err
	}
	return &ticket, nil
}

func MarkTicketRead(id int64, userID int) error {
	result := DB.Model(&Ticket{}).Where("id = ? AND user_id = ?", id, userID).Update("last_read_at", common.GetTimestamp())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ListTicketMessages(ticketID int64) ([]TicketMessage, error) {
	var messages []TicketMessage
	err := DB.Where("ticket_id = ?", ticketID).Order("id asc").Find(&messages).Error
	return messages, err
}

func AddTicketMessage(ticket *Ticket, senderID int, senderType string, content string) (*TicketMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > TicketContentMax {
		return nil, errors.New("message is empty or too long")
	}
	if ticket.Status != TicketStatusOpen {
		return nil, errors.New("closed tickets cannot be replied to")
	}
	message := &TicketMessage{TicketID: ticket.ID, SenderID: senderID, SenderType: senderType, Content: content}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current Ticket
		if err := tx.First(&current, "id = ?", ticket.ID).Error; err != nil {
			return err
		}
		if current.Status != TicketStatusOpen {
			return errors.New("closed tickets cannot be replied to")
		}
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		if err := tx.Model(&Ticket{}).Where("id = ?", ticket.ID).Updates(map[string]any{"updated_at": common.GetTimestamp()}).Error; err != nil {
			return err
		}
		return tx.Create(&TicketEvent{TicketID: ticket.ID, EventType: TicketEventMessage, ActorID: senderID}).Error
	})
	return message, err
}

func CloseTicket(id int64, actorID int) error {
	result := DB.Model(&Ticket{}).Where("id = ? AND status = ?", id, TicketStatusOpen).Updates(map[string]any{
		"status":     TicketStatusClosed,
		"closed_at":  common.GetTimestamp(),
		"updated_at": common.GetTimestamp(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return DB.Create(&TicketEvent{TicketID: id, EventType: TicketEventClosed, ActorID: actorID}).Error
}

func DeleteTicketsByUserID(tx *gorm.DB, userID int) error {
	if !tx.Migrator().HasTable(&Ticket{}) {
		return nil
	}
	var ids []int64
	if err := tx.Model(&Ticket{}).Where("user_id = ?", userID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) > 0 {
		if err := tx.Where("ticket_id IN ?", ids).Delete(&TicketMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("ticket_id IN ?", ids).Delete(&TicketEvent{}).Error; err != nil {
			return err
		}
	}
	return tx.Where("user_id = ?", userID).Delete(&Ticket{}).Error
}
