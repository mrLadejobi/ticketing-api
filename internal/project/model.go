package project

import (
	"time"
)

type Status string

const (
	StatusPlanned   Status = "PLANNED"
	StatusActive    Status = "ACTIVE"
	StatusPaused    Status = "PAUSED"
	StatusCompleted Status = "COMPLETED"
)

type Project struct {
	ID           int       `json:"id"`
	TeamID       int       `json:"team_id"`
	Name         string    `json:"name"`
	Prefix       string    `json:"prefix"`
	Description  string    `json:"description"`
	IssueCounter int       `json:"issue_counter"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateProjectRequest struct {
	TeamID      int    `json:"team_id" binding:"required,min=1"`
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Prefix      string `json:"prefix" binding:"required,min=2,max=5"`
	Description string `json:"description"`
	Status      Status `json:"status" binding:"omitempty,oneof=PLANNED ACTIVE PAUSED COMPLETED"`
}

type BoardColumn struct {
	Status  string `json:"status"`
	Count   int    `json:"count"`
	Tickets any    `json:"tickets"`
}

type BoardResponse struct {
	Project Project       `json:"project"`
	Columns []BoardColumn `json:"columns"`
}

