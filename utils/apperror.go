package utils

import "net/http"

// FieldError menjelaskan satu kesalahan pada field request. Frontend memakai
// Field untuk menempelkan pesan ke input yang tepat dan Code untuk branching.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AppError adalah error domain yang membawa status HTTP dan kode response.
// Service mengembalikan *AppError; middleware ErrorHandler menerjemahkannya
// menjadi envelope standar.
type AppError struct {
	HTTPStatus int
	Code       string
	Message    string
	Errors     []FieldError
	Data       interface{}
}

// Error mengimplementasikan interface error.
func (e *AppError) Error() string {
	return e.Message
}

// NewAppError membuat AppError baru.
func NewAppError(status int, code, message string) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: message}
}

// WithMessage mengembalikan salinan AppError dengan pesan berbeda, sehingga
// error sentinel di bawah tetap aman dipakai berulang (tidak termutasi).
func (e *AppError) WithMessage(message string) *AppError {
	clone := *e
	clone.Message = message
	return &clone
}

// WithField mengembalikan salinan AppError dengan satu detail field tambahan.
func (e *AppError) WithField(field, code, message string) *AppError {
	return e.WithFields(FieldError{Field: field, Code: code, Message: message})
}

// WithFields mengembalikan salinan AppError dengan detail field tambahan.
func (e *AppError) WithFields(fields ...FieldError) *AppError {
	clone := *e
	clone.Errors = append(append([]FieldError(nil), e.Errors...), fields...)
	return &clone
}

// WithData mengembalikan salinan AppError dengan payload data tambahan.
func (e *AppError) WithData(data interface{}) *AppError {
	clone := *e
	clone.Data = data
	return &clone
}

// NewValidationError membuat AppError validasi (422) beserta detail field.
func NewValidationError(errs []FieldError) *AppError {
	return &AppError{
		HTTPStatus: http.StatusUnprocessableEntity,
		Code:       CodeValidation,
		Message:    "Validasi data gagal",
		Errors:     errs,
	}
}

// Error sentinel domain (dipakai berulang via WithMessage/WithField/WithData).
var (
	ErrInvalidCredentials = NewAppError(http.StatusUnauthorized, CodeInvalidCredentials, "Email/username atau password salah")
	ErrAccountInactive    = NewAppError(http.StatusForbidden, CodeAccountInactive, "Akun tidak aktif, hubungi admin")
	ErrAccountLocked      = NewAppError(http.StatusTooManyRequests, CodeAccountLocked, "Akun terkunci sementara karena percobaan login berlebih")
	ErrUnauthorized       = NewAppError(http.StatusUnauthorized, CodeUnauthorized, "Autentikasi diperlukan atau token tidak valid")
	ErrTokenExpired       = NewAppError(http.StatusUnauthorized, CodeTokenExpired, "Token telah kedaluwarsa")
	ErrSessionRevoked     = NewAppError(http.StatusUnauthorized, CodeSessionRevoked, "Sesi tidak valid atau telah berakhir")
	ErrForbidden          = NewAppError(http.StatusForbidden, CodeForbidden, "Anda tidak memiliki hak akses")
	ErrNotFound           = NewAppError(http.StatusNotFound, CodeNotFound, "Data tidak ditemukan")
	ErrConflict           = NewAppError(http.StatusConflict, CodeConflict, "Data sudah terdaftar")
	ErrInternal           = NewAppError(http.StatusInternalServerError, CodeInternal, "Terjadi kesalahan pada server")

	ErrUserNotFound           = NewAppError(http.StatusNotFound, CodeAccountNotFound, "Akun tidak ditemukan")
	ErrStaleData              = NewAppError(http.StatusConflict, CodeStaleData, "Data telah berubah, muat ulang lalu coba lagi")
	ErrLastAdmin              = NewAppError(http.StatusConflict, CodeLastAdminProtected, "Operasi ditolak: minimal satu Admin harus tetap ada")
	ErrSelfAction             = NewAppError(http.StatusForbidden, CodeSelfActionForbidden, "Anda tidak dapat melakukan operasi ini pada akun sendiri")
	ErrAccountHasHistory      = NewAppError(http.StatusConflict, CodeAccountHasHistory, "Akun tidak dapat dihapus karena masih memiliki riwayat data")
	ErrEmailAlreadyExists     = NewAppError(http.StatusConflict, CodeEmailAlreadyExists, "Email sudah terdaftar")
	ErrAccountAlreadyInactive = NewAppError(http.StatusConflict, CodeAccountAlreadyInactive, "Akun sudah nonaktif")
	ErrAccountAlreadyActive   = NewAppError(http.StatusConflict, CodeAccountAlreadyActive, "Akun sudah aktif")
	ErrRoleNotAssignable      = NewAppError(http.StatusUnprocessableEntity, CodeRoleNotAssignable, "Role tidak dapat diberikan lewat endpoint ini")
	ErrInvalidRoleCombination = NewAppError(http.StatusUnprocessableEntity, CodeInvalidRoleCombination, "Kombinasi role tidak valid")
	ErrAccountTypeNotEditable = NewAppError(http.StatusUnprocessableEntity, CodeAccountTypeNotEditable, "Data akun siswa dikelola di Data Master Siswa")

	// Epic 3 - Master Data Management
	ErrAcademicYearNotFound      = NewAppError(http.StatusNotFound, CodeAcademicYearNotFound, "Tahun ajaran tidak ditemukan")
	ErrAcademicYearInUse         = NewAppError(http.StatusConflict, CodeAcademicYearInUse, "Tahun ajaran tidak dapat dihapus karena sudah memiliki data kelas terkait")
	ErrAcademicYearAlreadyExists = NewAppError(http.StatusConflict, CodeAcademicYearAlreadyExists, "Nama tahun ajaran sudah terdaftar")
	ErrSubjectNotFound           = NewAppError(http.StatusNotFound, CodeSubjectNotFound, "Mata pelajaran tidak ditemukan")
	ErrSubjectInUse              = NewAppError(http.StatusConflict, CodeSubjectInUse, "Mata pelajaran tidak dapat dihapus karena sedang digunakan. Silakan nonaktifkan (is_active = false) mata pelajaran ini")
	ErrSubjectAlreadyExists      = NewAppError(http.StatusConflict, CodeSubjectAlreadyExists, "Kode mata pelajaran sudah terdaftar")
	ErrClassNotFound             = NewAppError(http.StatusNotFound, CodeClassNotFound, "Kelas tidak ditemukan")
	ErrClassInUse                = NewAppError(http.StatusConflict, CodeClassInUse, "Kelas tidak dapat dihapus karena masih memiliki siswa atau jadwal terkait. Silakan nonaktifkan (is_active = false) kelas ini")
	ErrClassAlreadyExists        = NewAppError(http.StatusConflict, CodeClassAlreadyExists, "Nama kelas sudah terdaftar pada tahun ajaran ini")
	ErrTeacherNotFound           = NewAppError(http.StatusNotFound, CodeTeacherNotFound, "Data guru tidak ditemukan")
	ErrTeacherAlreadyExists      = NewAppError(http.StatusConflict, CodeTeacherAlreadyExists, "Akun ini sudah memiliki profil guru")
	ErrAccountNotTeacherRole     = NewAppError(http.StatusUnprocessableEntity, CodeAccountNotTeacherRole, "Akun yang dipilih harus memiliki role guru_mapel")
	ErrStudentNotFound           = NewAppError(http.StatusNotFound, CodeStudentNotFound, "Data siswa tidak ditemukan")

	// Epic 4 - Schedule Management
	ErrImportFileInvalid       = NewAppError(http.StatusUnprocessableEntity, CodeImportFileInvalid, "File Excel tidak valid atau format kolom tidak sesuai template")
	ErrImportNotValidated      = NewAppError(http.StatusUnprocessableEntity, CodeImportNotValidated, "Token import tidak valid atau telah kedaluwarsa, silakan ulangi preview")
	ErrScheduleNotFound        = NewAppError(http.StatusNotFound, CodeScheduleNotFound, "Jadwal tidak ditemukan")
	ErrScheduleSlotConflict    = NewAppError(http.StatusConflict, CodeScheduleSlotConflict, "Jadwal bentrok pada kelas atau guru di tanggal dan jam tersebut")
	ErrScheduleAlreadyActivity = NewAppError(http.StatusConflict, CodeScheduleAlreadyActivity, "Jadwal ini sudah berstatus kegiatan")
	ErrScheduleAlreadyLesson   = NewAppError(http.StatusConflict, CodeScheduleAlreadyLesson, "Jadwal ini sudah berstatus pelajaran")
	ErrScheduleInUse           = NewAppError(http.StatusConflict, CodeScheduleInUse, "Jadwal tidak dapat diubah atau dihapus karena sudah memiliki data absensi atau jurnal terkait")

	// Epic 5 - Student Attendance
	ErrAttendanceFormNotAvailable  = NewAppError(http.StatusForbidden, CodeAttendanceFormNotAvailable, "Form absensi belum terbuka untuk jadwal ini")
	ErrAttendanceAlreadyRecorded   = NewAppError(http.StatusConflict, CodeAttendanceAlreadyRecorded, "Absensi pada jadwal ini sudah pernah dicatat")
	ErrAttendanceNotFound          = NewAppError(http.StatusNotFound, CodeAttendanceNotFound, "Data absensi siswa tidak ditemukan")
	ErrLeaveRequestDeadlinePassed  = NewAppError(http.StatusUnprocessableEntity, CodeLeaveRequestDeadlinePassed, "Batas waktu pengajuan izin/sakit hari ini telah lewat (maksimal pukul 09:00 WIB)")
	ErrLeaveRequestAlreadyActive   = NewAppError(http.StatusConflict, CodeLeaveRequestAlreadyActive, "Siswa sudah memiliki pengajuan izin/sakit untuk tanggal ini")
	ErrLeaveRequestAlreadyReviewed = NewAppError(http.StatusConflict, CodeLeaveRequestAlreadyReviewed, "Pengajuan izin/sakit ini sudah diproses dan bersifat final")
	ErrLeaveRequestNotFound        = NewAppError(http.StatusNotFound, CodeLeaveRequestNotFound, "Pengajuan izin/sakit tidak ditemukan")
	ErrEvidenceFileRequired        = NewAppError(http.StatusUnprocessableEntity, CodeEvidenceFileRequired, "Bukti foto atau dokumen PDF wajib dilampirkan")
	ErrEvidenceFileInvalid         = NewAppError(http.StatusUnprocessableEntity, CodeEvidenceFileInvalid, "Format atau ukuran file bukti tidak valid (hanya JPG, PNG, WEBP, atau PDF maksimal 5 MB)")
	ErrStudentOutOfScope           = NewAppError(http.StatusForbidden, CodeStudentOutOfScope, "Siswa berada di luar cakupan kelas Anda")
	ErrPermissionLetterNotFound    = NewAppError(http.StatusNotFound, CodePermissionLetterNotFound, "Surat izin tidak ditemukan")

	// Epic 6 - Teacher Attendance & QR
	ErrQRCodeExpired                    = NewAppError(http.StatusBadRequest, CodeQRCodeExpired, "QR Code atau kode cadangan sudah kedaluwarsa, silakan generate ulang")
	ErrQRCodeInvalid                    = NewAppError(http.StatusBadRequest, CodeQRCodeInvalid, "QR Code atau kode cadangan tidak valid")
	ErrTeacherAttendanceAlreadyRecorded = NewAppError(http.StatusConflict, CodeTeacherAttendanceAlreadyRecorded, "Kehadiran guru untuk jadwal ini sudah tercatat")
	ErrTeacherAttendanceNotFound        = NewAppError(http.StatusNotFound, CodeTeacherAttendanceNotFound, "Data kehadiran guru tidak ditemukan")
	ErrScheduleTypeNotSupported         = NewAppError(http.StatusUnprocessableEntity, CodeScheduleTypeNotSupported, "Operasi ini hanya berlaku untuk jadwal bertipe pelajaran")
	ErrScheduleNotActive                = NewAppError(http.StatusUnprocessableEntity, CodeScheduleNotActive, "Tidak ada jadwal pelajaran yang sedang berlangsung saat ini")
	ErrReasonRequired                   = NewAppError(http.StatusUnprocessableEntity, CodeReasonRequired, "Alasan koreksi wajib diisi")

	ErrOTPInvalid       = NewAppError(http.StatusBadRequest, CodeOTPInvalid, "Kode OTP salah")
	ErrOTPExpired       = NewAppError(http.StatusBadRequest, CodeOTPExpired, "Kode OTP sudah kedaluwarsa, silakan minta kode baru")
	ErrOTPTooMany       = NewAppError(http.StatusTooManyRequests, CodeOTPTooMany, "Terlalu banyak percobaan OTP, silakan minta kode baru")
	ErrOTPResendTooSoon = NewAppError(http.StatusTooManyRequests, CodeOTPResendTooSoon, "Mohon tunggu sebelum meminta kode baru")
	ErrOTPResendLimit   = NewAppError(http.StatusTooManyRequests, CodeOTPResendLimit, "Batas permintaan OTP per jam tercapai")

	ErrPasswordSame      = NewAppError(http.StatusBadRequest, CodePasswordSame, "Kata sandi baru tidak boleh sama dengan kata sandi sebelumnya")
	ErrResetTokenInvalid = NewAppError(http.StatusUnauthorized, CodeResetTokenInvalid, "Token verifikasi tidak valid atau sudah digunakan")
	ErrResetTokenExpired = NewAppError(http.StatusUnauthorized, CodeResetTokenExpired, "Token verifikasi sudah kedaluwarsa, silakan minta kode OTP baru")

	ErrRateLimited           = NewAppError(http.StatusTooManyRequests, CodeRateLimited, "Terlalu banyak permintaan, silakan coba lagi nanti")
	ErrGoogleDisabled        = NewAppError(http.StatusServiceUnavailable, CodeGoogleDisabled, "Login Google belum dikonfigurasi")
	ErrGoogleTokenInvalid    = NewAppError(http.StatusUnauthorized, CodeGoogleTokenInvalid, "Token Google tidak valid")
	ErrGoogleEmailUnverified = NewAppError(http.StatusUnauthorized, CodeGoogleEmailUnverified, "Email Google belum terverifikasi")
	ErrAccountNotFound       = NewAppError(http.StatusNotFound, CodeAuthAccountNotFound, "Akun belum terdaftar, silakan registrasi terlebih dahulu")
)
