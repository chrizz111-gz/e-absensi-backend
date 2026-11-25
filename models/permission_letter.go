package models

import "time"

// Konstanta tipe surat pulang/dispensasi pada tabel permission_letters (022).
const (
	PermissionLetterPulangTatib     = "pulang_tatib"
	PermissionLetterDispensasi      = "dispensasi"
	PermissionLetterTypePulangTatib = "pulang_tatib"
	PermissionLetterTypeDispensasi  = "dispensasi"
)

// PermissionLetter merepresentasikan surat pulang tatib atau surat dispensasi
// yang diunggah Siswa Inti (Epic 5 F5.8). Bersifat append-only (tanpa updated_at).
type PermissionLetter struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	StudentID       int64     `gorm:"column:student_id;not null" json:"student_id"`
	Type            string    `gorm:"column:type;size:20;not null" json:"type"`
	UploadedBy      int64     `gorm:"column:uploaded_by;not null" json:"uploaded_by"`
	EvidenceFileURL string    `gorm:"column:evidence_file_url;size:1000;not null" json:"evidence_file_url"`
	Note            *string   `gorm:"column:note;size:500" json:"note"`
	CreatedAt       time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`

	Student  Student `gorm:"foreignKey:StudentID;references:ID" json:"student,omitempty"`
	Uploader User    `gorm:"foreignKey:UploadedBy;references:ID" json:"uploader,omitempty"`
}

// TableName memetakan PermissionLetter ke tabel "permission_letters".
func (PermissionLetter) TableName() string {
	return "permission_letters"
}
