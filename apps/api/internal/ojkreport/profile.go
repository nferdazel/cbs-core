package ojkreport

import (
	"strings"

	"cbs-core/apps/core-api/internal/domain"
)

// profile.go membangun Form 00.00 INFORMASI POKOK BPR dari konfigurasi bank.
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024, Form 00.00 – 1 (hlm. 15-16) dan
// Form 00.00 – 2 PENJELASAN INFORMASI POKOK BPR (hlm. 17 dst.).
//
// Data profil bank diambil dari tabel bank_profile (migrasi 000025) dan system_config
// (kunci ojk.* pada migrasi 000046). Tidak ada nilai yang dikarang: field yang belum
// diisi didaftarkan beserta kunci konfigurasi yang harus diisi bank. Bila nama bank
// belum diisi, seluruh form dinyatakan belum tersedia.

// form00Field adalah satu field Form 00.00.
type form00Field struct {
	Label string
	// Value diambil dari konfigurasi; kosong berarti belum tersedia.
	Value func(BankProfileConfig) string
	// Reason non-kosong berarti field tidak punya sumber data sama sekali.
	Reason string
	// ConfigKey, bila diisi, disebutkan pada alasan field yang belum dikonfigurasi.
	ConfigKey string
}

var form00Fields = []form00Field{
	{Label: "1. Nama BPR", Value: func(c BankProfileConfig) string { return c.Name }},
	{Label: "2. Alamat BPR", Value: func(c BankProfileConfig) string { return c.Address }},
	{Label: "3. Kabupaten/Kota", Value: func(c BankProfileConfig) string { return c.City },
		ConfigKey: OJKBankCityCodeKey},
	{Label: "4. Wilayah Kerja OJK", Value: func(c BankProfileConfig) string { return c.OJKRegionCode },
		ConfigKey: OJKBankOJKRegionKey},
	{Label: "5. No. Telepon", Value: func(c BankProfileConfig) string { return c.Phone }},
	{Label: "6. E-mail", Value: func(c BankProfileConfig) string { return c.Email },
		ConfigKey: OJKBankEmailKey},
	{Label: "7. Situs Web BPR", Value: func(c BankProfileConfig) string { return c.Website },
		ConfigKey: OJKBankWebsiteKey},
	{Label: "8. NPWP", Value: func(c BankProfileConfig) string { return c.NPWP }},
	{Label: "9.a Nama Penanggung Jawab Laporan", Value: func(c BankProfileConfig) string { return c.PICName },
		ConfigKey: OJKPICNameKey},
	{Label: "9.b Bagian/Divisi Penanggung Jawab Laporan", Value: func(c BankProfileConfig) string { return c.PICDivision },
		ConfigKey: OJKPICDivisionKey},
	{Label: "9.c No. Telepon Penanggung Jawab Laporan", Value: func(c BankProfileConfig) string { return c.PICPhone },
		ConfigKey: OJKPICPhoneKey},
	{Label: "9.d E-mail Penanggung Jawab Laporan", Value: func(c BankProfileConfig) string { return c.PICEmail },
		ConfigKey: OJKPICEmailKey},

	// Butir 10 s.d. 21 kini punya kunci system_config (migrasi 000096) sehingga bank
	// dapat mengisinya lewat API; kosong tetap berarti belum tersedia.
	{Label: "10. Dividen yang Dibayar", Value: func(c BankProfileConfig) string { return c.DividendsPaid },
		ConfigKey: domain.OJKDividendsPaidKey},
	{Label: "11. Bonus Tahunan dan Tantiem", Value: func(c BankProfileConfig) string { return c.AnnualBonusTantiem },
		ConfigKey: domain.OJKAnnualBonusKey},
	{Label: "12. Informasi Audit Laporan Keuangan Tahunan (KAP/AP)", Value: func(c BankProfileConfig) string { return c.AuditInfo },
		ConfigKey: domain.OJKAuditInfoKey},
	{Label: "13. Nilai Nominal per Lembar Saham", Value: func(c BankProfileConfig) string { return c.ShareNominalValue },
		ConfigKey: domain.OJKShareNominalKey},
	{Label: "14. Status Penawaran Umum Efek", Value: func(c BankProfileConfig) string { return c.PublicOfferingStatus },
		ConfigKey: domain.OJKPublicOfferingKey},
	{Label: "15. Pedagang Valuta Asing (PVA)", Value: func(c BankProfileConfig) string { return c.PVAStatus },
		ConfigKey: domain.OJKPVAStatusKey},
	{Label: "16. Layanan Perbankan Elektronik (E-Banking)", Value: func(c BankProfileConfig) string { return c.EBankingStatus },
		ConfigKey: domain.OJKEBankingKey},
	{Label: "17. Penyelenggara Teknologi Informasi", Value: func(c BankProfileConfig) string { return c.ITProvider },
		ConfigKey: domain.OJKITProviderKey},
	{Label: "18. Penyelenggara Laku Pandai", Value: func(c BankProfileConfig) string { return c.LakuPandaiProvider },
		ConfigKey: domain.OJKLakuPandaiProvKey},
	{Label: "19. Jumlah Agen Laku Pandai", Value: func(c BankProfileConfig) string { return c.LakuPandaiAgentCount },
		ConfigKey: domain.OJKLakuPandaiAgentKey},
	{Label: "20. Informasi RUPS Perubahan Kepemilikan", Value: func(c BankProfileConfig) string { return c.RUPSOwnershipChange },
		ConfigKey: domain.OJKRUPSOwnershipKey},
	{Label: "21. Nama Ultimate Shareholders", Value: func(c BankProfileConfig) string { return c.UltimateShareholders },
		ConfigKey: domain.OJKUltimateHolderKey},
}

// buildForm00 menyusun Form 00.00. ok=false berarti identitas inti bank belum
// dikonfigurasi sehingga seluruh form belum dapat dibangun.
func buildForm00(cfg *BankProfileConfig) (TableSection, bool) {
	sec := TableSection{
		Form:        "00.00",
		Name:        formName("00.00"),
		KeyLabel:    "Field",
		Columns:     []TableColumn{{Sandi: "NILAI", Nama: "Nilai"}},
		Unavailable: nil,
		Notes: []string{
			"Field yang kosong berarti belum dikonfigurasi, bukan bernilai nol. Isi tabel bank_profile dan kunci ojk.* pada system_config.",
		},
	}
	if cfg == nil || !cfg.Configured {
		return sec, false
	}
	for _, f := range form00Fields {
		if f.Reason != "" {
			sec.Rows = append(sec.Rows, TableRow{Key: f.Label, Reason: f.Reason})
			continue
		}
		v := strings.TrimSpace(f.Value(*cfg))
		if v == "" {
			reason := "belum dikonfigurasi"
			if f.ConfigKey != "" {
				reason = "belum dikonfigurasi; isi kunci konfigurasi " + f.ConfigKey
			}
			sec.Rows = append(sec.Rows, TableRow{Key: f.Label, Reason: reason})
			continue
		}
		sec.Rows = append(sec.Rows, TableRow{
			Key:   f.Label,
			Cells: []TableCell{{Sandi: "NILAI", Nama: "Nilai", Value: v}},
		})
	}
	return sec, true
}
