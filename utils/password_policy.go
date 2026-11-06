package utils

import "unicode"

// Password policy codes returned by PasswordPolicyViolation. They are used as
// the "code" value on field errors so the frontend can branch on them.
const (
	PasswordPolicyOK           = ""
	PasswordTooShort           = "PASSWORD_TOO_SHORT"
	PasswordTooLong            = "PASSWORD_TOO_LONG"
	PasswordNeedsLower         = "PASSWORD_NEEDS_LOWER"
	PasswordNeedsUpper         = "PASSWORD_NEEDS_UPPER"
	PasswordNeedsDigit         = "PASSWORD_NEEDS_DIGIT"
	PasswordNeedsSymbol        = "PASSWORD_NEEDS_SYMBOL"
	PasswordContainsWhitespace = "PASSWORD_CONTAINS_WHITESPACE"
)

// PasswordPolicyViolation checks a password against the project policy:
//   - 8-72 bytes (bcrypt only uses the first 72 bytes),
//   - at least one lowercase letter,
//   - at least one uppercase letter,
//   - at least one digit,
//   - at least one symbol (anything that is not a letter, digit, or whitespace),
//   - no whitespace.
//
// It returns an empty string when the password is valid, or one of the
// Password* codes above describing the first violation found.
func PasswordPolicyViolation(password string) string {
	if len(password) < 8 {
		return PasswordTooShort
	}
	if len(password) > 72 {
		return PasswordTooLong
	}

	var lower, upper, digit, symbol bool
	for _, r := range password {
		switch {
		case unicode.IsSpace(r):
			return PasswordContainsWhitespace
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsLetter(r):
			// Letter without case (for example scripts other than Latin) can
			// never satisfy the uppercase/lowercase requirements.
		default:
			symbol = true
		}
	}

	switch {
	case !lower:
		return PasswordNeedsLower
	case !upper:
		return PasswordNeedsUpper
	case !digit:
		return PasswordNeedsDigit
	case !symbol:
		return PasswordNeedsSymbol
	default:
		return PasswordPolicyOK
	}
}

// IsStrongPassword reports whether the password satisfies the policy.
func IsStrongPassword(password string) bool {
	return PasswordPolicyViolation(password) == PasswordPolicyOK
}

// PasswordPolicyMessage returns the human-readable message for a policy code.
func PasswordPolicyMessage(code string) string {
	switch code {
	case PasswordTooShort:
		return "Password minimal 8 karakter"
	case PasswordTooLong:
		return "Password maksimal 72 karakter"
	case PasswordNeedsLower:
		return "Password harus memuat huruf kecil"
	case PasswordNeedsUpper:
		return "Password harus memuat huruf besar"
	case PasswordNeedsDigit:
		return "Password harus memuat angka"
	case PasswordNeedsSymbol:
		return "Password harus memuat simbol"
	case PasswordContainsWhitespace:
		return "Password tidak boleh mengandung spasi"
	default:
		return "Password belum memenuhi kriteria keamanan"
	}
}
