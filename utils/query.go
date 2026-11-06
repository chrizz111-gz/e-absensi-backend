package utils

import "strings"

// EscapeLike meng-escape karakter wildcard LIKE/ILIKE agar input pencarian
// pengguna diperlakukan sebagai teks literal. Query pemakai wajib menambahkan
// klausa ESCAPE '\' agar hasil escape ini berlaku.
func EscapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
