package ojkreport

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// repo_source.go menghubungkan builder dengan repositori yang sudah ada tanpa
// menambah dependensi: kredit (loans), profil bank (bank_profile), konfigurasi
// (system_config), penempatan pada bank lain (lps_placements), dan modul KPMM.
//
// RepoSource mengimplementasikan Source (lewat embedding) sekaligus kontrak data
// opsional LoanDataSource, BankProfileSource, PlacementDataSource, dan KPMMSource.
// Bila builder tidak menerima sumber opsional, form/baris terkait ditandai belum
// tersedia.
type RepoSource struct {
	Source
	Loans      domain.LoanRepository
	Profile    domain.BankProfileRepository
	Config     domain.SystemConfigRepository
	Placements domain.LPSPlacementRepository
	// Customers memasok sandi referensi OJK per nasabah (Form 06.00 kolom XVIII
	// dan XX). Dibaca sekaligus lewat GetByIDs untuk menghindari N+1; bila nil,
	// kode dibiarkan kosong dan laporan menulis "-".
	Customers domain.CustomerRepository
	// KPMM mengisi baris KPMM Form 00.08. Bila nil, Form 00.08 menulis baris KPMM
	// sebagai tidak tersedia (bukan nol).
	KPMM domain.KPMMService
	// BMPK menyediakan status batas per pihak terkait untuk kolom Status BMPK Form
	// 05.00/06.00 dan laporan LAPORAN_BMPK. Bila nil, kolom BMPK dinyatakan belum
	// tersedia; nilainya tidak dikarang.
	BMPK domain.BMPKService
	// Savings menyediakan agregasi internal jenis nasabah per produk simpanan
	// (rekening tabungan/giro dan deposito berjangka) — dulu diberi nomor Form 00.14,
	// nomor itu tidak ada di SEOJK 16/2024 (docs/CELAH-FORM-OJK.md §6). Bila nil,
	// agregasi dinyatakan tidak tersedia; tidak ada angka yang dikarang.
	Savings domain.SavingsCustomerRepository
	// BankDeposits menyediakan agregasi Form 13.00 simpanan dari bank lain (rekening
	// tabungan/giro dan deposito berjangka milik nasabah bergolongan bank). Bila nil,
	// form dinyatakan belum tersedia; tidak ada angka yang dikarang.
	BankDeposits domain.BankDepositRepository
	// SavingsAccounts menyediakan baris per rekening tabungan Form 11.00 pada posisi
	// akhir periode. Bila nil, form dinyatakan belum tersedia; tidak ada angka yang
	// dikarang.
	SavingsAccounts domain.SavingsAccountReportRepository
	// TimeDeposits menyediakan baris per kontrak deposito berjangka Form 12.00 pada
	// posisi akhir periode. Bila nil, form dinyatakan belum tersedia.
	TimeDeposits domain.TimeDepositReportRepository
	// OffBalance menyediakan agregat Form 01.01 rekening administratif (pos komitmen/
	// kontinjensi off-balance). Bila nil, form dinyatakan belum tersedia; tidak ada
	// angka yang dikarang.
	OffBalance domain.OffBalanceRepository
	// AYDA menyediakan register per kasus Form 07.00 (agunan yang diambil alih).
	// Bila nil, form dinyatakan belum tersedia; tidak ada angka yang dikarang.
	AYDA domain.AYDARegisterRepository
	// Kepemilikan menyediakan register pemegang saham Form 00.01 (data kepemilikan
	// BPR). Bila nil, form dinyatakan belum tersedia; tidak ada angka yang dikarang.
	Kepemilikan domain.KepemilikanRegisterRepository
	// Pinjaman menyediakan register pinjaman yang diterima Form 00.07 (dari bank/Bank
	// Indonesia/pihak ketiga bukan bank). Bila nil, form dinyatakan belum tersedia;
	// tidak ada angka yang dikarang.
	Pinjaman domain.PinjamanRegisterRepository
	// Properti menyediakan register properti terbengkalai Form 17.00 (properti
	// terbengkalai yang bank catat). Bila nil, form dinyatakan belum tersedia; tidak ada
	// angka yang dikarang.
	Properti domain.PropertiRegisterRepository
	// AsetTetap menyediakan register aset tetap, inventaris, dan aset tidak berwujud
	// Form 08.00 (aset_tetap_register, migrasi 000119). Bila nil, form dinyatakan belum
	// tersedia; tidak ada angka yang dikarang.
	AsetTetap domain.AsetTetapRegisterRepository
	// Penyertaan menyediakan register penyertaan modal Form 16.00
	// (penyertaan_modal_register, migrasi 000120). Bila nil, form dinyatakan belum
	// tersedia; tidak ada angka yang dikarang.
	Penyertaan domain.PenyertaanRegisterRepository
	// AsetKeuangan menyediakan register aset keuangan lainnya Form 18.00
	// (aset_keuangan_lainnya_register, migrasi 000121). Bila nil, form dinyatakan belum
	// tersedia; tidak ada angka yang dikarang.
	AsetKeuangan domain.AsetKeuanganRegisterRepository
	// SuratBerharga menyediakan register surat berharga Form 04.00
	// (surat_berharga_register, migrasi 000122). Bila nil, form dinyatakan belum
	// tersedia; tidak ada angka yang dikarang.
	SuratBerharga domain.SuratBerhargaRegisterRepository
	// KasValas menyediakan register kas valuta asing Form 03.00
	// (kas_valas_register, migrasi 000123). Bila nil, form dinyatakan belum tersedia;
	// tidak ada angka yang dikarang.
	KasValas domain.KasValasRegisterRepository
	// KreditSindikasi menyediakan register kredit sindikasi Form 06.02
	// (kredit_sindikasi_register, migrasi 000124). Bila nil, form dinyatakan belum
	// tersedia; tidak ada angka yang dikarang.
	KreditSindikasi domain.SindikasiRegisterRepository
	// Kelembagaan menyediakan jaringan kantor (bank_offices) untuk memilih kantor
	// pelapor kolom I "Sandi Kantor" pada form bank-wide 09.00/01.01. Bila nil,
	// kolom I dinyatakan tidak tersedia; sandinya tidak dikarang.
	Kelembagaan domain.KelembagaanRepository
	// KelembagaanSvc menyediakan laporan kelembagaan terhitung (bank_offices dan
	// bank_management) untuk Form 00.02/00.03/00.04 pada bundel bulanan. Field ini
	// memakai layanan kelembagaan yang sama dengan LAPORAN_KELEMBAGAAN. Bila nil,
	// ketiga form dicatat belum tersedia beserta alasannya, bukan tampil kosong.
	KelembagaanSvc domain.KelembagaanService
}

// pageSizeKredit membatasi jumlah kredit per halaman pembacaan.
const pageSizeKredit = 500

// maxHalamanKredit mencegah pengulangan tanpa batas bila total dari repositori
// berubah di tengah pembacaan.
const maxHalamanKredit = 1000

// ListLoansForOJK membaca seluruh kredit bank-wide. Kebijakan bank-wide ditegakkan
// di lapisan data: aktor non-lintas cabang ditolak, bukan diberi sebagian.
//
// asOf dipakai menghitung tunggakan: angsuran yang jatuh tempo sebelum asOf dan
// belum lunas. Baris juga dibatasi kredit yang sudah cair pada akhir periode
// (loans.disbursed_at <= asOf, granularitas hari) agar kredit yang baru cair setelah
// periode tidak ikut pada laporan periode lampau. Kredit tanpa tanggal pencairan
// dibiarkan (data lama), bukan dikeluarkan tanpa bukti. Agregat jadwal diambil dengan
// SATU query per kredit (GROUP BY loan_id, lihat ListLoanScheduleAggregates), lalu
// dipetakan lewat nomor kredit yang unik; bukan satu query per kredit.
func (s RepoSource) ListLoansForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]LoanRow, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.Loans == nil {
		return nil, nil
	}
	aggregates, err := s.Loans.ListLoanScheduleAggregates(ctx, asOf, actor)
	if err != nil {
		return nil, err
	}
	agregat := make(map[string]domain.LoanScheduleAggregate, len(aggregates))
	for _, a := range aggregates {
		agregat[a.LoanNumber] = a
	}

	// Sumber as-of lain (Form 11.00/12.00) membandingkan tanggal pada granularitas
	// hari (::date), jadi pencairan pada hari akhir periode tetap ikut.
	asOfDate := asOf.UTC().Truncate(24 * time.Hour)

	var out []LoanRow
	offset := 0
	for page := 0; page < maxHalamanKredit; page++ {
		items, total, err := s.Loans.List(ctx, pageSizeKredit, offset, actor)
		if err != nil {
			return nil, err
		}
		for i := range items {
			if d := items[i].DisbursedAt; d != nil && d.UTC().Truncate(24*time.Hour).After(asOfDate) {
				continue
			}
			row := loanRowDariDomain(items[i])
			if a, ok := agregat[row.LoanNumber]; ok {
				row.FirstInstallmentDate = a.FirstInstallmentDate
				row.OverdueUnpaid = a.OverdueUnpaid
				row.AccruedProfit = a.AccruedProfit
			}
			out = append(out, row)
		}
		offset += len(items)
		if len(items) == 0 || offset >= total {
			break
		}
	}
	if err := s.lengkapiSandiReferensiCustomer(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// lengkapiSandiReferensiCustomer mengisi sandi pihak lawan (kolom XVIII), sektor
// ekonomi (kolom XX), hubungan dengan bank (kolom IX), dan ID Pihak Lawan/CIF (kolom
// II) tiap baris kredit dari SATU panggilan GetByIDs untuk seluruh nasabah yang
// berbeda, bukan satu query per kredit. Baris tanpa nasabah terbaca atau tanpa sandi
// dibiarkan kosong; form terkait menulis "-".
func (s RepoSource) lengkapiSandiReferensiCustomer(ctx context.Context, rows []LoanRow) error {
	if s.Customers == nil || len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	terlihat := make(map[uuid.UUID]struct{}, len(rows))
	for _, r := range rows {
		id, err := uuid.Parse(r.CustomerID)
		if err != nil {
			continue
		}
		if _, ok := terlihat[id]; ok {
			continue
		}
		terlihat[id] = struct{}{}
		ids = append(ids, id)
	}
	records, err := s.Customers.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range rows {
		id, err := uuid.Parse(rows[i].CustomerID)
		if err != nil {
			continue
		}
		rec, ok := records[id]
		if !ok {
			continue
		}
		rows[i].OJKPihakLawanCode = rec.OJKPihakLawanCode
		rows[i].OJKSektorEkonomiCode = rec.OJKSektorEkonomiCode
		rows[i].OJKHubunganBankCode = rec.OJKHubunganBankCode
		// Kolom II ID Pihak Lawan = nomor CIF internal nasabah (sama dengan SLIK),
		// bukan sandi OJK; tinggal disingkapkan dari nasabah yang sudah dibaca.
		rows[i].IDPihakLawan = rec.CIFNumber
	}
	return nil
}

// GetBankProfileConfig membaca identitas bank dari bank_profile dan kunci ojk.*.
func (s RepoSource) GetBankProfileConfig(ctx context.Context) (*BankProfileConfig, error) {
	cfg := &BankProfileConfig{}
	if s.Profile != nil {
		p, err := s.Profile.Get(ctx)
		if err != nil {
			return nil, err
		}
		if p != nil {
			cfg.Name = p.Name
			cfg.Address = p.Address
			cfg.City = p.City
			cfg.Phone = p.Phone
			cfg.NPWP = p.NPWP
		}
	}
	if s.Config != nil {
		values, err := s.Config.GetAll(ctx)
		if err != nil {
			return nil, err
		}
		cfg.Email = values[OJKBankEmailKey]
		cfg.Website = values[OJKBankWebsiteKey]
		cfg.CityCode = values[OJKBankCityCodeKey]
		cfg.OJKRegionCode = values[OJKBankOJKRegionKey]
		cfg.PICName = values[OJKPICNameKey]
		cfg.PICDivision = values[OJKPICDivisionKey]
		cfg.PICPhone = values[OJKPICPhoneKey]
		cfg.PICEmail = values[OJKPICEmailKey]
		// Butir 10 s.d. 21 Form 00.00 (migrasi 000096).
		cfg.DividendsPaid = values[domain.OJKDividendsPaidKey]
		cfg.AnnualBonusTantiem = values[domain.OJKAnnualBonusKey]
		cfg.AuditInfo = values[domain.OJKAuditInfoKey]
		cfg.ShareNominalValue = values[domain.OJKShareNominalKey]
		cfg.PublicOfferingStatus = values[domain.OJKPublicOfferingKey]
		cfg.PVAStatus = values[domain.OJKPVAStatusKey]
		cfg.EBankingStatus = values[domain.OJKEBankingKey]
		cfg.ITProvider = values[domain.OJKITProviderKey]
		cfg.LakuPandaiProvider = values[domain.OJKLakuPandaiProvKey]
		cfg.LakuPandaiAgentCount = values[domain.OJKLakuPandaiAgentKey]
		cfg.RUPSOwnershipChange = values[domain.OJKRUPSOwnershipKey]
		cfg.UltimateShareholders = values[domain.OJKUltimateHolderKey]
	}
	cfg.Configured = strings.TrimSpace(cfg.Name) != ""
	return cfg, nil
}

// KPMMModalATMR menyediakan modal (total) dan ATMR dari modul KPMM untuk baris
// KPMM Form 00.08. tersedia=false berarti modal/ATMR belum lengkap (mis. CKPN
// belum dihitung) sehingga baris harus ditulis tidak tersedia, bukan nol.
func (s RepoSource) KPMMModalATMR(ctx context.Context, asOf time.Time, book string, actor domain.Actor) (decimal.Decimal, decimal.Decimal, bool) {
	if s.KPMM == nil {
		return decimal.Zero, decimal.Zero, false
	}
	report, err := s.KPMM.Hitung(ctx, asOf, book, actor)
	if err != nil {
		return decimal.Zero, decimal.Zero, false
	}
	if !report.TotalModal.Tersedia || !report.ATMR.Tersedia {
		return decimal.Zero, decimal.Zero, false
	}
	return report.TotalModal.Nilai, report.ATMR.Nilai, true
}

// ListPlacementsForOJK membaca penempatan pada bank lain bank-wide.
func (s RepoSource) ListPlacementsForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]PlacementRow, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.Placements == nil {
		return nil, nil
	}
	items, err := s.Placements.ListPlacements(ctx, asOf, actor)
	if err != nil {
		return nil, err
	}
	out := make([]PlacementRow, 0, len(items))
	for _, p := range items {
		row := PlacementRow{
			BranchCode:         p.BranchCode,
			CounterpartyBank:   p.CounterpartyBank,
			PlacementType:      string(p.PlacementType),
			Outstanding:        p.Outstanding,
			Collectibility:     string(p.Collectibility),
			AsOf:               p.AsOf,
			StartDate:          p.StartDate,
			MaturityDate:       p.MaturityDate,
			InterestRateAnnual: p.InterestRateAnnual,
			OJKKabupatenCode:   p.OJKKabupatenCode,
			// Kolom Form 05.00 K1 lain (migrasi 000106); kosong/nil ditulis "-".
			OJKHubunganBankCode:       p.OJKHubunganBankCode,
			BlockedAmount:             p.BlockedAmount,
			OJKAlasanDiblokirCode:     p.OJKAlasanDiblokirCode,
			AccruedInterestReceivable: p.AccruedInterestReceivable,
			AccruedInterestPending:    p.AccruedInterestPending,
			CounterpartyCIF:           p.CounterpartyCIF,
			OJKKlasifikasiAsetCode:    p.OJKKlasifikasiAsetCode,
			CustomerID:                customerIDString(p.CustomerID),
		}
		if p.CKPN != nil {
			row.CKPN = &PlacementCKPNRow{
				Method:       string(p.CKPN.Method),
				RequiredCKPN: p.CKPN.RequiredCKPN,
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// ListSavingsCustomerTypes membaca agregasi internal jenis nasabah per produk (dulu
// diberi nomor Form 00.14 — nomor itu tidak ada di SEOJK 16/2024, lihat
// docs/CELAH-FORM-OJK.md §6) bank-wide pada posisi akhir periode asOf. Kebijakan
// bank-wide ditegakkan di lapisan data: aktor non-lintas cabang ditolak, bukan diberi
// sebagian.
func (s RepoSource) ListSavingsCustomerTypes(ctx context.Context, asOf time.Time, actor domain.Actor) ([]SavingsCustomerTypeRow, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.Savings == nil {
		return nil, nil
	}
	aggregates, err := s.Savings.ListSavingsCustomerAggregates(ctx, asOf)
	if err != nil {
		return nil, err
	}
	out := make([]SavingsCustomerTypeRow, 0, len(aggregates))
	for _, a := range aggregates {
		out = append(out, SavingsCustomerTypeRow{
			ProductFamily:    string(a.ProductFamily),
			ProductCode:      a.ProductCode,
			ProductName:      a.ProductName,
			CustomerTypeCode: a.CustomerTypeCode,
			AccountCount:     a.AccountCount,
			TotalAmount:      a.TotalAmount,
		})
	}
	return out, nil
}

// ListBankDepositsForOJK membaca agregasi Form 13.00 bank-wide pada posisi akhir
// periode asOf dan memetakannya ke baris ekspor. Kebijakan bank-wide ditegakkan di
// lapisan data: aktor non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListBankDepositsForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]BankDepositRow, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.BankDeposits == nil {
		return nil, nil
	}
	aggregates, err := s.BankDeposits.ListBankDeposits(ctx, asOf)
	if err != nil {
		return nil, err
	}
	out := make([]BankDepositRow, 0, len(aggregates))
	for _, a := range aggregates {
		out = append(out, BankDepositRow{
			BranchCode:       a.BranchCode,
			CounterpartyCIF:  a.CounterpartyCIF,
			JenisBankCode:    a.JenisBankCode,
			HubunganBankCode: a.HubunganBankCode,
			LocationCode:     a.LocationCode,
			Jenis:            a.Jenis,
			AccountCount:     a.AccountCount,
			TotalNominal:     a.TotalNominal,
		})
	}
	return out, nil
}

// ListSavingsAccountsForOJK membaca baris per rekening tabungan bank-wide pada posisi
// akhir periode asOf dan memetakannya ke baris ekspor Form 11.00. Kebijakan bank-wide
// ditegakkan di lapisan data: aktor non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListSavingsAccountsForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]SavingsAccountRow, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.SavingsAccounts == nil {
		return nil, nil
	}
	aggregates, err := s.SavingsAccounts.ListSavingsAccountsForOJK(ctx, asOf)
	if err != nil {
		return nil, err
	}
	out := make([]SavingsAccountRow, 0, len(aggregates))
	for _, a := range aggregates {
		out = append(out, SavingsAccountRow{
			AccountNumber:      a.AccountNumber,
			CounterpartyCIF:    a.CounterpartyCIF,
			CustomerTypeCode:   a.CustomerTypeCode,
			HubunganBankCode:   a.HubunganBankCode,
			LocationCode:       a.LocationCode,
			ProfitScheme:       a.ProfitScheme,
			InterestRateAnnual: a.InterestRateAnnual,
			Balance:            a.Balance,
		})
	}
	return out, nil
}

// ListTimeDepositsForOJK membaca baris per kontrak deposito berjangka bank-wide pada
// posisi akhir periode asOf dan memetakannya ke baris ekspor Form 12.00. Kebijakan
// bank-wide ditegakkan di lapisan data: aktor non-lintas cabang ditolak.
func (s RepoSource) ListTimeDepositsForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]TimeDepositRow, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.TimeDeposits == nil {
		return nil, nil
	}
	aggregates, err := s.TimeDeposits.ListTimeDepositsForOJK(ctx, asOf)
	if err != nil {
		return nil, err
	}
	out := make([]TimeDepositRow, 0, len(aggregates))
	for _, a := range aggregates {
		out = append(out, TimeDepositRow{
			AccountNumber:    a.AccountNumber,
			CounterpartyCIF:  a.CounterpartyCIF,
			CustomerTypeCode: a.CustomerTypeCode,
			HubunganBankCode: a.HubunganBankCode,
			LocationCode:     a.LocationCode,
			ProfitType:       a.ProfitType,
			PlacementAmount:  a.PlacementAmount,
			StartDate:        a.StartDate,
			MaturityDate:     a.MaturityDate,
			ProfitRate:       a.ProfitRate,
		})
	}
	return out, nil
}

// ListOffBalanceForOJK membaca agregat Form 01.01 bank-wide (pos komitmen/kontinjensi
// pada bulan asOf). Kebijakan bank-wide ditegakkan di lapisan data: aktor non-lintas
// cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListOffBalanceForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.OffBalanceAggregate, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.OffBalance == nil {
		return nil, nil
	}
	return s.OffBalance.ListAggregates(ctx, asOf)
}

// ListAYDAForOJK membaca baris register AYDA bank-wide yang as_of-nya pada bulan asOf
// (kandidat Form 07.00). Kebijakan bank-wide ditegakkan di lapisan data: aktor
// non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListAYDAForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.AYDAItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.AYDA == nil {
		return nil, nil
	}
	return s.AYDA.ListAYDAForOJK(ctx, asOf)
}

// ListKepemilikanForOJK membaca baris register pemegang saham bank-wide yang as_of-nya
// pada bulan asOf (kandidat Form 00.01). Kebijakan bank-wide ditegakkan di lapisan
// data: aktor non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListKepemilikanForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.KepemilikanItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.Kepemilikan == nil {
		return nil, nil
	}
	return s.Kepemilikan.ListKepemilikanForOJK(ctx, asOf)
}

// ListPinjamanForOJK membaca baris register pinjaman yang diterima bank-wide yang
// as_of-nya pada bulan asOf (kandidat Form 00.07). Kebijakan bank-wide ditegakkan di
// lapisan data: aktor non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListPinjamanForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.PinjamanItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.Pinjaman == nil {
		return nil, nil
	}
	return s.Pinjaman.ListPinjamanForOJK(ctx, asOf)
}

// ListPropertiForOJK membaca baris register properti terbengkalai bank-wide yang
// as_of-nya pada bulan asOf (kandidat Form 17.00). Kebijakan bank-wide ditegakkan di
// lapisan data: aktor non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListPropertiForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.PropertiItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.Properti == nil {
		return nil, nil
	}
	return s.Properti.ListPropertiForOJK(ctx, asOf)
}

// ListAsetTetapForOJK membaca baris register aset bank-wide yang as_of-nya pada bulan
// asOf (kandidat Form 08.00). Kebijakan bank-wide ditegakkan di lapisan data: aktor
// non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListAsetTetapForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.AsetTetapItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.AsetTetap == nil {
		return nil, nil
	}
	return s.AsetTetap.ListAsetTetapForOJK(ctx, asOf)
}

// ListPenyertaanForOJK membaca baris register penyertaan modal bank-wide yang as_of-nya
// pada bulan asOf (kandidat Form 16.00). Kebijakan bank-wide ditegakkan di lapisan data:
// aktor non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListPenyertaanForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.PenyertaanItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.Penyertaan == nil {
		return nil, nil
	}
	return s.Penyertaan.ListPenyertaanForOJK(ctx, asOf)
}

// ListAsetKeuanganForOJK membaca baris register aset keuangan lainnya bank-wide yang
// as_of-nya pada bulan asOf (kandidat Form 18.00). Kebijakan bank-wide ditegakkan di
// lapisan data: aktor non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListAsetKeuanganForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.AsetKeuanganItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.AsetKeuangan == nil {
		return nil, nil
	}
	return s.AsetKeuangan.ListAsetKeuanganForOJK(ctx, asOf)
}

// ListSuratBerhargaForOJK membaca baris register surat berharga bank-wide yang as_of-nya
// pada bulan asOf (kandidat Form 04.00). Kebijakan bank-wide ditegakkan di lapisan data:
// aktor non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListSuratBerhargaForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.SuratBerhargaItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.SuratBerharga == nil {
		return nil, nil
	}
	return s.SuratBerharga.ListSuratBerhargaForOJK(ctx, asOf)
}

// ListKasValasForOJK membaca baris register kas valas bank-wide yang as_of-nya pada bulan
// asOf (kandidat Form 03.00). Kebijakan bank-wide ditegakkan di lapisan data: aktor
// non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListKasValasForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.KasValasItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.KasValas == nil {
		return nil, nil
	}
	return s.KasValas.ListKasValasForOJK(ctx, asOf)
}

// ListSindikasiForOJK membaca baris register kredit sindikasi bank-wide yang as_of-nya pada
// bulan asOf (kandidat Form 06.02). Kebijakan bank-wide ditegakkan di lapisan data: aktor
// non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListSindikasiForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.SindikasiItem, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.KreditSindikasi == nil {
		return nil, nil
	}
	return s.KreditSindikasi.ListSindikasiForOJK(ctx, asOf)
}

// ReportingOffice memilih satu kantor pelapor dari jaringan kantor bank (bank_offices,
// migrasi 000112). Aturan pemilihan ada di selectReportingOffice. Bila repositori
// kelembagaan belum dirangkai, kolom I tetap dinyatakan tidak tersedia, bukan diisi
// tebakan.
func (s RepoSource) ReportingOffice(ctx context.Context) (ReportingOffice, error) {
	if s.Kelembagaan == nil {
		return ReportingOffice{Reason: "sumber jaringan kantor (bank_offices) belum dikonfigurasi pada ekspor ini"}, nil
	}
	offices, err := s.Kelembagaan.ListOffices(ctx)
	if err != nil {
		return ReportingOffice{}, err
	}
	return selectReportingOffice(offices), nil
}

// selectReportingOffice memilih kantor pelapor untuk kolom I form bank-wide:
//   - hanya kantor berstatus AKTIF dan closed_at kosong;
//   - hanya kantor yang code-nya tidak kosong (code = sandi kantor yang bank pakai);
//   - TEPAT SATU hasil dipakai sebagai sandi dan nama kantor pelapor;
//   - NOL hasil -> tidak tersedia dengan alasan spesifik;
//   - LEBIH DARI SATU -> tidak tersedia dengan jumlahnya; sistem tidak menebak
//     "kantor pusat" dari office_type (teks bebas bank) dan tidak mengarang sandi.
func selectReportingOffice(offices []domain.BankOffice) ReportingOffice {
	var aktifBerSandi []domain.BankOffice
	for _, o := range offices {
		if o.Status != domain.KelembagaanKantorAktif {
			continue
		}
		if o.ClosedAt != nil && !o.ClosedAt.IsZero() {
			continue
		}
		if strings.TrimSpace(o.Code) == "" {
			continue
		}
		aktifBerSandi = append(aktifBerSandi, o)
	}
	switch len(aktifBerSandi) {
	case 0:
		return ReportingOffice{Reason: "belum ada kantor aktif ber-sandi pada bank_offices (status TUTUP, closed_at terisi, atau code kosong); kolom Sandi Kantor tidak dikarang"}
	case 1:
		return ReportingOffice{
			Sandi: strings.TrimSpace(aktifBerSandi[0].Code),
			Nama:  strings.TrimSpace(aktifBerSandi[0].Name),
		}
	default:
		return ReportingOffice{Reason: fmt.Sprintf("%d kantor aktif ber-sandi; sistem tidak memilih kantor pelapor sendiri", len(aktifBerSandi))}
	}
}

// KelembagaanReport meneruskan laporan kelembagaan terhitung agar Form 00.02/00.03/
// 00.04 ikut bundel bulanan. Bila layanan belum dirangkai, ErrKelembagaanSourceUnavailable
// dikembalikan supaya builder mencatat ketiga form sebagai belum dibangun, bukan
// menggagalkan seluruh ekspor.
func (s RepoSource) KelembagaanReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.KelembagaanReport, error) {
	if s.KelembagaanSvc == nil {
		return domain.KelembagaanReport{}, ErrKelembagaanSourceUnavailable
	}
	return s.KelembagaanSvc.KelembagaanReport(ctx, asOf, actor)
}

// loanRowDariDomain memetakan kredit domain ke baris Form 06.00/NPL. Hanya field
// yang tersedia yang dipetakan; sisanya dibiarkan nol/kosong dan ditandai belum
// tersedia oleh form terkait.
func loanRowDariDomain(l domain.Loan) LoanRow {
	return LoanRow{
		Status:             string(l.Status),
		BranchCode:         l.BranchCode,
		LoanNumber:         l.LoanNumber,
		CustomerID:         l.CustomerID.String(),
		Collectibility:     string(l.Collectibility),
		DPD:                l.DPD,
		Outstanding:        l.OutstandingPrincipal,
		RequiredCKPN:       l.RequiredCKPN,
		RequiredPPAP:       l.RequiredPPAP,
		CKPNMethod:         l.CKPNMethod,
		IsRestructured:     l.IsRestructured,
		RestructuredCount:  l.RestructuredCount,
		InterestRateAnnual: l.InterestRateAnnual,
		PrincipalAmount:    l.PrincipalAmount,
		AkadDate:           l.AkadDate,
		FinalDueDate:       l.FinalDueDate,
		// Sandi inline/field sederhana Form 06.00 (VIII, XI, XXII) yang diisi bank
		// lewat SQL/seed; dibiarkan kosong bila belum ada, form menulis "-".
		OJKJenisPenggunaanCode:   l.OJKJenisPenggunaanCode,
		OJKPeriodePembayaranCode: l.OJKPeriodePembayaranCode,
		OJKKabupatenCode:         l.OJKKabupatenCode,
		// Kolom K1 Form 06.00 lain (migrasi 000107); kosong/nil ditulis "-".
		OJKKelompokKreditCode:  l.OJKKelompokKreditCode,
		OJKSumberDanaCode:      l.OJKSumberDanaCode,
		OJKKategoriUsahaCode:   l.OJKKategoriUsahaCode,
		OJKSifatKreditCode:     l.OJKSifatKreditCode,
		OJKPenjaminCode:        l.OJKPenjaminCode,
		OJKPenjaminBagianPct:   l.OJKPenjaminBagianPct,
		OJKTanggalMulaiMacet:   l.OJKTanggalMulaiMacet,
		OJKKlasifikasiAsetCode: l.OJKKlasifikasiAsetCode,
		// Kolom XXV/XXVI Form 06.00 (migrasi 000109): nilai agunan PPKA dan kelonggaran
		// tarik. Nil berarti belum diisi/belum dihitung; form menulis "-", bukan nol.
		OJKAgunanPPKAAmount:       l.OJKAgunanPPKAAmount,
		OJKKelonggaranTarikAmount: l.OJKKelonggaranTarikAmount,
		// Kolom XXIX–XXXII Form 06.00 (migrasi 000111): komponen amortisasi provisi/biaya
		// dan restrukturisasi yang diisi bank. Nil berarti belum diisi; form menulis "-".
		OJKProvisiBelumDiamortisasiAmount:        l.OJKProvisiBelumDiamortisasiAmount,
		OJKBiayaTransaksiBelumDiamortisasiAmount: l.OJKBiayaTransaksiBelumDiamortisasiAmount,
		OJKPendapatanBungaDitangguhkanAmount:     l.OJKPendapatanBungaDitangguhkanAmount,
		OJKCadanganKerugianRestrukturisasiAmount: l.OJKCadanganKerugianRestrukturisasiAmount,
		RestructuredAt:                           l.RestructuredAt,
	}
}

// customerIDString menuliskan id nasabah sebagai string; penempatan yang belum
// ditautkan ke pihak terkait (customer_id NULL) menjadi string kosong.
func customerIDString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// BMPKReport meneruskan laporan BMPK dari modul BMPK. Kontrak bank-wide ditegakkan
// modul BMPK sendiri (domain.ErrBMPKBankWide); RepoSource tidak menggandakan
// pemeriksaannya agar identitas galat tetap satu.
func (s RepoSource) BMPKReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.BMPKReport, error) {
	if s.BMPK == nil {
		return domain.BMPKReport{}, ErrBMPKSourceUnavailable
	}
	return s.BMPK.BMPKReport(ctx, asOf, actor)
}
