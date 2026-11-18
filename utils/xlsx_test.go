package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAndReadXLSXRoundTrip(t *testing.T) {
	input := [][]string{
		{"Tanggal (YYYY-MM-DD)", "Jam Mulai (HH:MM)", "Jam Selesai (HH:MM)", "Nama Kelas", "Kode Mata Pelajaran", "Email Guru"},
		{"2026-11-18", "07:00", "08:30", "X RPL 1", "MTK-10", "guru@sekolah.sch.id"},
	}

	raw, err := BuildXLSX("Template Jadwal", input)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	rows, err := ReadXLSX(raw)
	require.NoError(t, err)
	assert.Equal(t, input, rows)
}

func TestReadXLSXRejectsInvalidBytes(t *testing.T) {
	_, err := ReadXLSX([]byte("bukan-file-zip"))
	assert.ErrorIs(t, err, ErrInvalidXLSX)
}
