package dto

type CreateTicketRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type TicketMessageRequest struct {
	Content string `json:"content"`
}
