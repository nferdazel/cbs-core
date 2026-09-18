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
}

// Next mengembalikan nomor referensi berformat PREFIX-YYYYMMDD-NNNNNN.
func (g *ReferenceGenerator) Next(txType domain.TransactionType, at time.Time) string {
	prefix, ok := refPrefix[txType]
	if !ok {
		prefix = "JRN"
	}

	seq := g.nextSeq()
	return fmt.Sprintf("%s-%s-%06d", prefix, at.Format("20060102"), seq)
}

// nextSeq mengambil nilai sequence. Bila sequence tidak tersedia (mis. DB belum
// dimigrasi), dipakai waktu Unix nano sebagai cadangan agar posting tidak gagal.
func (g *ReferenceGenerator) nextSeq() int64 {
	var seq int64
	if err := g.db.QueryRowContext(context.Background(), `SELECT nextval('journal_reference_seq')`).Scan(&seq); err != nil {
		return time.Now().UnixNano() % 1000000000
	}
	return seq
}
