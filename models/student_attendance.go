package models

import "time"

// Konstanta status absensi siswa pada tabel student_attendances (020).
const (
	AttendanceStatusHadir              = "hadir"
	AttendanceStatusIzin               = "izin"
	AttendanceStatusSakit              = "sakit"
	AttendanceStatusAlpha              = "alpha"
	AttendanceStatusDispensasi         = "dispensasi"
	StudentAttendanceStatusHadir       = "hadir"
	StudentAttendanceStatusIzin        = "izin"
	StudentAttendanceStatusSakit       = "sakit"
	StudentAttendanceStatusAlpha       = "alpha"
	StudentAttendanceStatusDispensasi  = "dispensasi"
)

// Konstanta source pencatatan absensi siswa (020).
const (
	AttendanceSourceCeklisGuru                  = "ceklis_guru"
	AttendanceSourceFallbackWaliKelas           = "fallback_wali_kelas"
	AttendanceSourceAutoFromLeaveRequest        = "auto_from_leave_request"
	AttendanceSourceManualEdit                  = "manual_edit"
	StudentAttendanceSourceCeklisGuru           = "ceklis_guru"
	StudentAttendanceSourceFallbackWaliKelas    = "fallback_wali_kelas"
	StudentAttendanceSourceAutoFromLeaveRequest = "auto_from_leave_request"
	StudentAttendanceSourceManualEdit           = "manual_edit"
)

// StudentAttendance merepresentasikan satu baris absensi siswa per jadwal (Epic 5).
// class_id dan academic_year_id adalah snapshot yang diisi otomatis oleh trigger DB.
type StudentAttendance struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ScheduleID     int64     `gorm:"column:schedule_id;not null" json:"schedule_id"`
	StudentID      int64     `gorm:"column:student_id;not null" json:"student_id"`
	ClassID        int64     `gorm:"column:class_id;not null" json:"class_id"`
	AcademicYearID int64     `gorm:"column:academic_year_id;not null" json:"academic_year_id"`
	Status         string    `gorm:"column:status;size:12;not null" json:"status"`
	Source         string    `gorm:"column:source;size:24;not null" json:"source"`
	RecordedBy     int64     `gorm:"column:recorded_by;not null" json:"recorded_by"`
	Note           *string   `gorm:"column:note;size:500" json:"note"`
	Version        int       `gorm:"column:version;not null;default:1" json:"version"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`

	Schedule Schedule `gorm:"foreignKey:ScheduleID;references:ID" json:"schedule,omitempty"`
	Student  Student  `gorm:"foreignKey:StudentID;references:ID" json:"student,omitempty"`
	Class    Class    `gorm:"foreignKey:ClassID;references:ID" json:"class,omitempty"`
}

// TableName memetakan StudentAttendance ke tabel "student_attendances".
func (StudentAttendance) TableName() string {
	return "student_attendances"
}
