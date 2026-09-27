package ojkreport

import (
	"strings"

	"cbs-core/apps/core-api/internal/domain"
)

// form05.go membangun Form 05.00 DAFTAR PENEMPATAN PADA BANK LAIN dari penanda
// lps_placements (migrasi 000045).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024, Form 05.00 – 1 (hlm. 85-87) dan
// Form 05.00 – 2 SANDI DAFTAR PENEMPATAN PADA BANK LAIN (hlm. 88-90).
//
// BATAS SUMBER: lps_placements hanya menyimpan penempatan yang secara eksplisit
// ditandai sebagai pengurang PPKA Pasal 23 POJK No. 1 Tahun 2024. Tabel itu BUKAN
// register lengkap penempatan pada bank lain; bank yang belum mengisi penandanya
// akan mendapat daftar yang tidak lengkap. Batas ini dicatat pada Notes.

// form05Column adalah satu kolom Form 05.00; Reason != "" berarti belum tersedia.
type form05Column struct {
	Sandi  string
	Nama   string
	Reason string
	Value  func(PlacementRow) string
}

const (
	form05SandiKantor      = "I"
	form05SandiBank        = "II"
	form05SandiLokasi      = "III"
	form05SandiJenis       = "IV"
	form05SandiHubungan    = "V"
	form05SandiJangkaWaktu = "VI"
	form05SandiKualitas    = "VII"
	form05SandiSukuBunga   = "VIII"
	form05SandiJumlah      = "IX"
	form05SandiDiblokir    = "X"
	form05SandiAlasan      = "XI"
	form05SandiCKPN        = "XII"
	form05SandiBungaAkan   = "XIII"
	form05SandiBungaProses = "XIV"
	form05SandiBMPK        = "XV"
	form05SandiIDPihak     = "XVI"
	form05SandiCKPNBaik    = "XVII"
	form05SandiCKPNKurang  = "XVIII"
	form05SandiCKPNTidak   = "XIX"
	form05SandiKlasifikasi = "XX"
	form05SandiJenisCKPN   = "XXI"
)

// form05Columns adalah susunan kolom Form 05.00.
var form05Columns = []form05Column{
	{Sandi: form05SandiKantor, Nama: "Sandi Kantor", Value: func(p PlacementRow) string {
		return dashIfEmpty(p.BranchCode)
	}},
	{Sandi: form05SandiBank, Nama: "Sandi Bank", Reason: "sandi bank lawan menurut Sistem Pelaporan OJK belum dipetakan; yang tersimpan hanya nama bank lawan"},
	{Sandi: form05SandiLokasi, Nama: "Lokasi Bank", Value: func(p PlacementRow) string {
		return dashIfEmpty(p.OJKKabupatenCode)
	}},
	{Sandi: form05SandiJenis, Nama: "Jenis", Value: func(p PlacementRow) string {
		return sandiJenisPenempatan(p.PlacementType)
	}},
	{Sandi: form05SandiHubungan, Nama: "Hubungan dengan Bank", Value: func(p PlacementRow) string {
		// Sandi inline Lampiran II Form 05.00-2 (docs/LAMPIRAN-OJK.md): 12 terkait,
		// 20 tidak terkait. Diisi bank lewat SQL/seed (lps_placements.ojk_hubungan_bank_code).
		return dashIfEmpty(p.OJKHubunganBankCode)
	}},
	{Sandi: form05SandiJangkaWaktu, Nama: "Jangka Waktu", Value: func(p PlacementRow) string {
		if p.StartDate == nil {
			return "-"
		}
		jangka := p.StartDate.Format("02-01-2006")
		if p.MaturityDate != nil {
			jangka += " s.d. " + p.MaturityDate.Format("02-01-2006")
		}
		return jangka
	}},
	{Sandi: form05SandiKualitas, Nama: "Kualitas", Value: func(p PlacementRow) string {
		return sandiKualitasPenempatan(p.Collectibility)
	}},
	{Sandi: form05SandiSukuBunga, Nama: "Suku Bunga", Value: func(p PlacementRow) string {
		if !p.InterestRateAnnual.IsPositive() {
			return "-"
		}
		return p.InterestRateAnnual.StringFixed(2)
	}},
	{Sandi: form05SandiJumlah, Nama: "Jumlah", Value: func(p PlacementRow) string {
		return FormatRupiah(p.Outstanding)
	}},
	{Sandi: form05SandiDiblokir, Nama: "Nominal yang Diblokir/Dijaminkan", Value: func(p PlacementRow) string {
		if p.BlockedAmount == nil {
			return "-"
		}
		return FormatRupiah(*p.BlockedAmount)
	}},
	{Sandi: form05SandiAlasan, Nama: "Alasan Diblokir", Value: func(p PlacementRow) string {
		// Daftar sandi alasan diblokir tidak tersurat di repo; nilainya disimpan apa
		// adanya dan baris tanpa isian ditulis "-", bukan ditebak dari nominal.
		return dashIfEmpty(p.OJKAlasanDiblokirCode)
	}},
	{Sandi: form05SandiCKPN, Nama: "CKPN", Value: func(p PlacementRow) string {
		if p.CKPN == nil {
			return "-"
		}
		return FormatRupiah(p.CKPN.RequiredCKPN)
	}},
	{Sandi: form05SandiBungaAkan, Nama: "Pendapatan Bunga yang Akan Diterima", Value: func(p PlacementRow) string {
		if p.AccruedInterestReceivable == nil {
			return "-"
		}
		return FormatRupiah(*p.AccruedInterestReceivable)
	}},
	{Sandi: form05SandiBungaProses, Nama: "Pendapatan Bunga Dalam Penyelesaian", Value: func(p PlacementRow) string {
		if p.AccruedInterestPending == nil {
			return "-"
		}
		return FormatRupiah(*p.AccruedInterestPending)
	}},
	{Sandi: form05SandiBMPK, Nama: "Status BMPK Individu", Value: func(p PlacementRow) string {
		// Status batas pihak terkait bank lawan (lps_placements.customer_id, migrasi
		// 000103) yang dimuat satu kali oleh perakit. Penempatan yang belum ditautkan
		// ke pihak terkait ditulis "-", bukan dianggap sesuai batas.
		return dashIfEmpty(p.BMPKStatus)
	}},
	{Sandi: form05SandiIDPihak, Nama: "ID Pihak Lawan", Value: func(p PlacementRow) string {
		// ID Pihak Lawan = CIF internal bank lawan (BAB II Lampiran II), bukan sandi
		// OJK; harus sama dengan CIF pada SLIK. Diisi bank lewat SQL/seed.
		return dashIfEmpty(p.CounterpartyCIF)
	}},
	{Sandi: form05SandiCKPNBaik, Nama: "Cadangan Kerugian Penurunan Nilai Aset Baik", Value: func(p PlacementRow) string {
		return ckpnGolonganKualitas(p, "LANCAR")
	}},
	{Sandi: form05SandiCKPNKurang, Nama: "Cadangan Kerugian Penurunan Nilai Aset Kurang Baik", Value: func(p PlacementRow) string {
		return ckpnGolonganKualitas(p, "KURANG_LANCAR")
	}},
	{Sandi: form05SandiCKPNTidak, Nama: "Cadangan Kerugian Penurunan Nilai Aset Tidak Baik", Value: func(p PlacementRow) string {
		return ckpnGolonganKualitas(p, "MACET")
	}},
	{Sandi: form05SandiKlasifikasi, Nama: "Klasifikasi Aset Keuangan", Reason: "klasifikasi SAK EP belum dipetakan per penempatan"},
	{Sandi: form05SandiJenisCKPN, Nama: "Jenis CKPN", Value: func(p PlacementRow) string {
		if p.CKPN == nil {
			return "-"
		}
		return domain.SandiJenisCKPNPABL(domain.PABLCKPNMethod(p.CKPN.Method))
	}},
}

// buildForm05 menyusun Form 05.00. Bila tidak ada baris penempatan, bagian Rows
// kosong tetapi daftar kolom yang belum tersedia tetap dibawa. Kolom XII (CKPN) dan
// XXI (Jenis CKPN) hanya tersedia bila minimal satu baris sudah diasesmen; selama
// belum ada, keduanya dinyatakan belum tersedia dengan alasan yang menyebut saklar
// ckpn.pabl.enabled dan asesmen.
func buildForm05(rows []PlacementRow) TableSection {
	sec := TableSection{
		Form:     "05.00",
		Name:     formName("05.00"),
		KeyLabel: "Bank Lawan",
		Notes: []string{
			"Dibangun dari penanda lps_placements (migrasi 000045), yaitu penempatan yang secara eksplisit ditandai untuk pengurang PPKA Pasal 23 POJK 1/2024. Tabel itu bukan register lengkap seluruh penempatan pada bank lain.",
			"Jenis penempatan KREDIT dan LAINNYA ditempatkan sebagai '-' karena Form 05.00 – 2 hanya memberi sandi giro/tabungan/deposito/sertifikat deposito.",
			"Kolom XII dan XXI hanya berisi CKPN penempatan yang sudah diasesmen (saklar ckpn.pabl.enabled dan asesmen tersimpan); penempatan lain ditulis '-'.",
			"Kolom XVII-XIX memisahkan CKPN menurut golongan kualitas aset: Baik=Lancar, Kurang Baik=Kurang Lancar, Tidak Baik=Macet (PABL tidak mengenal kualitas Dalam Perhatian Khusus maupun Diragukan). CKPN satu penempatan hanya muncul pada kolom golongannya; kolom lain ditulis '-'.",
			"Status BMPK Individu (kolom XV) diambil dari hasil uji batas modul BMPK untuk nasabah yang ditautkan ke penempatan (lps_placements.customer_id); penempatan yang belum ditautkan ditulis '-' (bukan dianggap sesuai batas).",
		},
	}

	adaCKPN := false
	adaJangkaWaktu := false
	adaSukuBunga := false
	adaBMPK := false
	for _, p := range rows {
		if p.CKPN != nil {
			adaCKPN = true
		}
		if p.StartDate != nil {
			adaJangkaWaktu = true
		}
		if p.InterestRateAnnual.IsPositive() {
			adaSukuBunga = true
		}
		if strings.TrimSpace(p.BMPKStatus) != "" {
			adaBMPK = true
		}
		if adaCKPN && adaJangkaWaktu && adaSukuBunga && adaBMPK {
			break
		}
	}
	cols := form05Columns
	if !adaCKPN || !adaJangkaWaktu || !adaSukuBunga || !adaBMPK {
		cols = make([]form05Column, len(form05Columns))
		copy(cols, form05Columns)
		for i := range cols {
			switch cols[i].Sandi {
			case form05SandiCKPN, form05SandiJenisCKPN, form05SandiCKPNBaik, form05SandiCKPNKurang, form05SandiCKPNTidak:
				if !adaCKPN {
					cols[i].Reason = "CKPN per penempatan belum diasesmen (saklar ckpn.pabl.enabled + asesmen)"
				}
			case form05SandiJangkaWaktu:
				if !adaJangkaWaktu {
					cols[i].Reason = "tanggal mulai dan jatuh tempo penempatan belum disimpan pada lps_placements"
				}
			case form05SandiSukuBunga:
				if !adaSukuBunga {
					cols[i].Reason = "suku bunga tahunan penempatan belum disimpan pada lps_placements"
				}
			case form05SandiBMPK:
				if !adaBMPK {
					cols[i].Reason = "modul BMPK tidak tersedia atau belum ada penempatan ini yang ditautkan ke pihak terkait (lps_placements.customer_id) sehingga batasnya belum dapat diuji"
				}
			}
		}
	}

	for _, c := range cols {
		if c.Reason != "" {
			sec.Unavailable = append(sec.Unavailable, ColumnUnavailable{Sandi: c.Sandi, Nama: c.Nama, Reason: c.Reason})
			continue
		}
		sec.Columns = append(sec.Columns, TableColumn{Sandi: c.Sandi, Nama: c.Nama})
	}
	for _, p := range rows {
		row := TableRow{Key: dashIfEmpty(p.CounterpartyBank)}
		for _, c := range cols {
			if c.Reason != "" || c.Value == nil {
				continue
			}
			row.Cells = append(row.Cells, TableCell{Sandi: c.Sandi, Nama: c.Nama, Value: c.Value(p)})
		}
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}

// sandiJenisPenempatan memetakan jenis penempatan ke sandi Form 05.00 – 2 butir IV.
// KREDIT dan LAINNYA tidak punya sandi pada kolom IV sehingga ditulis "-".
func sandiJenisPenempatan(t string) string {
	switch normalizePlacementType(t) {
	case "GIRO":
		return "10"
	case "TABUNGAN":
		return "20"
	case "DEPOSITO":
		return "30"
	case "SERTIFIKAT_DEPOSITO":
		return "40"
	default:
		return "-"
	}
}

// sandiKualitasPenempatan memetakan kualitas PABL ke sandi Form 05.00 – 2 butir VII:
// 1 Lancar, 3 Kurang Lancar, 5 Macet (Pasal 15 ayat (1) POJK 1/2024).
func sandiKualitasPenempatan(k string) string {
	switch normalizePlacementCollectibility(k) {
	case "LANCAR":
		return "1"
	case "KURANG_LANCAR":
		return "3"
	case "MACET":
		return "5"
	default:
		return "-"
	}
}

// ckpnGolonganKualitas mengisi kolom XVII-XIX: CKPN satu penempatan hanya muncul pada
// kolom golongan kualitasnya sendiri. Penempatan tanpa asesmen atau di luar golongan
// itu ditulis "-", bukan nol, agar tidak terbaca sebagai CKPN yang memang dibentuk.
func ckpnGolonganKualitas(p PlacementRow, golongan string) string {
	if p.CKPN == nil || normalizePlacementCollectibility(p.Collectibility) != golongan {
		return "-"
	}
	return FormatRupiah(p.CKPN.RequiredCKPN)
}

func normalizePlacementType(t string) string {
	s := strings.ToUpper(strings.TrimSpace(t))
	switch s {
	case "SERTIFIKAT DEPOSITO", "SERTIFIKAT_DEPOSITO":
		return "SERTIFIKAT_DEPOSITO"
	default:
		return s
	}
}

func normalizePlacementCollectibility(k string) string {
	s := strings.ToUpper(strings.TrimSpace(k))
	switch s {
	case "KURANG LANCAR", "KURANG_LANCAR":
		return "KURANG_LANCAR"
	default:
		return s
	}
}
