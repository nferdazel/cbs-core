package postgres

import (
	"context"
	"database/sql"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// TimeDepositReportRepository menyediakan baris per kontrak deposito berjangka pada
// posisi akhir periode, kebutuhan Form 12.00 "Daftar Deposito".
//
// Golongan nasabah dibaca apa adanya dari customers.ojk_pihak_lawan_code (Lampiran 02),
// hubungan dari customers.ojk_hubungan_bank_code, dan lokasi dari
// customers.ojk_kabupaten_code (Lampiran 03). Sandi yang belum diisi dibiarkan kosong
// dan form menulis "-", bukan menebak.
//
// Posisi yang dibaca adalah AKHIR PERIODE (asOf): kontrak dihitung bila sudah
// ditempatkan (start_date <= asOf) dan belum ditutup/dicairkan pada asOf (closed_at
// NULL atau setelah asOf). Rollover menulis ulang start_date/placement_amount sehingga
// riwayat sebelum rollover tidak dapat direkonstruksi; yang dilaporkan adalah kontrak
// sebagaimana tercatat pada asOf.
type TimeDepositReportRepository struct {
	db *sql.DB
}

func NewTimeDepositReportRepository(db *sql.DB) *TimeDepositReportRepository {
	return &TimeDepositReportRepository{db: db}
}

// timeDepositReportQuery menghimpun kontrak deposito berjangka pada posisi akhir
// periode $1. Bunga (profit_rate) hanyalah persen bila profit_type INTEREST; untuk
// MARGIN/BAGI_HASIL nilainya nisbah/parameter lain dan form menulis "-".
const timeDepositReportQuery = `
	SELECT
		d.account_number,
		COALESCE(c.cif_number, '') AS counterparty_cif,
		COALESCE(c.ojk_pihak_lawan_code, '') AS customer_type_code,
		COALESCE(c.ojk_hubungan_bank_code, '') AS hubungan_bank_code,
		COALESCE(c.ojk_kabupaten_code, '') AS location_code,
		COALESCE(d.profit_type, '') AS profit_type,
		d.placement_amount,
		d.start_date,
		d.maturity_date,
		d.profit_rate
	FROM time_deposits d
	JOIN customers c ON c.id = d.customer_id
	WHERE d.start_date <= $1::date
		AND (d.closed_at IS NULL OR d.closed_at::date > $1::date)
	ORDER BY d.account_number`

func (r *TimeDepositReportRepository) ListTimeDepositsForOJK(ctx context.Context, asOf time.Time) ([]domain.TimeDepositAggregate, error) {
	result, err := r.db.QueryContext(ctx, timeDepositReportQuery, asOf)
	if err != nil {
		return nil, err
	}
	defer func() { _ = result.Close() }()

	var out []domain.TimeDepositAggregate
	for result.Next() {
		var row domain.TimeDepositAggregate
		if err := result.Scan(
			&row.AccountNumber, &row.CounterpartyCIF, &row.CustomerTypeCode,
			&row.HubunganBankCode, &row.LocationCode, &row.ProfitType,
			&row.PlacementAmount, &row.StartDate, &row.MaturityDate, &row.ProfitRate,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, result.Err()
}

var _ domain.TimeDepositReportRepository = (*TimeDepositReportRepository)(nil)
