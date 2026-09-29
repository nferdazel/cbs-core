package domain

// ojk_reference_list.go menyajikan tabel referensi sandi OJK sebagai daftar baca-saja
// untuk pemilih UI, melengkapi pemeriksaan penulisan di ojk_reference_codes.go.
//
// Tabel referensi (migrasi 000102) sebelumnya hanya dipakai memverifikasi sandi saat
// pengisian; belum ada endpoint yang menyajikannya, sehingga UI mengisi sandi sebagai
// teks bebas (temuan docs/CELAH-FORM-OJK.md §3 poin 9). Daftar di sini hanya MEMBACA
// salinan sandi resmi Lampiran 02/03 SEOJK No. 16/SEOJK.03/2024; tidak ada sandi yang
// dikarang dan tidak ada penulisan.

// OJKReferenceOption adalah satu baris daftar sandi referensi OJK. Code adalah sandi
// resmi (mis. "001"), Name adalah label yang tersimpan di tabel referensi. Province
// hanya berarti untuk Lampiran 03 (kabupaten/kota) dan kosong untuk Lampiran 02.
type OJKReferenceOption struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Province string `json:"province,omitempty"`
}
