package models

import "time"

// Subject merepresentasikan master mata pelajaran (Epic 3 F3.4).
type Subject struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Code      string    `gorm:"column:code;size:20;not null" json:"code"`
	Name      string    `gorm:"column:name;size:100;not null" json:"name"`
	IsActive  bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

// TableName memetakan Subject ke tabel "subjects".
func (Subject) TableName() string {
	return "subjects"
}
