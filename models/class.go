package models

import "time"

// Class merepresentasikan master kelas per tahun ajaran (Epic 3 F3.3).
// WaliKelasUserID bersifat read-only di Epic 3 dan dikelola di Epic 9.
type Class struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name            string    `gorm:"column:name;size:50;not null" json:"name"`
	GradeLevel      int16     `gorm:"column:grade_level;not null" json:"grade_level"`
	AcademicYearID  int64     `gorm:"column:academic_year_id;not null" json:"academic_year_id"`
	WaliKelasUserID *int64    `gorm:"column:wali_kelas_user_id" json:"wali_kelas_user_id,omitempty"`
	IsActive        bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`

	AcademicYear *AcademicYear `gorm:"foreignKey:AcademicYearID;references:ID" json:"academic_year,omitempty"`
	WaliKelas    *User         `gorm:"foreignKey:WaliKelasUserID;references:ID" json:"wali_kelas,omitempty"`
}

// TableName memetakan Class ke tabel "classes".
func (Class) TableName() string {
	return "classes"
}
