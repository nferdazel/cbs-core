package domain

import "strings"

// InstitutionBookScope adalah cakupan buku COA tingkat INSTALASI. CBS Core dijual
// per bank (satu instalasi per badan hukum), jadi badan hukum menentukan lini usaha
// yang dilayani:
//
//   - KONVENSIONAL: bank konvensional (BPR). Sisi syariah tidak diakses sama sekali.
//   - SYARIAH:      bank syariah (BPRS). Sisi konvensional tidak diakses sama sekali.
//   - DUAL:         bank umum dengan UUS. Kedua lini aktif (perilaku yang berlaku
//     sebelum setelan ini ada).
//
// Setelan ini adalah lapisan pertama; penugasan buku per pengguna
// (staff_users.book lewat Actor.Book/CanAccessBook/bookReadClause) tetap menjadi
// lapisan kedua khusus untuk instalasi DUAL dan TIDAK dihapus.
type InstitutionBookScope string

const (
	ScopeConventional InstitutionBookScope = "KONVENSIONAL"
	ScopeSyariah      InstitutionBookScope = "SYARIAH"
	ScopeDual         InstitutionBookScope = "DUAL"
)

// ConfigKeyInstitutionBookScope adalah kunci system_config cakupan buku instalasi.
// Nilai awalnya DUAL (migrasi 000074) agar perilaku produksi tidak berubah.
const ConfigKeyInstitutionBookScope = "institution.book_scope"

// ParseInstitutionBookScope menafsirkan nilai konfigurasi. Nilai kosong, tidak
// dikenal, atau rusak diperlakukan sebagai DUAL supaya instalasi yang belum
// di-provision tetap berjalan persis seperti sebelumnya.
func ParseInstitutionBookScope(raw string) InstitutionBookScope {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(ScopeSyariah):
		return ScopeSyariah
	case string(ScopeConventional), "CONVENTIONAL":
		return ScopeConventional
	default:
		return ScopeDual
	}
}

// IsDual melaporkan apakah kedua buku aktif. Scope zero-value ("") juga DUAL agar
// Actor yang dibangun tanpa scope (uji lama, pemanggil non-HTTP) tidak berubah.
func (s InstitutionBookScope) IsDual() bool {
	return s == "" || s == ScopeDual
}

// SingleBook mengembalikan satu-satunya buku aktif, atau "" bila cakupannya DUAL.
func (s InstitutionBookScope) SingleBook() COABook {
	switch s {
	case ScopeConventional:
		return BookConventional
	case ScopeSyariah:
		return BookSyariah
	default:
		return ""
	}
}

// ActiveBooks mengembalikan daftar buku yang aktif di instalasi ini. Inilah satu
// sumber kebenaran "buku apa saja yang aktif", dipakai penjaga API, batch/EOD, dan
// endpoint identitas (/auth/me) untuk menyaring menu web.
func (s InstitutionBookScope) ActiveBooks() []COABook {
	if b := s.SingleBook(); b != "" {
		return []COABook{b}
	}
	return []COABook{BookConventional, BookSyariah}
}

// AllowsBook melaporkan apakah buku book boleh diakses pada cakupan instalasi ini.
// Buku kosong (data pra-buku) selalu diizinkan, mengikuti semantik Actor.CanAccessBook,
// agar data lama tidak mendadak terblokir.
func (s InstitutionBookScope) AllowsBook(book COABook) bool {
	if s.IsDual() {
		return true
	}
	if book == "" {
		return true
	}
	return book == s.SingleBook()
}
