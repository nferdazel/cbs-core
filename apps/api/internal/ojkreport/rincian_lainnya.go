package ojkreport

import (
	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// rincian_lainnya.go memuat fondasi bersama Form 09.01 dan Form 14.01 — dua form
// KONDISIONAL yang merinci pos "Lainnya" pada Form 09.00/14.00 per akun COA.
//
// Keduanya hanya terbit bila pos Lainnya melebihi 25% dari jumlah kelompoknya
// (melebihi25Persen, form09_01.go) dan, bila terlampaui, barisnya adalah satu akun
// COA yang dipetakan ke pos Lainnya pada pemetaan form induk.

// rincianLainnya adalah satu baris rincian per akun COA untuk Form 09.01/14.01.
// Tersedia=false berarti akun dipetakan ke pos Lainnya tetapi tidak muncul pada baris
// neraca periodEnd; Uraian dan Jumlahnya ditulis "-" beserta alasan, bukan nol.
type rincianLainnya struct {
	COACode  string
	Name     string
	Amount   decimal.Decimal
	Tersedia bool
}

// collectRincianLainnya mengumpulkan saldo per akun COA yang dipetakan ke satu sandi
// pos "Lainnya" (1299990000 atau 2299990000). Urutan baris mengikuti urutan entri
// pemetaan agar stabil. Nama akun diambil dari baris neraca periodEnd; kode yang tidak
// muncul di situ ditandai Tersedia=false sehingga perakit menulis "-" dengan alasan.
func collectRincianLainnya(entries []MappingEntry, sandi string, rows []domain.ReportRow) []rincianLainnya {
	type saldo struct {
		name   string
		amount decimal.Decimal
	}
	byCode := make(map[string]saldo, len(rows))
	for _, row := range rows {
		s := byCode[row.AccountCode]
		if s.name == "" {
			s.name = row.AccountName
		}
		s.amount = s.amount.Add(row.Amount)
		byCode[row.AccountCode] = s
	}

	var out []rincianLainnya
	for _, e := range entries {
		if e.Sandi != sandi {
			continue
		}
		s, ok := byCode[e.COACode]
		if !ok {
			out = append(out, rincianLainnya{COACode: e.COACode})
			continue
		}
		amount := s.amount
		if e.Sign < 0 {
			amount = amount.Neg()
		}
		out = append(out, rincianLainnya{COACode: e.COACode, Name: s.name, Amount: amount, Tersedia: true})
	}
	return out
}
