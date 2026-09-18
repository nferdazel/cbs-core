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
type PostingRequest struct {
	TransactionType TransactionType `json:"transaction_type"`
	Description     string          `json:"description"`
	ReferenceNumber string          `json:"reference_number,omitempty"` // kosong = dibangkitkan
	IdempotencyKey  string          `json:"idempotency_key,omitempty"`
	CreatedBy       string          `json:"created_by"`
	BranchCode      string          `json:"branch_code,omitempty"`
	Lines           []PostingLine   `json:"lines"`
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
// transaksi paralel.
type ReferenceGenerator interface {
	Next(txType TransactionType, at time.Time) string
}
