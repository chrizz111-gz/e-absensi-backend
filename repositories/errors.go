package repositories

import (
	"errors"

	"gorm.io/gorm"
)

// ErrNotFound dikembalikan repository saat baris tidak ditemukan, sebagai
// abstraksi dari gorm.ErrRecordNotFound.
var ErrNotFound = errors.New("data tidak ditemukan")

// translate memetakan error GORM ke error domain repository.
func translate(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
