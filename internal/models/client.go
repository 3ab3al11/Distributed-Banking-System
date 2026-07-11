package models

import "time"

type Client struct {
	ID         int     `json:"id"`
	Name       string  `json:"name" binding:"required,max=100"`
	NationalID string  `json:"national_id" binding:"required,max=14"`
	Phone      string  `json:"phone" binding:"required,max=15"`
	Email      string  `json:"email" binding:"required,email,max=150"`
	Balance    float64 `json:"balance" binding:"gte=0"`
}

type CreateClientRequest struct {
	Name       string  `json:"name" binding:"required,max=100"`
	NationalID string  `json:"national_id" binding:"required,max=14"`
	Phone      string  `json:"phone" binding:"required,max=15"`
	Email      string  `json:"email" binding:"required,email,max=150"`
	Balance    float64 `json:"balance" binding:"gte=0"`
}

type UpdateClientRequest struct {
	Name       string  `json:"name" binding:"required,max=100"`
	NationalID string  `json:"national_id" binding:"required,max=14"`
	Phone      string  `json:"phone" binding:"required,max=15"`
	Email      string  `json:"email" binding:"required,email,max=150"`
	Balance    float64 `json:"balance" binding:"gte=0"`
}

type ApproveWriteRequest struct {
	Operation  string              `json:"operation" binding:"required,oneof=create update delete"`
	SourceNode string              `json:"source_node" binding:"required,max=50"`
	ClientID   int                 `json:"client_id"`
	Client     CreateClientRequest `json:"client"`
}

type ApproveWriteResponse struct {
	Approved  bool   `json:"approved"`
	Pending   bool   `json:"pending,omitempty"`
	RequestID int    `json:"request_id,omitempty"`
	Message   string `json:"message"`
}

type PendingWriteRequest struct {
	RequestID  int                 `json:"request_id"`
	Operation  string              `json:"operation"`
	SourceNode string              `json:"source_node"`
	ClientID   int                 `json:"client_id"`
	Client     CreateClientRequest `json:"client"`
	Status     string              `json:"status"`
	CreatedAt  time.Time           `json:"created_at"`
}

type ReplicationRequest struct {
	Operation string `json:"operation" binding:"required,oneof=insert update delete"`
	Client    Client `json:"client" binding:"required"`
}
