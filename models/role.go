package models

import "time"

// Role adalah master role sistem (7 baris seed). Referensi tetap: baris tidak
// dihapus selama masih dipakai (FK user_roles ON DELETE RESTRICT).
type Role struct {
	ID          int16     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;size:50;not null" json:"name"`
	Description *string   `gorm:"column:description;size:255" json:"description,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
}

// TableName memetakan Role ke tabel "roles".
func (Role) TableName() string {
	return "roles"
}

// Konstanta nama role sesuai seed pada 001_epic1_auth.sql.
const (
	RoleAdmin         = "admin"
	RoleWakaKurikulum = "waka_kurikulum"
	RoleWakaKesiswaan = "waka_kesiswaan"
	RoleGuruMapel     = "guru_mapel"
	RoleWaliKelas     = "wali_kelas"
	RoleSiswaInti     = "siswa_inti"
	RoleSiswa         = "siswa"
)
