package ojkreport

// form12.go membangun Form 12.00 "Daftar Deposito" dari baris per kontrak deposito.
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 12.00 - 1/-2 kolom dan sandi: PDF #page 200-203 (hlm. tercetak 148-151).
//     Kolom I Sandi Kantor, II ID Pihak Lawan, III No. Rekening, IV Hubungan dengan
//     Bank, V Golongan Nasabah, VI Lokasi Nasabah, VII Jangka Waktu (Tanggal Mulai/
//     Tanggal Jatuh Tempo), VIII Suku Bunga, IX Nominal, X Nominal yang
//     Diblokir/Dijaminkan, XI Alasan Diblokir, XII Biaya Transaksi Belum Diamortisasi,
//     XIII Jumlah, XIV Nomor Identitas, XV Jenis PEP, XVI Risiko Nasabah, XVII Status
//     Data. Berbeda dari Form 11.00, form ini TIDAK punya kolom "Jenis".
//   - Form 12.00 - 3 penjelasan: PDF #page 204-206 (hlm. 152-154). Kolom IX Nominal
//     adalah "nilai nominal deposito pada tanggal laporan"; kolom XIII Jumlah adalah
//     "nominal dikurangi dengan biaya transaksi belum diamortisasi".
//
// BATAS SUMBER - jangan diisi tebakan:
//   - Baris = satu rekening deposito unik (PDF #page 204). Posisi yang dibaca adalah
//     AKHIR PERIODE laporan: kontrak sudah ditempatkan dan belum ditutup saat itu.
//   - Kolom I Sandi Kantor memakai kantor pelapor tunggal (bank_offices) yang sama
//     dengan Form 09.00/01.01 lewat pasangKolomSandiKantor.
//   - Kolom XIV Nomor Identitas TIDAK dipaparkan: NIK/NPWP tersimpan terenkripsi dan
//     sengaja tidak dibuka sebagai keluaran laporan (keputusan privasi).
//   - Kolom X Nominal Diblokir/Dijaminkan, XI Alasan Diblokir, XII Biaya Transaksi
//     Belum Diamortisasi, XV Jenis PEP, XVI Risiko Nasabah, dan XVII Status Data belum
//     punya sumber tersimpan; ditulis "-" beserta alasan, bukan nol.

// form12Column adalah satu kolom Form 12.00; Reason != "" berarti belum tersedia.
type form12Column struct {
	Sandi  string
	Nama   string
	Reason string
	Value  func(TimeDepositRow) string
}

const (
	form12SandiKantor     = "I"
	form12SandiIDPihak    = "II"
	form12SandiNoRek      = "III"
	form12SandiHubungan   = "IV"
	form12SandiGolongan   = "V"
	form12SandiLokasi     = "VI"
	form12SandiJangka     = "VII"
	form12SandiSukuBunga  = "VIII"
	form12SandiNominal    = "IX"
	form12SandiDiblokir   = "X"
	form12SandiAlasan     = "XI"
	form12SandiBiaya      = "XII"
	form12SandiJumlah     = "XIII"
	form12SandiNomorID    = "XIV"
	form12SandiPEP        = "XV"
	form12SandiRisiko     = "XVI"
	form12SandiStatusData = "XVII"
)

// form12Columns adalah susunan kolom Form 12.00. Kolom I disisipkan
// pasangKolomSandiKantor; daftar ini mulai dari kolom II.
var form12Columns = []form12Column{
	{Sandi: form12SandiIDPihak, Nama: "ID Pihak Lawan", Value: func(r TimeDepositRow) string {
		// ID Pihak Lawan = nomor CIF internal nasabah (BAB II Lampiran II), bukan sandi OJK.
		return dashIfEmpty(r.CounterpartyCIF)
	}},
	{Sandi: form12SandiNoRek, Nama: "No. Rekening", Value: func(r TimeDepositRow) string {
		return dashIfEmpty(r.AccountNumber)
	}},
	{Sandi: form12SandiHubungan, Nama: "Hubungan dengan Bank", Value: func(r TimeDepositRow) string {
		// Sandi inline Lampiran II Form 12.00 (12 terkait, 20 tidak terkait) per nasabah.
		return dashIfEmpty(r.HubunganBankCode)
	}},
	{Sandi: form12SandiGolongan, Nama: "Golongan Nasabah", Value: func(r TimeDepositRow) string {
		// Sandi Lampiran 02 - Daftar Sandi Pihak Lawan (customers.ojk_pihak_lawan_code).
		return dashIfEmpty(r.CustomerTypeCode)
	}},
	{Sandi: form12SandiLokasi, Nama: "Lokasi Nasabah", Value: func(r TimeDepositRow) string {
		// Sandi Lampiran 03 - Daftar Sandi Kabupaten/Kota (customers.ojk_kabupaten_code).
		return dashIfEmpty(r.LocationCode)
	}},
	{Sandi: form12SandiJangka, Nama: "Jangka Waktu", Value: func(r TimeDepositRow) string {
		// Tanggal mulai dan jatuh tempo perjanjian terakhir (PDF #page 204; BAB II
		// butir K). Format tanggal TT-BB-TTTT mengikuti transkrip form.
		if r.StartDate.IsZero() {
			return "-"
		}
		jangka := r.StartDate.Format("02-01-2006")
		if !r.MaturityDate.IsZero() {
			jangka += " s.d. " + r.MaturityDate.Format("02-01-2006")
		}
		return jangka
	}},
	{Sandi: form12SandiSukuBunga, Nama: "Suku Bunga", Value: func(r TimeDepositRow) string {
		// Hanya deposito berbunga (profit_type INTEREST) yang dinyatakan sebagai
		// persen; margin/bagi hasil syariah bukan suku bunga sehingga ditulis "-".
		return sukuBungaPersen(r.ProfitType, r.ProfitRate)
	}},
	{Sandi: form12SandiNominal, Nama: "Nominal", Value: func(r TimeDepositRow) string {
		// Nilai nominal deposito pada tanggal laporan (PDF #page 204).
		return FormatRupiah(r.PlacementAmount)
	}},
	{Sandi: form12SandiDiblokir, Nama: "Nominal yang Diblokir/Dijaminkan", Reason: "kontrak deposito tidak menyimpan nominal yang diblokir/dijaminkan per akhir periode; tidak direkonstruksi (bukan ditulis nol)"},
	{Sandi: form12SandiAlasan, Nama: "Alasan Diblokir", Reason: "alasan pemblokiran dana tidak disimpan per kontrak deposito"},
	{Sandi: form12SandiBiaya, Nama: "Biaya Transaksi Belum Diamortisasi", Reason: "saldo biaya transaksi belum diamortisasi per kontrak deposito belum disimpan; jadwal amortisasinya belum dimodelkan"},
	{Sandi: form12SandiJumlah, Nama: "Jumlah", Value: func(r TimeDepositRow) string {
		// Form 12.00 - 3 (PDF #page 205): Jumlah = Nominal - Biaya Transaksi Belum
		// Diamortisasi. Karena biaya transaksi (kolom XII) belum tersimpan, nilai
		// pengurangnya nol; keterbatasan dicatat pada Notes.
		return FormatRupiah(r.PlacementAmount)
	}},
	{Sandi: form12SandiNomorID, Nama: "Nomor Identitas", Reason: "NIK/NPWP nasabah tersimpan terenkripsi untuk dokumen dan sengaja tidak dibuka sebagai keluaran laporan (keputusan privasi)"},
	{Sandi: form12SandiPEP, Nama: "Jenis PEP", Reason: "status PEP per nasabah belum disimpan"},
	{Sandi: form12SandiRisiko, Nama: "Risiko Nasabah", Reason: "profil risiko nasabah belum disimpan"},
	{Sandi: form12SandiStatusData, Nama: "Status Data", Reason: "penanda pengkinian data bulan berjalan belum disimpan"},
}

// buildForm12 menyusun Form 12.00 dari baris per kontrak deposito dan kantor pelapor.
// Fungsi ini murni sehingga dapat diuji tanpa basis data. Kantor pelapor dipasang
// sebagai kolom I; bila tidak dapat ditentukan, kolom I dinyatakan tidak tersedia.
func buildForm12(rows []TimeDepositRow, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "12.00",
		Name:     formName("12.00"),
		KeyLabel: "No. Rekening",
		Notes: []string{
			"Baris dibangun per rekening deposito unik (Form 12.00 - 3, PDF #page 204); posisi yang dibaca adalah AKHIR PERIODE laporan: kontrak sudah ditempatkan (start_date <= akhir periode) dan belum ditutup saat itu, bukan keadaan saat ekspor dijalankan.",
			"Kolom II ID Pihak Lawan memakai nomor CIF internal nasabah (BAB II Lampiran II), bukan sandi OJK.",
			"Kolom V Golongan Nasabah memakai sandi Lampiran 02 dan kolom VI Lokasi Nasabah memakai sandi Lampiran 03 (customers.ojk_pihak_lawan_code / ojk_kabupaten_code); sandi yang belum diisi ditulis '-'.",
			"Kolom VII Jangka Waktu memuat tanggal mulai dan jatuh tempo perjanjian terakhir. Kolom VIII Suku Bunga diisi hanya untuk deposito berbunga (profit_type INTEREST); margin/bagi hasil syariah bukan suku bunga sehingga ditulis '-'.",
			"Kolom IX Nominal adalah nilai nominal deposito pada tanggal laporan. Kolom XIII Jumlah ditulis sama dengan Nominal karena biaya transaksi belum diamortisasi (kolom XII) belum tersimpan (Form 12.00 - 3, PDF #page 205).",
			"Kolom X, XI, XII, XIV, XV, XVI, dan XVII belum punya sumber tersimpan; masing-masing ditulis '-' beserta alasan, bukan nol. Nomor Identitas tidak dipaparkan karena NIK/NPWP tersimpan terenkripsi (keputusan privasi).",
		},
	}
	for _, c := range form12Columns {
		if c.Reason != "" {
			sec.Unavailable = append(sec.Unavailable, ColumnUnavailable{Sandi: c.Sandi, Nama: c.Nama, Reason: c.Reason})
			continue
		}
		sec.Columns = append(sec.Columns, TableColumn{Sandi: c.Sandi, Nama: c.Nama})
	}
	for _, r := range rows {
		row := TableRow{Key: dashIfEmpty(r.AccountNumber)}
		for _, c := range form12Columns {
			if c.Reason != "" || c.Value == nil {
				continue
			}
			row.Cells = append(row.Cells, TableCell{Sandi: c.Sandi, Nama: c.Nama, Value: c.Value(r)})
		}
		sec.Rows = append(sec.Rows, row)
	}
	pasangKolomSandiKantor(&sec, kantor)
	return sec
}
