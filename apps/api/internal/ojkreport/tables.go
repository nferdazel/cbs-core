package ojkreport

// tables.go memuat representasi form daftar/rincian (Form 00.00, 05.00, 06.00).
//
// Berbeda dari Form 01.00/02.00 yang berupa pos-pos neraca/laba rugi, form daftar
// berisi baris per pihak lawan (debitur/bank). Susunan kolom mengikuti Lampiran II
// SEOJK No. 16/SEOJK.03/2024; kolom yang datanya tidak ada di sistem didaftarkan
// pada Unavailable beserta alasannya, sehingga berkas tidak pernah menampilkan nol
// sebagai pengganti "tidak tersedia".

// TableColumn adalah satu kolom form daftar yang datanya tersedia.
type TableColumn struct {
	Sandi string
	Nama  string
}

// ColumnUnavailable adalah kolom form daftar yang datanya tidak ada, beserta alasan
// spesifik dan (bila ada) kunci konfigurasi yang harus diisi bank.
type ColumnUnavailable struct {
	Sandi  string
	Nama   string
	Reason string
}

// TableCell adalah satu nilai kolom pada satu baris form daftar. Value sudah
// diformat sebagai teks (rupiah penuh, persen, tanggal, atau sandi).
type TableCell struct {
	Sandi string
	Nama  string
	Value string
}

// TableRow adalah satu baris form daftar. Key adalah identitas baris (mis. nomor
// rekening kredit atau nama bank lawan). Reason != "" berarti baris belum tersedia
// (dipakai Form 00.00 yang field-nya berbentuk baris).
type TableRow struct {
	Key    string
	Cells  []TableCell
	Reason string
}

// TableSection adalah satu form daftar lengkap.
type TableSection struct {
	Form string
	Name string
	// KeyLabel menjelaskan isi Key (mis. "No. Rekening").
	KeyLabel    string
	Columns     []TableColumn
	Rows        []TableRow
	Unavailable []ColumnUnavailable
	// Notes mencatat batas sumber data yang perlu diketahui pembaca berkas.
	Notes []string
}
