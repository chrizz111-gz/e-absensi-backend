package services

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"e-absensi-backend/models"
)

// Kode event kanonik Epic 2 pada tabel activity_logs.
const (
	ActionAccountCreated     = "ACCOUNT_CREATED"
	ActionAccountUpdated     = "ACCOUNT_UPDATED"
	ActionAccountDeactivated = "ACCOUNT_DEACTIVATED"
	ActionAccountActivated   = "ACCOUNT_ACTIVATED"
	ActionRoleAssigned       = "ROLE_ASSIGNED"
	ActionRoleRevoked        = "ROLE_REVOKED"
	ActionAccountDeleted     = "ACCOUNT_DELETED"
	ActionAccountArchived    = "ACCOUNT_ARCHIVED"
)

// AuditEvent adalah satu baris log aktivitas yang ditulis di transaksi yang
// sama dengan operasi CUD. Snapshot wajib tidak memuat password_hash,
// google_id, token, atau OTP.
type AuditEvent struct {
	ActorID  int64
	TargetID int64
	Action   string
	Snapshot []byte
}

// AuditLogger menulis event audit. Implementasi menerima transaksi supaya
// operasi CUD dan log-nya commit/rollback bersama.
type AuditLogger interface {
	LogTx(ctx context.Context, tx *gorm.DB, event AuditEvent) error
}

type gormAuditLogger struct{}

// NewAuditLogger membuat logger activity_logs berbasis GORM.
func NewAuditLogger() AuditLogger {
	return gormAuditLogger{}
}

// LogTx menulis event ke activity_logs memakai transaksi yang diberikan.
func (gormAuditLogger) LogTx(ctx context.Context, tx *gorm.DB, event AuditEvent) error {
	if tx == nil {
		return errors.New("audit logger: transaksi nil")
	}

	var snapshot interface{}
	if len(event.Snapshot) > 0 {
		snapshot = string(event.Snapshot)
	}

	return tx.WithContext(ctx).Exec(`
		INSERT INTO activity_logs (actor_user_id, target_user_id, action, snapshot)
		VALUES (?, ?, ?, ?::jsonb)
	`, event.ActorID, event.TargetID, event.Action, snapshot).Error
}

type noopAuditLogger struct{}

// NewNoopAuditLogger membuat logger yang tidak menulis apa pun. Dipakai untuk
// unit test atau service yang tidak membutuhkan audit.
func NewNoopAuditLogger() AuditLogger {
	return noopAuditLogger{}
}

func (noopAuditLogger) LogTx(context.Context, *gorm.DB, AuditEvent) error {
	return nil
}

// auditUserSnapshotData membentuk data akun + role yang aman untuk audit.
// Field sensitif (password_hash, google_id, token, OTP) sengaja tidak pernah
// dimasukkan.
func auditUserSnapshotData(user *models.User, roles []models.Role) map[string]interface{} {
	return map[string]interface{}{
		"id":              user.ID,
		"name":            user.Name,
		"email":           user.Email,
		"nisn":            user.NISN,
		"is_active":       user.IsActive,
		"inactive_reason": user.InactiveReason,
		"deactivated_at":  user.DeactivatedAt,
		"archived_at":     user.ArchivedAt,
		"roles":           roleNameList(roles),
	}
}

// auditUserSnapshot membentuk snapshot aman akun + role. Field sensitif
// (password_hash, google_id) sengaja tidak pernah dimasukkan.
func auditUserSnapshot(user *models.User, roles []models.Role) ([]byte, error) {
	return json.Marshal(auditUserSnapshotData(user, roles))
}

// auditUserUpdateSnapshot menyimpan nilai lama dan baru saat akun diperbarui.
func auditUserUpdateSnapshot(before, after map[string]interface{}) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"before": before,
		"after":  after,
	})
}

// auditRolesSnapshot menyimpan role sebelum/sesudah penggantian role.
func auditRolesSnapshot(user *models.User, before, after []models.Role) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":            user.ID,
		"name":          user.Name,
		"email":         user.Email,
		"roles_before":  roleNameList(before),
		"roles_after":   roleNameList(after),
		"roles_added":   diffAddedRoleNames(before, after),
		"roles_removed": diffRemovedRoleNames(before, after),
	})
}

// diffAddedRoleNames mengembalikan role yang ditambahkan.
func diffAddedRoleNames(before, after []models.Role) []string {
	added, _ := diffRoleNames(before, after)
	return added
}

// diffRemovedRoleNames mengembalikan role yang dicabut.
func diffRemovedRoleNames(before, after []models.Role) []string {
	_, removed := diffRoleNames(before, after)
	return removed
}
