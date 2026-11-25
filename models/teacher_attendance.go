package models

import "time"

// Konstanta status kehadiran guru pada tabel teacher_attendances (019).
const (
	TeacherAttendanceHadir               = "hadir"
	TeacherAttendanceGuruPengganti       = "guru_pengganti"
	TeacherAttendanceTidakHadir          = "tidak_hadir"
	TeacherAttendanceStatusHadir         = "hadir"
	TeacherAttendanceStatusGuruPengganti = "guru_pengganti"
	TeacherAttendanceStatusTidakHadir    = "tidak_hadir"
)

// Konstanta metode konfirmasi kehadiran guru (019 & 025).
const (
	ConfirmationMethodQRScan        = "qr_scan"
	ConfirmationMethodKodeCadangan  = "kode_cadangan"
	ConfirmationMethodKoreksiAdmin  = "koreksi_admin"
	TeacherConfirmationQRScan       = "qr_scan"
	TeacherConfirmationBackupCode   = "kode_cadangan"
	TeacherConfirmationKoreksiAdmin = "koreksi_admin"
)

// Konstanta role pembuat QR kehadiran guru (019).
const (
	TeacherQRGeneratedBySiswaInti = "siswa_inti"
	TeacherQRGeneratedByWaliKelas = "wali_kelas"
)

// TeacherAttendance merepresentasikan kehadiran guru per jadwal pelajaran (Epic 6).
// Satu baris per schedule_id (UNIQUE).
type TeacherAttendance struct {
	ID                  int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ScheduleID          int64      `gorm:"column:schedule_id;not null" json:"schedule_id"`
	Status              string     `gorm:"column:status;size:20;not null" json:"status"`
	ConfirmedTeacherID  *int64     `gorm:"column:confirmed_teacher_id" json:"confirmed_teacher_id"`
	ConfirmationMethod  *string    `gorm:"column:confirmation_method;size:20" json:"confirmation_method"`
	ConfirmedAt         *time.Time `gorm:"column:confirmed_at" json:"confirmed_at"`
	GeneratedByUserID   *int64     `gorm:"column:generated_by_user_id" json:"generated_by_user_id"`
	GeneratedByRole     *string    `gorm:"column:generated_by_role;size:20" json:"generated_by_role"`
	Keterangan          *string    `gorm:"column:keterangan" json:"keterangan"`
	KeteranganUpdatedBy *int64     `gorm:"column:keterangan_updated_by" json:"keterangan_updated_by,omitempty"`
	CreatedAt           time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`

	Schedule         Schedule `gorm:"foreignKey:ScheduleID;references:ID" json:"schedule,omitempty"`
	ConfirmedTeacher *Teacher `gorm:"foreignKey:ConfirmedTeacherID;references:ID" json:"confirmed_teacher,omitempty"`
	GeneratedByUser  *User    `gorm:"foreignKey:GeneratedByUserID;references:ID" json:"generated_by_user,omitempty"`
	KeteranganEditor *User    `gorm:"foreignKey:KeteranganUpdatedBy;references:ID" json:"keterangan_editor,omitempty"`
}

// TableName memetakan TeacherAttendance ke tabel "teacher_attendances".
func (TeacherAttendance) TableName() string {
	return "teacher_attendances"
}
