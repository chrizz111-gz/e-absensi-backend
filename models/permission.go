package models

// Permission adalah satu izin sistem pada tabel permissions. Kode berformat
// module.action dan dipakai middleware RequirePermission untuk otorisasi.
type Permission struct {
	ID          int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Code        string  `gorm:"column:code;size:100;not null" json:"code"`
	Module      string  `gorm:"column:module;size:50;not null" json:"module"`
	Description *string `gorm:"column:description;size:255" json:"description,omitempty"`
}

// TableName memetakan Permission ke tabel "permissions".
func (Permission) TableName() string {
	return "permissions"
}

// RolePermission adalah pivot pemberian permission ke role. Matriks ini
// bersifat tetap dan diubah lewat migrasi/seeder, bukan endpoint CRUD.
type RolePermission struct {
	RoleID       int16 `gorm:"column:role_id;primaryKey" json:"role_id"`
	PermissionID int64 `gorm:"column:permission_id;primaryKey" json:"permission_id"`
}

// TableName memetakan RolePermission ke tabel "role_permissions".
func (RolePermission) TableName() string {
	return "role_permissions"
}

// Kode permission Epic 2, Epic 3, Epic 4, Epic 5, dan Epic 6.
// Nilai ini harus sama dengan seed pada migrasi 004, 017, dan 024.
const (
	// Epic 2 - User & Role Management
	PermissionAccountView      = "account.view"
	PermissionAccountCreate    = "account.create"
	PermissionAccountUpdate    = "account.update"
	PermissionAccountSetStatus = "account.set_status"
	PermissionAccountDelete    = "account.delete"
	PermissionRoleAssign       = "role.assign"
	PermissionPermissionView   = "permission.view"

	// Epic 3 - Master Data Management
	PermissionStudentView        = "student.view"
	PermissionStudentUpdate      = "student.update"
	PermissionTeacherView        = "teacher.view"
	PermissionTeacherCreate      = "teacher.create"
	PermissionTeacherUpdate      = "teacher.update"
	PermissionClassView          = "class.view"
	PermissionClassCreate        = "class.create"
	PermissionClassUpdate        = "class.update"
	PermissionClassDelete        = "class.delete"
	PermissionSubjectView        = "subject.view"
	PermissionSubjectCreate      = "subject.create"
	PermissionSubjectUpdate      = "subject.update"
	PermissionSubjectDelete      = "subject.delete"
	PermissionAcademicYearView   = "academic_year.view"
	PermissionAcademicYearCreate = "academic_year.create"
	PermissionAcademicYearUpdate = "academic_year.update"
	PermissionAcademicYearDelete = "academic_year.delete"

	// Epic 4 - Schedule Management
	PermissionScheduleDownloadTemplate = "schedule.download_template"
	PermissionScheduleImport           = "schedule.import"
	PermissionScheduleView             = "schedule.view"
	PermissionScheduleUpdate           = "schedule.update"
	PermissionScheduleDelete           = "schedule.delete"
	PermissionScheduleConvertActivity  = "schedule.convert_activity"

	// Epic 5 - Student Attendance
	PermissionAttendanceInput        = "attendance.input"
	PermissionAttendanceInputFallback = "attendance.input_fallback"
	PermissionAttendanceUpdate       = "attendance.update"
	PermissionAttendanceView         = "attendance.view"
	PermissionAttendanceExport       = "attendance.export"
	PermissionLeaveRequestSubmit     = "leave_request.submit"
	PermissionLeaveRequestReview     = "leave_request.review"
	PermissionPermissionLetterUpload = "permission_letter.upload"

	// Epic 6 - Teacher Attendance & QR
	PermissionTeacherAttendanceGenerateQR = "teacher_attendance.generate_qr"
	PermissionTeacherAttendanceConfirm    = "teacher_attendance.confirm"
	PermissionTeacherAttendanceView       = "teacher_attendance.view"
	PermissionTeacherAttendanceAnnotate   = "teacher_attendance.annotate"
	PermissionTeacherAttendanceExport     = "teacher_attendance.export"
)
