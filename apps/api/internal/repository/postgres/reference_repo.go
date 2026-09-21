package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// ReferenceGenerator membangkitkan nomor referensi jurnal dari sequence database,
// sehingga aman terhadap posting paralel (tidak memakai timestamp/nanosecond).
type ReferenceGenerator struct {
	db *sql.DB
}

func NewReferenceGenerator(db *sql.DB) *ReferenceGenerator {
	return &ReferenceGenerator{db: db}
}

var refPrefix = map[domain.TransactionType]string{
	domain.TxTypeDeposit:          "DEP",
	domain.TxTypeWithdrawal:       "WDR",
	domain.TxTypeTransferInternal: "TRF",
	domain.TxTypeFeeCharge:        "FEE",
	domain.TxTypeInterestAccrual:  "ACR",
	domain.TxTypeReversal:         "REV",
	domain.TxTypeAdjustment:       "ADJ",
	// Penempatan deposito punya prefix sendiri agar tidak tertukar dengan setoran
	// tunai teller (DEP) di daftar referensi maupun rekonsiliasi.
	domain.TxTypeDepositPlacement: "DPL",
}

// Next mengembalikan nomor referensi berformat PREFIX-YYYYMMDD-NNNNNN di luar
// transaksi pemanggil. Error sequence diteruskan, tidak ada nomor cadangan.
func (g *ReferenceGenerator) Next(txType domain.TransactionType, at time.Time) (string, error) {
	return g.NextTx(context.Background(), nil, txType, at)
}

// NextTx mengambil nomor referensi memakai sequence database. Bila tx disediakan,
// nextval dijalankan pada koneksi transaksi yang sama dengan posting sehingga
// kegagalan membaca sequence membatalkan jurnal, bukan menulis tanpa nomor.
func (g *ReferenceGenerator) NextTx(ctx context.Context, tx any, txType domain.TransactionType, at time.Time) (string, error) {
	prefix, ok := refPrefix[txType]
	if !ok {
		prefix = "JRN"
	}

	var seq int64
	var err error
	if sqlTx, ok := tx.(*sql.Tx); ok {
		err = sqlTx.QueryRowContext(ctx, `SELECT nextval('journal_reference_seq')`).Scan(&seq)
	} else {
		err = g.db.QueryRowContext(ctx, `SELECT nextval('journal_reference_seq')`).Scan(&seq)
	}
	if err != nil {
		return "", fmt.Errorf("membaca sequence referensi jurnal: %w", err)
	}
	return fmt.Sprintf("%s-%s-%06d", prefix, at.Format("20060102"), seq), nil
}

// NextLoanNumber membangkitkan nomor kredit berformat PREFIX-YYYYMMDD-NNNNNN.
// Prefix mengikuti buku produk (KRD konvensional, PMB syariah). Nomor diambil dari
// loan_number_seq yang TERPISAH dari journal_reference_seq, sehingga pencairan kredit
// tidak lagi memakan urutan referensi transaksi. Kegagalan sequence dikembalikan
// sebagai error, bukan nomor cadangan: nomor kredit ganda membuat dua kredit berbeda
// tampak sebagai satu.
//
// ctx diteruskan ke kueri sequence, bukan context.Background(): pembatalan atau timeout
// permintaan HTTP harus menyebar ke pembacaan sequence. Tanpa itu, permintaan yang sudah
// dibatalkan klien tetap menunggu koneksi database dan dapat membakar nomor.
func (g *ReferenceGenerator) NextLoanNumber(ctx context.Context, product *domain.BankingProduct, at time.Time) (string, error) {
	var seq int64
	if err := g.db.QueryRowContext(ctx, `SELECT nextval('loan_number_seq')`).Scan(&seq); err != nil {
		return "", fmt.Errorf("membaca sequence nomor kredit: %w", err)
	}
	return fmt.Sprintf("%s-%s-%06d", domain.LoanNumberPrefix(product), at.Format("20060102"), seq), nil
}

// NextCIF mengembalikan nomor CIF berurutan. CIF dipakai sebagai identitas nasabah
// lintas cabang dan tidak boleh berulang, sehingga diambil dari sequence database.
// Kegagalan sequence dikembalikan sebagai error, bukan nomor pengganti.
func (g *ReferenceGenerator) NextCIF() (string, error) {
	var seq int64
	if err := g.db.QueryRowContext(context.Background(), `SELECT nextval('cif_number_seq')`).Scan(&seq); err != nil {
		return "", fmt.Errorf("membaca sequence nomor CIF: %w", err)
	}
	return fmt.Sprintf("CIF%09d", seq), nil
}
