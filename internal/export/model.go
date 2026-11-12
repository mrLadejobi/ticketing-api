package export

import (
	"time"

	"ticketing-api/internal/auth"
)

type BragSummary struct {
	TotalTasks      int     `json:"total_tasks"`
	CompletedTasks  int     `json:"completed_tasks"`
	InProgressTasks int     `json:"in_progress_tasks"`
	OnHoldTasks     int     `json:"on_hold_tasks"`
	FeaturesShipped int     `json:"features_shipped"`
	BugsResolved    int     `json:"bugs_resolved"`
	CompletionRate  float64 `json:"completion_rate"`
}

type ProjectStat struct {
	ProjectID      int     `json:"project_id"`
	ProjectName    string  `json:"project_name"`
	Prefix         string  `json:"prefix"`
	Total          int     `json:"total"`
	Completed      int     `json:"completed"`
	CompletionRate float64 `json:"completion_rate"`
}

type BragTaskItem struct {
	ID              int        `json:"id"`
	Key             string     `json:"key"`
	ProjectName     string     `json:"project_name"`
	TicketType      string     `json:"ticket_type"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	WorkSummary     string     `json:"work_summary"`
	Status          string     `json:"status"`
	Priority        string     `json:"priority"`
	ProgressPercent *int       `json:"progress_percent,omitempty"`
	DueDate         *time.Time `json:"due_date,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type BragReport struct {
	OrgName          string           `json:"org_name"`
	User             auth.UserSummary `json:"user"`
	StartDate        time.Time        `json:"start_date"`
	EndDate          time.Time        `json:"end_date"`
	GeneratedAt      time.Time        `json:"generated_at"`
	KeyInsight       string           `json:"key_insight"`
	Summary          BragSummary      `json:"summary"`
	TypeBreakdown    map[string]int   `json:"type_breakdown"`
	ProjectBreakdown []ProjectStat    `json:"project_breakdown"`
	CompletedWork    []BragTaskItem   `json:"completed_work"`
	ActiveWork       []BragTaskItem   `json:"active_work"`
}

