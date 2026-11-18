package models

import "time"

// Semester konstanta nilai active_semester pada academic_years.
const (
	Semester1 = "semester_1"
	Semester2 = "semester_2"
)

// AcademicYear merepresentasikan tahun ajaran dan semester aktif (Epic 3 F3.5).
// Partial unique index uq_academic_years_active menjamin maksimal 1 baris aktif.
type AcademicYear struct {
	ID             int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name           string     `gorm:"column:name;size:20;not null" json:"name"`
	ActiveSemester string     `gorm:"column:active_semester;size:12;not null" json:"active_semester"`
	StartDate      time.Time  `gorm:"column:start_date;type:date;not null" json:"start_date"`
	EndDate        *time.Time `gorm:"column:end_date;type:date" json:"end_date,omitempty"`
	IsActive       bool       `gorm:"column:is_active;not null;default:false" json:"is_active"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

// TableName memetakan AcademicYear ke tabel "academic_years".
func (AcademicYear) TableName() string {
	return "academic_years"
}
