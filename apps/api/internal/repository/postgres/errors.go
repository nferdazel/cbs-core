package postgres

import "strings"

// isUniqueViolation mendeteksi pelanggaran unique constraint (SQLSTATE 23505).
// Pengecekan lewat pesan dipakai agar tidak bergantung pada tipe error driver.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}
