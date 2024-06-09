package model

import "time"

type Task struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Title     string    `json:"title" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdateAt  time.Time `json:"update_at"`
	User      User      `json:"user" gorm:"foreignkey:UserId; constraint:OnDelete:CASCADE"`
	UserId    uint      `json:"user_id" gorm:"not null"`
}

type TaskResponse struct {
	ID       uint      `json:"id" gorm:"primaryKey"`
	Title    string    `json:"title" gorm:"not null"`
	CreateAt time.Time `json:"create_at"`
	UpdateAt time.Time `json:"update_at"`
}
