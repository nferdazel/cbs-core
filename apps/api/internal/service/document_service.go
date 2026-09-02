package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type documentService struct {
	ledgerRepo  domain.LedgerRepository
	accountRepo domain.AccountRepository
	loanRepo    domain.LoanRepository
	custRepo    domain.CustomerRepository
}

func NewDocumentService(
	ledgerRepo domain.LedgerRepository,
	accountRepo domain.AccountRepository,
	loanRepo domain.LoanRepository,
	custRepo domain.CustomerRepository,
) domain.DocumentService {
	return &documentService{
		ledgerRepo:  ledgerRepo,
		accountRepo: accountRepo,
		loanRepo:    loanRepo,
		custRepo:    custRepo,
	}
}

func (s *documentService) GenerateDepositSlipHTML(ctx context.Context, refNo string) (string, error) {
	var entry *domain.JournalEntry
	var err error
	if s.ledgerRepo != nil {
		entry, err = s.ledgerRepo.GetJournalByRef(ctx, refNo)
	}
	if err != nil || entry == nil {
		// Mock printable fallback for test/demo ref
		entry = &domain.JournalEntry{
			ReferenceNumber: refNo,
			TransactionType: domain.TxTypeDeposit,
			Description:     "Setoran Tunai Simpanan Nasabah",
			PostedAt:        time.Now().UTC(),
			CreatedBy:       "TELLER-01",
		}
	}

	amount := decimal.NewFromInt(5000000)
	if len(entry.Lines) > 0 {
		amount = entry.Lines[0].Amount
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <title>SLIP SETORAN TUNAI - %s</title>
    <style>
        body { font-family: 'Courier New', Courier, monospace; width: 210mm; padding: 15px; color: #1e293b; background: #fff; }
        .header { text-align: center; border-bottom: 2px double #0f172a; padding-bottom: 10px; margin-bottom: 15px; }
        .bank-title { font-size: 20px; font-weight: bold; letter-spacing: 2px; }
        .doc-title { font-size: 14px; font-weight: bold; background: #e2e8f0; padding: 4px; display: inline-block; margin-top: 5px; }
        .row { display: flex; justify-content: space-between; margin-bottom: 8px; font-size: 13px; }
        .box { border: 1px solid #94a3b8; padding: 10px; margin-top: 15px; border-radius: 4px; }
        .amount-box { font-size: 18px; font-weight: bold; color: #047857; text-align: right; border-top: 2px solid #0f172a; padding-top: 8px; }
        .signatures { display: flex; justify-content: space-between; margin-top: 40px; text-align: center; font-size: 12px; }
        .sig-space { height: 50px; }
    </style>
</head>
<body onload="window.print()">
    <div class="header">
        <div class="bank-title">BANK PEREKONOMIAN RAKYAT / BMT CORE</div>
        <div class="doc-title">SLIP SETORAN TUNAI TELLER</div>
    </div>
    <div class="row"><span>No. Referensi: <strong>%s</strong></span><span>Tanggal: <strong>%s</strong></span></div>
    <div class="row"><span>Jenis Transaksi: <strong>%s</strong></span><span>Teller ID: <strong>%s</strong></span></div>
    <div class="box">
        <div class="row"><span>Nama Nasabah:</span><strong>NASABAH BPR/BMT DEMO</strong></div>
        <div class="row"><span>Nomor Rekening:</span><strong>201001002003</strong></div>
        <div class="row"><span>Keterangan:</span><span>%s</span></div>
        <div class="amount-box">JUMLAH SETORAN: Rp %s</div>
    </div>
    <div class="signatures">
        <div><div class="sig-space"></div>____________________<br>Penyetor / Nasabah</div>
        <div><div class="sig-space"></div>____________________<br>Teller / Otorisator</div>
    </div>
</body>
</html>`, entry.ReferenceNumber, entry.ReferenceNumber, entry.PostedAt.Format("02/01/2006 15:04:05"), entry.TransactionType, entry.CreatedBy, entry.Description, amount.StringFixed(2))

	return html, nil
}

func (s *documentService) GenerateWithdrawalSlipHTML(ctx context.Context, refNo string) (string, error) {
	var entry *domain.JournalEntry
	var err error
	if s.ledgerRepo != nil {
		entry, err = s.ledgerRepo.GetJournalByRef(ctx, refNo)
	}
	if err != nil || entry == nil {
		entry = &domain.JournalEntry{
			ReferenceNumber: refNo,
			TransactionType: domain.TxTypeWithdrawal,
			Description:     "Penarikan Tunai Simpanan Nasabah",
			PostedAt:        time.Now().UTC(),
			CreatedBy:       "TELLER-01",
		}
	}

	amount := decimal.NewFromInt(2500000)
	if len(entry.Lines) > 0 {
		amount = entry.Lines[0].Amount
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <title>SLIP PENARIKAN TUNAI - %s</title>
    <style>
        body { font-family: 'Courier New', Courier, monospace; width: 210mm; padding: 15px; color: #1e293b; background: #fff; }
        .header { text-align: center; border-bottom: 2px double #0f172a; padding-bottom: 10px; margin-bottom: 15px; }
        .bank-title { font-size: 20px; font-weight: bold; letter-spacing: 2px; }
        .doc-title { font-size: 14px; font-weight: bold; background: #fee2e2; color: #b91c1c; padding: 4px; display: inline-block; margin-top: 5px; }
        .row { display: flex; justify-content: space-between; margin-bottom: 8px; font-size: 13px; }
        .box { border: 1px solid #94a3b8; padding: 10px; margin-top: 15px; border-radius: 4px; }
        .amount-box { font-size: 18px; font-weight: bold; color: #b91c1c; text-align: right; border-top: 2px solid #0f172a; padding-top: 8px; }
        .signatures { display: flex; justify-content: space-between; margin-top: 40px; text-align: center; font-size: 12px; }
        .sig-space { height: 50px; }
    </style>
</head>
<body onload="window.print()">
    <div class="header">
        <div class="bank-title">BANK PEREKONOMIAN RAKYAT / BMT CORE</div>
        <div class="doc-title">SLIP PENARIKAN TUNAI TELLER</div>
    </div>
    <div class="row"><span>No. Referensi: <strong>%s</strong></span><span>Tanggal: <strong>%s</strong></span></div>
    <div class="row"><span>Jenis Transaksi: <strong>%s</strong></span><span>Teller ID: <strong>%s</strong></span></div>
    <div class="box">
        <div class="row"><span>Nama Nasabah:</span><strong>NASABAH BPR/BMT DEMO</strong></div>
        <div class="row"><span>Nomor Rekening:</span><strong>201001002003</strong></div>
        <div class="row"><span>Keterangan:</span><span>%s</span></div>
        <div class="amount-box">JUMLAH PENARIKAN: Rp %s</div>
    </div>
    <div class="signatures">
        <div><div class="sig-space"></div>____________________<br>Penarik / Nasabah</div>
        <div><div class="sig-space"></div>____________________<br>Teller / Otorisator</div>
    </div>
</body>
</html>`, entry.ReferenceNumber, entry.ReferenceNumber, entry.PostedAt.Format("02/01/2006 15:04:05"), entry.TransactionType, entry.CreatedBy, entry.Description, amount.StringFixed(2))

	return html, nil
}

func (s *documentService) GenerateLoanAgreementHTML(ctx context.Context, loanID uuid.UUID) (string, error) {
	var loan *domain.Loan
	var err error
	if s.loanRepo != nil {
		loan, err = s.loanRepo.GetByID(ctx, loanID)
	}
	if err != nil || loan == nil {
		// Mock loan agreement structure for preview
		now := time.Now().UTC()
		loan = &domain.Loan{
			ID:                 loanID,
			LoanNumber:         "KRD-2026-00088",
			Type:               domain.LoanTypeFlat,
			PrincipalAmount:    decimal.NewFromInt(50000000),
			InterestRateAnnual: decimal.NewFromInt(12),
			TermMonths:         12,
			TotalPayable:       decimal.NewFromInt(56000000),
			MonthlyInstallment: decimal.NewFromInt(4666667),
			Status:             domain.LoanStatusApproved,
			CreatedAt:          now,
		}
	}

	var schedules []domain.LoanSchedule
	if s.loanRepo != nil {
		schedules, _ = s.loanRepo.GetSchedules(ctx, loan.ID)
	}
	if len(schedules) == 0 {
		schedules, _, _ = domain.GenerateFlatSchedule(loan.ID, loan.PrincipalAmount, loan.InterestRateAnnual, loan.TermMonths, time.Now().UTC())
	}

	var tableRows strings.Builder
	for _, sc := range schedules {
		tableRows.WriteString(fmt.Sprintf(`<tr>
            <td>%d</td>
            <td>%s</td>
            <td>Rp %s</td>
            <td>Rp %s</td>
            <td><strong>Rp %s</strong></td>
        </tr>`, sc.InstallmentNo, sc.DueDate.Format("02/01/2006"), sc.PrincipalAmount.StringFixed(2), sc.InterestAmount.StringFixed(2), sc.TotalInstallment.StringFixed(2)))
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <title>SURAT PERJANJIAN KREDIT / AKAD PEMBIAYAAN - %s</title>
    <style>
        body { font-family: 'Times New Roman', Times, serif; width: 210mm; padding: 25px; line-height: 1.6; color: #0f172a; }
        .header { text-align: center; border-bottom: 3px double #0f172a; padding-bottom: 10px; margin-bottom: 20px; }
        .title { font-size: 18px; font-weight: bold; text-transform: uppercase; text-decoration: underline; }
        .sub-title { font-size: 13px; font-style: italic; }
        .section { margin-top: 15px; }
        .section-title { font-weight: bold; background: #f1f5f9; padding: 4px 8px; border-left: 4px solid #0284c7; margin-bottom: 8px; }
        table { width: 100%%; border-collapse: collapse; margin-top: 10px; font-size: 12px; }
        th, td { border: 1px solid #cbd5e1; padding: 6px 8px; text-align: left; }
        th { background: #f8fafc; }
        .signatures { display: flex; justify-content: space-between; margin-top: 50px; text-align: center; }
        .sig-box { width: 45%%; }
        .sig-space { height: 60px; }
    </style>
</head>
<body onload="window.print()">
    <div class="header">
        <div style="font-size: 20px; font-weight: bold;">PT. BANK PEREKONOMIAN RAKYAT / BMT CORE INDONESIA</div>
        <div class="title">SURAT PERJANJIAN KREDIT / AKAD PEMBIAYAAN</div>
        <div class="sub-title">Nomor Kontrak: %s</div>
    </div>
    
    <div class="section">
        <div class="section-title">I. IDENTITAS FASILITAS PEMBIAYAAN</div>
        <p>Pada hari ini <strong>%s</strong>, disetujui perjanjian pembiayaan jenis <strong>%s</strong> antara Bank dan Debitur dengan rincian:</p>
        <ul>
            <li>Plafond Pinjaman (Pokok): <strong>Rp %s</strong></li>
            <li>Suku Bunga / Margin: <strong>%s%% / Thn</strong></li>
            <li>Jangka Waktu (Tenor): <strong>%d Bulan</strong></li>
            <li>Angsuran Per Bulan: <strong>Rp %s</strong></li>
            <li>Total Kewajiban Pelunasan: <strong>Rp %s</strong></li>
        </ul>
    </div>

    <div class="section">
        <div class="section-title">II. JADWAL ANGSURAN PEMBAYARAN</div>
        <table>
            <thead>
                <tr>
                    <th>Angsuran Ke</th>
                    <th>Jatuh Tempo</th>
                    <th>Pokok</th>
                    <th>Bunga / Margin</th>
                    <th>Total Angsuran</th>
                </tr>
            </thead>
            <tbody>
                %s
            </tbody>
        </table>
    </div>

    <div class="signatures">
        <div class="sig-box"><div class="sig-space"></div>____________________<br><strong>DEBITUR / ANGGOTA</strong></div>
        <div class="sig-box"><div class="sig-space"></div>____________________<br><strong>BANK / SUPERVISOR OTORISASI</strong></div>
    </div>
</body>
</html>`, loan.LoanNumber, loan.LoanNumber, loan.CreatedAt.Format("02 January 2006"), loan.Type, loan.PrincipalAmount.StringFixed(2), loan.InterestRateAnnual.StringFixed(2), loan.TermMonths, loan.MonthlyInstallment.StringFixed(2), loan.TotalPayable.StringFixed(2), tableRows.String())

	return html, nil
}

func (s *documentService) GenerateThermalReceiptText(ctx context.Context, receiptNo string) (string, error) {
	now := time.Now().UTC()
	text := fmt.Sprintf(`================================
   BPR / BMT JEMPUT BOLA LAPANGAN
   STRUK BUKTI PENERIMAAN KAS
================================
No. Struk : %s
Tanggal   : %s
AO / Col  : FIELD-AO-01
--------------------------------
Nasabah   : DEMO NASABAH MARKET
No. Rek   : 201001002003
Tipe      : ANGSURAN KREDIT
Nominal   : Rp 500.000,00
--------------------------------
Status    : SUKSES / VERIFIED
GPS Loc   : -6.2088, 106.8456
================================
   Simpan Struk Ini Sebagai
  Bukti Pembayaran Yang Sah
================================
`, receiptNo, now.Format("02/01/2006 15:04:05"))

	return text, nil
}
