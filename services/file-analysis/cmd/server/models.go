package main

import "time"

type WorkStatus string

const (
	StatusPending   WorkStatus = "pending"
	StatusCompleted WorkStatus = "completed"
	StatusFailed    WorkStatus = "failed"
)

type Work struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	StudentID    string     `json:"student_id" gorm:"index;not null"`
	AssignmentID string     `json:"assignment_id" gorm:"index;not null"`
	FileID       string     `json:"file_id" gorm:"not null"`
	Plagiarism   bool       `json:"plagiarism"`
	Status       WorkStatus `json:"status" gorm:"type:varchar(16);not null;default:'pending'"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}


