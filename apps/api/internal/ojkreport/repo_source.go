package ojkreport

import (
	"context"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// repo_source.go menghubungkan builder dengan repositori yang sudah ada tanpa
// menambah dependensi: kredit (loans), profil bank (bank_profile), konfigurasi
// (system_config), dan penempatan pada bank lain (lps_placements).
//
// RepoSource mengimplementasikan Source (lewat embedding) sekaligus kontrak data
// opsional LoanDataSource, BankProfileSource, dan PlacementDataSource. Bila builder
// tidak menerima sumber opsional, form terkait ditandai belum tersedia.
type RepoSource struct {
	Source
	Loans      domain.LoanRepository
	Profile    domain.BankProfileRepository
	Config     domain.SystemConfigRepository
	Placements domain.LPSPlacementRepository
}

// pageSizeKredit membatasi jumlah kredit per halaman pembacaan.
const pageSizeKredit = 500

// maxHalamanKredit mencegah pengulangan tanpa batas bila total dari repositori
// berubah di tengah pembacaan.
const maxHalamanKredit = 1000

// ListLoansForOJK membaca seluruh kredit bank-wide. Kebijakan bank-wide ditegakkan
// di lapisan data: aktor non-lintas cabang ditolak, bukan diberi sebagian.
func (s RepoSource) ListLoansForOJK(ctx context.Context, _ time.Time, actor domain.Actor) ([]LoanRow, error) {
	if err := pastikanLintasCabang(actor); err != nil {
		return nil, err
	}
	if s.Loans == nil {
		return nil, nil
	}
	var out []LoanRow
	offset := 0
	for page := 0; page < maxHalamanKredit; page++ {
		items, total, err := s.Loans.List(ctx, pageSizeKredit, offset, actor)
		if err != nil {
			return nil, err
		}
		for i := range items {
			out = append(out, loanRowDariDomain(items[i]))
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
	}
	cfg.Configured = strings.TrimSpace(cfg.Name) != ""
	return cfg, nil
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
		out = append(out, PlacementRow{
			BranchCode:       p.BranchCode,
			CounterpartyBank: p.CounterpartyBank,
			PlacementType:    string(p.PlacementType),
			Outstanding:      p.Outstanding,
			Collectibility:   string(p.Collectibility),
			AsOf:             p.AsOf,
		})
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
		IsRestructured:     l.IsRestructured,
		RestructuredCount:  l.RestructuredCount,
		InterestRateAnnual: l.InterestRateAnnual,
		PrincipalAmount:    l.PrincipalAmount,
		AkadDate:           l.AkadDate,
		FinalDueDate:       l.FinalDueDate,
	}
}
