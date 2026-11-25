package models

import "time"

// AlphaDailySummary merepresentasikan ringkasan perhitungan Alpha harian per
// siswa per tanggal (Epic 5 F5.10, tabel alpha_daily_summaries pada 023).
// Rumus DB: is_alpha = (lesson_total > 0 AND lesson_alpha * 4 > lesson_total * 3).
type AlphaDailySummary struct {
	StudentID      int64     `gorm:"column:student_id;primaryKey" json:"student_id"`
	Date           time.Time `gorm:"column:date;type:date;primaryKey" json:"date"`
	AcademicYearID int64     `gorm:"column:academic_year_id;not null" json:"academic_year_id"`
	ClassID        int64     `gorm:"column:class_id;not null" json:"class_id"`
	LessonTotal    int16     `gorm:"column:lesson_total;not null" json:"lesson_total"`
	LessonAlpha    int16     `gorm:"column:lesson_alpha;not null" json:"lesson_alpha"`
	IsAlpha        bool      `gorm:"column:is_alpha;not null" json:"is_alpha"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

// TableName memetakan AlphaDailySummary ke tabel "alpha_daily_summaries".
func (AlphaDailySummary) TableName() string {
	return "alpha_daily_summaries"
}

// Ambang batas peringatan dini akumulasi hari Alpha per tahun ajaran (F5.10).
const (
	AlphaThresholdWarning3 int16 = 3
	AlphaThresholdWarning5 int16 = 5
	AlphaThresholdWarning9 int16 = 9
)

// AlphaWarningLog merepresentasikan log pemicu peringatan akumulasi hari Alpha
// pada ambang batas 3, 5, atau 9 hari per tahun ajaran (Epic 5 F5.10, 023).
type AlphaWarningLog struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	StudentID      int64     `gorm:"column:student_id;not null" json:"student_id"`
	AcademicYearID int64     `gorm:"column:academic_year_id;not null" json:"academic_year_id"`
	Threshold      int16     `gorm:"column:threshold;not null" json:"threshold"`
	AlphaDays      int16     `gorm:"column:alpha_days;not null" json:"alpha_days"`
	TriggeredAt    time.Time `gorm:"column:triggered_at;not null;autoCreateTime" json:"triggered_at"`

	Student Student `gorm:"foreignKey:StudentID;references:ID" json:"student,omitempty"`
}

// TableName memetakan AlphaWarningLog ke tabel "alpha_warning_logs".
func (AlphaWarningLog) TableName() string {
	return "alpha_warning_logs"
}
