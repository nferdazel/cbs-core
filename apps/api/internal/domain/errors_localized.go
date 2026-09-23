package domain

import "errors"

// LocalizedError adalah galat domain yang pesannya ditampilkan ke pengguna dan
// memiliki padanan kode katalog i18n. Pesan default tetap berbahasa Indonesia
// (makna lama tidak berubah); lapisan HTTP menerjemahkannya memakai kode ini lewat
// LocalizedMessage. Galat baru cukup dibuat dengan NewLocalizedError di dekat
// definisinya, sehingga tidak ada daftar pemetaan kedua di handler dan galat yang
// dibungkus %w pun tetap dikenali (errors.As pada LocalizedMessage).
type LocalizedError struct {
	code string
	msg  string
}

// NewLocalizedError membuat galat domain berkode katalog. code harus sama dengan
// konstanta i18n.Code yang didaftarkan di katalog terjemahan; uji keselarasan di
// paket ini menolak kode yang tidak punya terjemahan.
func NewLocalizedError(code, message string) *LocalizedError {
	return &LocalizedError{code: code, msg: message}
}

func (e *LocalizedError) Error() string { return e.msg }

// MessageCode mengembalikan kode katalog i18n galat ini.
func (e *LocalizedError) MessageCode() string { return e.code }

// LocalizedMessage mengembalikan kode katalog dan pesan dasar bila galat (atau galat
// yang dibungkusnya) adalah LocalizedError. errors.As dipakai agar galat yang
// dibungkus fmt.Errorf("%w: ...") ikut dikenali, bukan hanya kecocokan ==.
func LocalizedMessage(err error) (code, base string, ok bool) {
	var le *LocalizedError
	if errors.As(err, &le) {
		return le.code, le.msg, true
	}
	return "", "", false
}
