package models

import "time"

// Teacher merepresentasikan profil guru (Epic 3 F3.2) yang terhubung satu-satu
// ke akun users ber-role guru_mapel.
type Teacher struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"column:user_id;not null" json:"user_id"`
	NIP       string    `gorm:"column:nip;size:30;not null" json:"nip"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`

	User     User      `gorm:"foreignKey:UserID;references:ID" json:"user"`
	Subjects []Subject `gorm:"many2many:teacher_subjects;foreignKey:ID;joinForeignKey:teacher_id;References:ID;joinReferences:subject_id" json:"subjects,omitempty"`
}

// TableName memetakan Teacher ke tabel "teachers".
func (Teacher) TableName() string {
	return "teachers"
}

// TeacherSubject adalah tabel pivot mapel yang diampu seorang guru.
type TeacherSubject struct {
	TeacherID int64     `gorm:"column:teacher_id;primaryKey" json:"teacher_id"`
	SubjectID int64     `gorm:"column:subject_id;primaryKey" json:"subject_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`

	Subject Subject `gorm:"foreignKey:SubjectID;references:ID" json:"subject,omitempty"`
}

// TableName memetakan TeacherSubject ke tabel "teacher_subjects".
func (TeacherSubject) TableName() string {
	return "teacher_subjects"
}
