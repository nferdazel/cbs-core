package domain

import (
	"errors"
	"fmt"
)

// LocalizedError adalah galat domain yang pesannya ditampilkan ke pengguna dan
// memiliki padanan kode katalog i18n. Pesan default tetap berbahasa Indonesia
// (makna lama tidak berubah); lapisan HTTP menerjemahkannya memakai kode ini lewat
// LocalizedMessage. Galat baru cukup dibuat dengan NewLocalizedError di dekat
// definisinya, sehingga tidak ada daftar pemetaan kedua di handler dan galat yang
// dibungkus %w pun tetap dikenali (errors.As pada LocalizedMessage).
type LocalizedError struct {
	code string
	msg  string
	// args adalah nilai untuk placeholder pada pesan katalog. Kosong berarti pesan
	// katalog tidak memuat placeholder dan diterjemahkan apa adanya.
	args []any
}

// NewLocalizedError membuat galat domain berkode katalog. code harus sama dengan
// konstanta i18n.Code yang didaftarkan di katalog terjemahan; uji keselarasan di
// paket ini menolak kode yang tidak punya terjemahan.
func NewLocalizedError(code, message string) *LocalizedError {
	return &LocalizedError{code: code, msg: message}
}

// NewLocalizedErrorf sama dengan NewLocalizedError tetapi pesan dasarnya memuat
// placeholder %s yang diisi DAMPAK PEMBUATAN GALAT. Dipakai bila data dinamis berada
// di TENGAH pesan (mis. "akun COA 10999 tidak ditemukan"): pesan dasar harus sudah
// lengkap supaya ID tetap sama persis, sementara katalog EN memakai susunan kata
// sendiri dengan placeholder yang sama. Pesan dasar yang memuat %s wajib punya
// argumen; pemanggil tidak boleh menyerahkan %s tanpa nilai.
func NewLocalizedErrorf(code, format string, args ...any) *LocalizedError {
	return &LocalizedError{code: code, msg: fmt.Sprintf(format, args...), args: args}
}

// MessageArgs mengembalikan nilai placeholder pesan katalog, bila ada. Lapisan HTTP
// memakainya untuk menerjemahkan pesan berplaceholder lewat Textf; galat tanpa
// placeholder mengembalikan nil sehingga terjemahan biasa tetap dipakai.
func (e *LocalizedError) MessageArgs() []any { return e.args }

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

// LocalizedMessageArgs mengembalikan kode katalog, pesan dasar, dan nilai placeholder
// galat domain (bila ada). Dipakai lapisan HTTP supaya pesan katalog berplaceholder
// (data di TENGAH pesan) dapat diterjemahkan tanpa mengubah bunyi pesan Indonesia.
func LocalizedMessageArgs(err error) (code, base string, args []any, ok bool) {
	var le *LocalizedError
	if errors.As(err, &le) {
		return le.code, le.msg, le.args, true
	}
	return "", "", nil, false
}
