package model

import "time"

type Project struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"type:varchar(128);not null"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (Project) TableName() string {
	return "projects"
}
