package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"

	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type documentService struct {
	ledgerRepo      domain.LedgerRepository
	accountRepo     domain.AccountRepository
	loanRepo        domain.LoanRepository
	custRepo        domain.CustomerRepository
	bankProfileRepo domain.BankProfileRepository
	cipher          *crypto.Cipher
}

// bankProfileMissingLabel ditampilkan pada dokumen bila profil bank belum diisi
// lewat konfigurasi. Identitas bank tidak boleh dikarang di kode dokumen.
const bankProfileMissingLabel = "PROFIL BANK BELUM DIKONFIGURASI"

// NewDocumentService membangun generator dokumen. Cipher wajib: tanpa kunci
// enkripsi nama nasabah tidak dapat didekripsi, dan dokumen tidak boleh mencetak
// placeholder sebagai gantinya. Kegagalan itu lebih baik terlihat saat penyusunan
// dependensi daripada saat nasabah menunggu slipnya dicetak.
func NewDocumentService(
	ledgerRepo domain.LedgerRepository,
	accountRepo domain.AccountRepository,
	loanRepo domain.LoanRepository,
	custRepo domain.CustomerRepository,
	bankProfileRepo domain.BankProfileRepository,
	cipher *crypto.Cipher,
) domain.DocumentService {
	return &documentService{
		ledgerRepo:      ledgerRepo,
		accountRepo:     accountRepo,
		loanRepo:        loanRepo,
		custRepo:        custRepo,
		bankProfileRepo: bankProfileRepo,
		cipher:          cipher,
	}
}

// decryptCustomerName mengambil nama nasabah dari repository (ciphertext) lalu
// mendekripsinya dengan kunci enkripsi. Nilai kosong dikembalikan tanpa error
// hanya bila jaringan data nasabah atau cipher belum tersedia; dokumen transaksi
// memeriksa prasyarat ini lebih dulu agar tidak mencetak nama kosong.
func (s *documentService) decryptCustomerName(ctx context.Context, customerID uuid.UUID) (string, error) {
	if s.custRepo == nil || s.cipher == nil {
		return "", nil
	}
	record, err := s.custRepo.GetByID(ctx, customerID)
	if err != nil {
		return "", fmt.Errorf("mengambil data nasabah: %w", err)
	}
	if record == nil {
		return "", nil
	}
	name, err := s.cipher.Decrypt(record.FullNameEnc)
	if err != nil {
		return "", fmt.Errorf("mendekripsi nama nasabah: %w", err)
	}
	return name, nil
}

// bankIdentity merangkum identitas bank untuk dokumen cetak. missing true berarti
// profil belum diisi; Name berisi penanda eksplisit, bukan nama bank karangan.
type bankIdentity struct {
	Name    string
	Address string
	City    string
	Phone   string
	NPWP    string
	missing bool
}

// detailLine menyusun satu baris alamat/kontak untuk header dokumen. Bila profil
// belum dikonfigurasi, baris menjelaskan hal itu secara eksplisit.
func (b bankIdentity) detailLine() string {
	if b.missing {
		return "Identitas bank belum diisi pada konfigurasi bank_profile"
	}
	parts := make([]string, 0, 3)
	if v := strings.TrimSpace(b.Address); v != "" {
		parts = append(parts, v)
	}
	city := strings.TrimSpace(b.City)
	if phone := strings.TrimSpace(b.Phone); phone != "" {
		if city != "" {
			city += " - "
		}
		city += "Telp: " + phone
	}
	if city != "" {
		parts = append(parts, city)
	}
	if v := strings.TrimSpace(b.NPWP); v != "" {
		parts = append(parts, "NPWP: "+v)
	}
	return strings.Join(parts, " · ")
}

func (s *documentService) bankIdentity(ctx context.Context) (bankIdentity, error) {
	if s.bankProfileRepo == nil {
		return bankIdentity{Name: bankProfileMissingLabel, missing: true}, nil
	}
	profile, err := s.bankProfileRepo.Get(ctx)
	if err != nil {
		return bankIdentity{}, fmt.Errorf("memuat profil bank: %w", err)
	}
	if !profile.Configured() {
		return bankIdentity{Name: bankProfileMissingLabel, missing: true}, nil
	}
	return bankIdentity{
		Name:    strings.TrimSpace(profile.Name),
		Address: profile.Address,
		City:    profile.City,
		Phone:   profile.Phone,
		NPWP:    profile.NPWP,
	}, nil
}

// loadJournalEntry menelusuri jurnal dari nomor referensi. Tidak ada fallback
// mock: nomor yang tidak ditemukan adalah error, bukan alasan mencetak data palsu.
// Jurnal di luar cabang atau buku aktor ditolak, karena nomor referensi dapat
// diiterasi.
func (s *documentService) loadJournalEntry(ctx context.Context, refNo string, actor domain.Actor) (*domain.JournalEntry, error) {
	if s.ledgerRepo == nil {
		return nil, errors.New("repositori jurnal tidak tersedia")
	}
	entry, err := s.ledgerRepo.GetJournalByRef(ctx, refNo)
	if err != nil {
		return nil, fmt.Errorf("mencari jurnal %s: %w", refNo, err)
	}
	if entry == nil {
		return nil, fmt.Errorf("jurnal %s tidak ditemukan", refNo)
	}
	if !actor.CanAccessBranch(entry.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	// Buku jurnal diturunkan dari akun barisnya (entry.Book/entry.BookMixed), sehingga
	// slip setoran dan dokumen lain tidak membocorkan jurnal buku lain. Jurnal tanpa
	// buku tetap boleh dicetak, jurnal lintas buku ditolak (CanAccessJournal).
	if !actor.CanAccessJournal(entry.Book, entry.BookMixed) {
		return nil, domain.ErrCrossBookAccess
	}
	return entry, nil
}

// errJournalCustomerLine menandai jurnal yang tidak memiliki kaki rekening nasabah,
// sehingga nama dan nomor rekening tidak dapat ditentukan.
var errJournalCustomerLine = errors.New("tidak memiliki kaki rekening nasabah")

// resolveTransactionData menelusuri jurnal -> baris rekening nasabah -> akun ->
// customer_id -> nama terenkripsi. Nama didekripsi lewat cipher; nilai terenkripsi
// tidak pernah ditulis ke dokumen. Nomor rekening dan nominal diambil dari baris
// rekening yang sama dengan jurnal, bukan dari literal.
func (s *documentService) resolveTransactionData(
	ctx context.Context,
	entry *domain.JournalEntry,
) (customerName, accountNumber string, amount decimal.Decimal, err error) {
	if s.accountRepo == nil {
		return "", "", decimal.Zero, errors.New("repositori rekening tidak tersedia")
	}
	if s.custRepo == nil || s.cipher == nil {
		return "", "", decimal.Zero, errors.New("kunci enkripsi data nasabah belum dikonfigurasi")
	}

	for _, line := range entry.Lines {
		acc, accErr := s.accountRepo.GetByID(ctx, line.AccountID)
		if accErr != nil {
			if errors.Is(accErr, domain.ErrAccountNotFound) {
				continue
			}
			return "", "", decimal.Zero, fmt.Errorf("mengambil rekening baris jurnal: %w", accErr)
		}
		if acc == nil || acc.CustomerID == nil {
			continue
		}
		name, decErr := s.decryptCustomerName(ctx, *acc.CustomerID)
		if decErr != nil {
			return "", "", decimal.Zero, decErr
		}
		if strings.TrimSpace(name) == "" {
			return "", "", decimal.Zero, fmt.Errorf("nama nasabah rekening %s tidak dapat didekripsi", acc.AccountNumber)
		}
		return name, acc.AccountNumber, line.Amount, nil
	}

	return "", "", decimal.Zero, fmt.Errorf("jurnal %s %w", entry.ReferenceNumber, errJournalCustomerLine)
}

func (s *documentService) GenerateDepositSlipHTML(ctx context.Context, refNo string, actor domain.Actor) (string, error) {
	entry, err := s.loadJournalEntry(ctx, refNo, actor)
	if err != nil {
		return "", err
	}
	customerName, accountNumber, amount, err := s.resolveTransactionData(ctx, entry)
	if err != nil {
		return "", err
	}
	bank, err := s.bankIdentity(ctx)
	if err != nil {
		return "", err
	}

	ref := html.EscapeString(entry.ReferenceNumber)
	customer := html.EscapeString(customerName)
	account := html.EscapeString(accountNumber)
	description := html.EscapeString(entry.Description)
	teller := html.EscapeString(entry.CreatedBy)
	bankName := html.EscapeString(bank.Name)
	bankDetail := html.EscapeString(bank.detailLine())
	txType := html.EscapeString(string(entry.TransactionType))

	htmlDoc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <title>SLIP SETORAN TUNAI - %s</title>
    <style>
        body { font-family: 'Courier New', Courier, monospace; width: 210mm; padding: 15px; color: #1e293b; background: #fff; }
        .header { text-align: center; border-bottom: 2px double #0f172a; padding-bottom: 10px; margin-bottom: 15px; }
        .bank-title { font-size: 20px; font-weight: bold; letter-spacing: 2px; }
        .bank-detail { font-size: 11px; color: #475569; margin-top: 2px; }
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
        <div class="bank-title">%s</div>
        <div class="bank-detail">%s</div>
        <div class="doc-title">SLIP SETORAN TUNAI TELLER</div>
    </div>
    <div class="row"><span>No. Referensi: <strong>%s</strong></span><span>Tanggal: <strong>%s</strong></span></div>
    <div class="row"><span>Jenis Transaksi: <strong>%s</strong></span><span>Teller ID: <strong>%s</strong></span></div>
    <div class="box">
        <div class="row"><span>Nama Nasabah:</span><strong>%s</strong></div>
        <div class="row"><span>Nomor Rekening:</span><strong>%s</strong></div>
        <div class="row"><span>Keterangan:</span><span>%s</span></div>
        <div class="amount-box">JUMLAH SETORAN: Rp %s</div>
    </div>
    <div class="signatures">
        <div><div class="sig-space"></div>____________________<br>Penyetor / Nasabah</div>
        <div><div class="sig-space"></div>____________________<br>Teller / Otorisator</div>
    </div>
</body>
</html>`, ref, bankName, bankDetail, ref, entry.PostedAt.Format("02/01/2006 15:04:05"), txType, teller, customer, account, description, amount.StringFixed(2))

	return htmlDoc, nil
}

func (s *documentService) GenerateWithdrawalSlipHTML(ctx context.Context, refNo string, actor domain.Actor) (string, error) {
	entry, err := s.loadJournalEntry(ctx, refNo, actor)
	if err != nil {
		return "", err
	}
	customerName, accountNumber, amount, err := s.resolveTransactionData(ctx, entry)
	if err != nil {
		return "", err
	}
	bank, err := s.bankIdentity(ctx)
	if err != nil {
		return "", err
	}

	ref := html.EscapeString(entry.ReferenceNumber)
	customer := html.EscapeString(customerName)
	account := html.EscapeString(accountNumber)
	description := html.EscapeString(entry.Description)
	teller := html.EscapeString(entry.CreatedBy)
	bankName := html.EscapeString(bank.Name)
	bankDetail := html.EscapeString(bank.detailLine())
	txType := html.EscapeString(string(entry.TransactionType))

	htmlDoc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <title>SLIP PENARIKAN TUNAI - %s</title>
    <style>
        body { font-family: 'Courier New', Courier, monospace; width: 210mm; padding: 15px; color: #1e293b; background: #fff; }
        .header { text-align: center; border-bottom: 2px double #0f172a; padding-bottom: 10px; margin-bottom: 15px; }
        .bank-title { font-size: 20px; font-weight: bold; letter-spacing: 2px; }
        .bank-detail { font-size: 11px; color: #475569; margin-top: 2px; }
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
        <div class="bank-title">%s</div>
        <div class="bank-detail">%s</div>
        <div class="doc-title">SLIP PENARIKAN TUNAI TELLER</div>
    </div>
    <div class="row"><span>No. Referensi: <strong>%s</strong></span><span>Tanggal: <strong>%s</strong></span></div>
    <div class="row"><span>Jenis Transaksi: <strong>%s</strong></span><span>Teller ID: <strong>%s</strong></span></div>
    <div class="box">
        <div class="row"><span>Nama Nasabah:</span><strong>%s</strong></div>
        <div class="row"><span>Nomor Rekening:</span><strong>%s</strong></div>
        <div class="row"><span>Keterangan:</span><span>%s</span></div>
        <div class="amount-box">JUMLAH PENARIKAN: Rp %s</div>
    </div>
    <div class="signatures">
        <div><div class="sig-space"></div>____________________<br>Penarik / Nasabah</div>
        <div><div class="sig-space"></div>____________________<br>Teller / Otorisator</div>
    </div>
</body>
</html>`, ref, bankName, bankDetail, ref, entry.PostedAt.Format("02/01/2006 15:04:05"), txType, teller, customer, account, description, amount.StringFixed(2))

	return htmlDoc, nil
}

func (s *documentService) GenerateLoanAgreementHTML(ctx context.Context, loanID uuid.UUID, actor domain.Actor) (string, error) {
	if s.loanRepo == nil {
		return "", errors.New("repositori kredit tidak tersedia")
	}
	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return "", err
	}
	if loan == nil {
		return "", errors.New("kredit tidak ditemukan")
	}
	if !actor.CanAccessBranch(loan.BranchCode) {
		return "", domain.ErrCrossBranchAccess
	}

	customerName, err := s.decryptCustomerName(ctx, loan.CustomerID)
	if err != nil {
		return "", err
	}
	if customerName == "" {
		customerName = "-"
	}

	bank, err := s.bankIdentity(ctx)
	if err != nil {
		return "", err
	}

	schedules, err := s.loanRepo.GetSchedules(ctx, loan.ID)
	if err != nil {
		return "", err
	}

	var tableRows strings.Builder
	for _, sc := range schedules {
		fmt.Fprintf(&tableRows, `<tr>
            <td>%d</td>
            <td>%s</td>
            <td>Rp %s</td>
            <td>Rp %s</td>
            <td><strong>Rp %s</strong></td>
        </tr>`, sc.InstallmentNo, sc.DueDate.Format("02/01/2006"), sc.PrincipalAmount.StringFixed(2), sc.ProfitAmount.StringFixed(2), sc.TotalInstallment.StringFixed(2))
	}

	bankName := html.EscapeString(bank.Name)
	bankDetail := html.EscapeString(bank.detailLine())
	loanNumber := html.EscapeString(loan.LoanNumber)
	customer := html.EscapeString(customerName)
	scheme := html.EscapeString(loan.ProfitSchemeLabel())

	htmlDoc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <title>SURAT PERJANJIAN KREDIT / AKAD PEMBIAYAAN - %s</title>
    <style>
        body { font-family: 'Times New Roman', Times, serif; width: 210mm; padding: 25px; line-height: 1.6; color: #0f172a; }
        .header { text-align: center; border-bottom: 3px double #0f172a; padding-bottom: 10px; margin-bottom: 20px; }
        .bank-name { font-size: 20px; font-weight: bold; }
        .bank-detail { font-size: 12px; color: #475569; }
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
        <div class="bank-name">%s</div>
        <div class="bank-detail">%s</div>
        <div class="title">SURAT PERJANJIAN KREDIT / AKAD PEMBIAYAAN</div>
        <div class="sub-title">Nomor Kontrak: %s</div>
    </div>
    
    <div class="section">
        <div class="section-title">I. IDENTITAS FASILITAS PEMBIAYAAN</div>
        <p>Nama Debitur: <strong>%s</strong></p>
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
</html>`, loanNumber, bankName, bankDetail, loanNumber, customer, loan.CreatedAt.Format("02 January 2006"), scheme, loan.PrincipalAmount.StringFixed(2), loan.InterestRateAnnual.StringFixed(2), loan.TermMonths, loan.MonthlyInstallment.StringFixed(2), loan.TotalPayable.StringFixed(2), tableRows.String())

	return htmlDoc, nil
}

// GenerateThermalReceiptText membuat struk thermal dari jurnal yang ditunjuk
// receiptNo. Data nasabah (nama, rekening, nominal) diambil dari jurnal, bukan
// literal.
//
// KETERBATASAN yang perlu diketahui: nomor struk kolektor lapangan
// (MBL-YYYYMMDD-NNNNN) dibuat di collection_service dan TIDAK disimpan sebagai
// referensi jurnal. Akibatnya struk hanya dapat dicetak untuk nomor yang memang
// merujuk ke jurnal; nomor kolektor akan ditolak dengan error yang jelas. Agar
// struk kolektor bisa dicetak, collection_service perlu menyimpan pemetaan
// nomor struk -> referensi jurnal. Tanpa pemetaan itu, mencetak struk berisi data
// karangan lebih berbahaya daripada gagal dengan jujur.
func (s *documentService) GenerateThermalReceiptText(ctx context.Context, receiptNo string, actor domain.Actor) (string, error) {
	entry, err := s.loadJournalEntry(ctx, receiptNo, actor)
	if err != nil {
		return "", fmt.Errorf("struk thermal %s: %w", receiptNo, err)
	}
	customerName, accountNumber, amount, err := s.resolveTransactionData(ctx, entry)
	if err != nil {
		return "", err
	}
	bank, err := s.bankIdentity(ctx)
	if err != nil {
		return "", err
	}

	text := fmt.Sprintf(`================================
   %s
   STRUK BUKTI PENERIMAAN KAS
================================
No. Struk : %s
Tanggal   : %s
AO / Col  : %s
--------------------------------
Nasabah   : %s
No. Rek   : %s
Tipe      : %s
Nominal   : Rp %s
--------------------------------
Status    : %s
================================
   Simpan Struk Ini Sebagai
  Bukti Pembayaran Yang Sah
================================
`, bank.Name, receiptNo, entry.PostedAt.Format("02/01/2006 15:04:05"), entry.CreatedBy,
		customerName, accountNumber, entry.TransactionType, amount.StringFixed(2), entry.Status)

	return text, nil
}
