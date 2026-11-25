package models

import "time"

// Konstanta tipe dan status pengajuan izin/sakit pada tabel leave_requests (021).
const (
	LeaveTypeIzin        = "izin"
	LeaveTypeSakit       = "sakit"
	LeaveRequestTypeIzin  = "izin"
	LeaveRequestTypeSakit = "sakit"

	LeaveStatusDraft             = "draft"
	LeaveStatusDisetujui         = "disetujui"
	LeaveStatusDitolak           = "ditolak"
	LeaveRequestStatusDraft      = "draft"
	LeaveRequestStatusDisetujui  = "disetujui"
	LeaveRequestStatusDitolak    = "ditolak"
)

// LeaveRequest merepresentasikan pengajuan Form Izin Tidak Masuk / Form Izin
// Sakit satu hari oleh siswa (Epic 5 F5.6 & F5.7).
type LeaveRequest struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	StudentID       int64      `gorm:"column:student_id;not null" json:"student_id"`
	ClassID         int64      `gorm:"column:class_id;not null" json:"class_id"`
	Type            string     `gorm:"column:type;size:10;not null" json:"type"`
	Date            time.Time  `gorm:"column:date;type:date;not null" json:"date"`
	Reason          string     `gorm:"column:reason;size:500;not null" json:"reason"`
	EvidenceFileURL string     `gorm:"column:evidence_file_url;size:1000;not null" json:"evidence_file_url"`
	Status          string     `gorm:"column:status;size:12;not null;default:draft" json:"status"`
	ReviewedBy      *int64     `gorm:"column:reviewed_by" json:"reviewed_by"`
	ReviewedAt      *time.Time `gorm:"column:reviewed_at" json:"reviewed_at"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`

	Student  Student `gorm:"foreignKey:StudentID;references:ID" json:"student,omitempty"`
	Class    Class   `gorm:"foreignKey:ClassID;references:ID" json:"class,omitempty"`
	Reviewer *User   `gorm:"foreignKey:ReviewedBy;references:ID" json:"reviewer,omitempty"`
}

// TableName memetakan LeaveRequest ke tabel "leave_requests".
func (LeaveRequest) TableName() string {
	return "leave_requests"
}
