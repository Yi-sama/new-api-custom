package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TicketEventCreated = "created"
	TicketEventMessage = "message"
	TicketEventClosed  = "closed"
)

type TicketEvent struct {
	ID        int64  `json:"id" gorm:"primaryKey"`
	TicketID  int64  `json:"ticket_id" gorm:"index"`
	EventType string `json:"event_type" gorm:"type:varchar(16);index"`
	ActorID   int    `json:"actor_id"`
	CreatedAt int64  `json:"created_at" gorm:"index"`
}

func (event *TicketEvent) BeforeCreate(_ *gorm.DB) error {
	if event.CreatedAt == 0 {
		event.CreatedAt = common.GetTimestamp()
	}
	return nil
}
