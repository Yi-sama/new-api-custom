package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TicketSenderUser  = "user"
	TicketSenderAdmin = "admin"
)

type TicketMessage struct {
	ID         int64  `json:"id" gorm:"primaryKey"`
	TicketID   int64  `json:"ticket_id" gorm:"index"`
	SenderID   int    `json:"sender_id"`
	SenderType string `json:"sender_type" gorm:"type:varchar(16);index"`
	Content    string `json:"content" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"index"`
}

func (message *TicketMessage) BeforeCreate(_ *gorm.DB) error {
	if message.CreatedAt == 0 {
		message.CreatedAt = common.GetTimestamp()
	}
	return nil
}
