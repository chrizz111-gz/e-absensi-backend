package models

import "time"

// Student merepresentasikan profil akademik siswa (Epic 3 F3.1).
// NISN disinkronkan dua arah dengan users.nisn lewat trigger DB.
type Student struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"column:user_id;not null" json:"user_id"`
	NISN      string    `gorm:"column:nisn;size:10;not null" json:"nisn"`
	ClassID   *int64    `gorm:"column:class_id" json:"class_id"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`

	User  User   `gorm:"foreignKey:UserID;references:ID" json:"user"`
	Class *Class `gorm:"foreignKey:ClassID;references:ID" json:"class,omitempty"`
}

// TableName memetakan Student ke tabel "students".
func (Student) TableName() string {
	return "students"
}
