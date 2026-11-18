package models

import "time"

// Konstanta tipe jadwal pada tabel schedules.
const (
	ScheduleTypeLesson   = "pelajaran"
	ScheduleTypeActivity = "kegiatan"
)

// Schedule merepresentasikan satu baris jadwal berbasis tanggal spesifik (Epic 4).
// Kolom DB memakai schedule_date dan schedule_type sesuai 016_schedules.sql.
type Schedule struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ScheduleDate time.Time `gorm:"column:schedule_date;type:date;not null" json:"date"`
	StartTime    string    `gorm:"column:start_time;type:time;not null" json:"start_time"`
	EndTime      string    `gorm:"column:end_time;type:time;not null" json:"end_time"`
	ClassID      int64     `gorm:"column:class_id;not null" json:"class_id"`
	SubjectID    int64     `gorm:"column:subject_id;not null" json:"subject_id"`
	TeacherID    int64     `gorm:"column:teacher_id;not null" json:"teacher_id"`
	ScheduleType string    `gorm:"column:schedule_type;size:10;not null;default:pelajaran" json:"type"`
	ActivityName *string   `gorm:"column:activity_name;size:150" json:"activity_name,omitempty"`
	CreatedBy    *int64    `gorm:"column:created_by" json:"created_by,omitempty"`
	Version      int       `gorm:"column:version;not null;default:1" json:"version"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`

	Class   *Class   `gorm:"foreignKey:ClassID;references:ID" json:"class,omitempty"`
	Subject *Subject `gorm:"foreignKey:SubjectID;references:ID" json:"subject,omitempty"`
	Teacher *Teacher `gorm:"foreignKey:TeacherID;references:ID" json:"teacher,omitempty"`
}

// TableName memetakan Schedule ke tabel "schedules".
func (Schedule) TableName() string {
	return "schedules"
}
