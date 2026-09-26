package ojkreport

import (
	"context"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
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
	// KPMM mengisi baris KPMM Form 00.08. Bila nil, Form 00.08 menulis baris KPMM
	// sebagai tidak tersedia (bukan nol).
	KPMM domain.KPMMService
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
// belum lunas. Agregat jadwal diambil dengan SATU query per kredit (GROUP BY
// loan_id, lihat ListLoanScheduleAggregates), lalu dipetakan lewat nomor kredit
// yang unik; bukan satu query per kredit.
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

	var out []LoanRow
	offset := 0
	for page := 0; page < maxHalamanKredit; page++ {
		items, total, err := s.Loans.List(ctx, pageSizeKredit, offset, actor)
		if err != nil {
			return nil, err
		}
		for i := range items {
			row := loanRowDariDomain(items[i])
			if a, ok := agregat[row.LoanNumber]; ok {
				row.FirstInstallmentDate = a.FirstInstallmentDate
				row.OverdueUnpaid = a.OverdueUnpaid
			}
			out = append(out, row)
		}
		offset += len(items)
		if len(items) == 0 || offset >= total {
			break
		}
	}
	return out, nil
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
		CKPNMethod:         l.CKPNMethod,
		IsRestructured:     l.IsRestructured,
		RestructuredCount:  l.RestructuredCount,
		InterestRateAnnual: l.InterestRateAnnual,
		PrincipalAmount:    l.PrincipalAmount,
		AkadDate:           l.AkadDate,
		FinalDueDate:       l.FinalDueDate,
	}
}
