package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// PostingLine adalah satu sisi jurnal yang sudah teresolusi ke akun konkret.
type PostingLine struct {
	AccountNumber string          `json:"account_number"`
	Direction     EntryDirection  `json:"direction"`
	Amount        decimal.Decimal `json:"amount"`
	Description   string          `json:"description,omitempty"`
}

// PostingRequest adalah permintaan tunggal untuk memposting jurnal. Semua modul
// (tabungan, deposito, kredit, pembiayaan) memakai struktur ini, tidak lagi menulis
// SQL jurnal sendiri.
// JournalSource menandai alur bisnis yang membuat jurnal. Dipakai untuk memisahkan
// transaksi yang aman dibatalkan dari yang membawa state domain: transaction_type tidak
// bisa dipakai karena DEPOSIT/WITHDRAWAL dipakai bersama oleh alur teller dan alur deposito.
type JournalSource string

const (
	// SourceTeller adalah transaksi rekening yang diinput petugas: setoran, penarikan,
	// transfer antar rekening, dan pembatalannya.
	SourceTeller JournalSource = "TELLER"
	// SourceLoan adalah jurnal kredit: pencairan, angsuran, denda, hapus buku, recovery.
	SourceLoan JournalSource = "LOAN"
	// SourceDeposit adalah jurnal deposito berjangka: penempatan, akrual, pencairan.
	SourceDeposit JournalSource = "DEPOSIT"
	// SourceBatch adalah jurnal proses otomatis: tutup hari/bulan/tahun, PPAP, akrual.
	SourceBatch JournalSource = "BATCH"
)

type PostingRequest struct {
	TransactionType TransactionType `json:"transaction_type"`
	// Source diisi pemanggil sesuai alurnya. Kosong berarti alur belum ditandai, dan
	// jurnal seperti itu tidak boleh dibatalkan.
	Source          JournalSource `json:"source,omitempty"`
	Description     string        `json:"description"`
	ReferenceNumber string        `json:"reference_number,omitempty"` // kosong = dibangkitkan
	IdempotencyKey  string        `json:"idempotency_key,omitempty"`
	CreatedBy       string        `json:"created_by"`
	BranchCode      string        `json:"branch_code,omitempty"`
	// EntryDate adalah tanggal akuntansi entri. Bila nol, posting engine memakai
	// tanggal bisnis berjalan dari BusinessDateRepository; posting ditolak bila
	// tanggal itu tidak terbaca. Pemanggil yang tanggalnya penting (tutup buku,
	// akrual) wajib mengisinya, karena laporan periode membaca kolom ini.
	EntryDate time.Time     `json:"entry_date,omitempty"`
	Lines     []PostingLine `json:"lines"`
}

// PostingService mengeksekusi posting jurnal. Post membuka transaksinya sendiri.
// Tx adalah handle transaksi database yang bersifat opaque bagi domain; implementasi
// konkretnya ada di lapisan repository.
type PostingService interface {
	Post(ctx context.Context, req PostingRequest) (*JournalEntry, error)
	PostTx(ctx context.Context, tx any, req PostingRequest) (*JournalEntry, error)
}

// PostingRepository menyediakan operasi jurnal tingkat rendah yang dipakai posting service.
type PostingRepository interface {
	InsertJournal(ctx context.Context, tx any, entry *JournalEntry) error
	FindJournalByIdempotencyKey(ctx context.Context, key string) (*JournalEntry, error)
}

// AccountResolver menerjemahkan kode COA menjadi nomor akun GL internal yang bisa
// diposting. Produk mendefinisikan jurnal memakai kode COA; operasi jurnal memerlukan
// nomor akun konkret.
type AccountResolver interface {
	ResolveGLAccount(ctx context.Context, tx any, coaCode string) (string, error)
}

// BusinessDateProvider menentukan tanggal bisnis aktif.
type BusinessDateProvider interface {
	CurrentBusinessDate(ctx context.Context) (time.Time, error)
}

// ReferenceGenerator membangkitkan nomor referensi jurnal yang unik. Implementasi
// konkret memakai sequence database, bukan timestamp, agar tidak bertabrakan saat
// transaksi paralel. Kegagalan membaca sequence harus dikembalikan sebagai error;
// nomor referensi ganda di bank menyamarkan dua transaksi berbeda.
type ReferenceGenerator interface {
	Next(txType TransactionType, at time.Time) (string, error)
	// NextTx mengambil nomor di dalam transaksi pemanggil agar kegagalan membaca
	// sequence ikut membatalkan jurnal, bukan meninggalkan jurnal tanpa nomor.
	NextTx(ctx context.Context, tx any, txType TransactionType, at time.Time) (string, error)
	// NextLoanNumber membangkitkan nomor kredit dari sequence tersendiri, terpisah
	// dari sequence referensi transaksi. Prefix mengikuti buku produk.
	NextLoanNumber(ctx context.Context, product *BankingProduct, at time.Time) (string, error)
}
