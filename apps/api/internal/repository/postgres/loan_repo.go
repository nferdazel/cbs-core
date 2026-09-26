package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type LoanRepository struct {
	db *sql.DB
}

func NewLoanRepository(db *sql.DB) *LoanRepository {
	return &LoanRepository{db: db}
}

// loanColumns memakai cast ::text untuk kolom bertipe enum kustom (loan_type,
// status). Tanpa cast, nilainya tidak dapat dipindai ke string Go.
const loanColumns = `id, loan_number, customer_id, product_id, branch_id, disbursement_account_id, loan_type::text, status::text,
	principal_amount, acquisition_cost, deferred_margin, interest_rate_annual, margin_amount, profit_sharing_ratio,
	total_payable, term_months, monthly_installment, outstanding_principal, penalty_accrued,
	collectibility, dpd, accrual_status, COALESCE(ckpn_method, 'COLLECTIVE'), required_ppap, required_ckpn, COALESCE(ckpn_individual_target, 0),
	is_restructured, restructured_count, restructured_at, restructuring_reason,
	pre_restructure_collectibility,
	akad_number, akad_date, purpose,
	ao_id, approved_by, approved_at, disbursed_at,
	created_at, updated_at,
	COALESCE((SELECT b.code FROM branches b WHERE b.id = loans.branch_id), ''),
	(SELECT MAX(sf.due_date) FROM loan_schedules sf WHERE sf.loan_id = loans.id),
	original_eir_monthly, original_eir_method, original_eir_basis, original_eir_calculated_at, restructure_loss_balance, rejection_reason,
	written_off_amount,
	ojk_jenis_penggunaan_code, ojk_periode_pembayaran_code, ojk_kabupaten_code`

func scanLoan(row interface{ Scan(...any) error }) (*domain.Loan, error) {
	var l domain.Loan
	var aoID, approvedBy sql.NullString
	var approvedAt, disbursedAt, restructuredAt, akadDate sql.NullTime
	var restructuringReason, akadNumber, purpose sql.NullString
	var preRestructure sql.NullString
	var finalDue, eirCalculatedAt sql.NullTime
	var eirMethod, eirBasis sql.NullString
	var rejectionReason sql.NullString
	var ojkJenisPenggunaan, ojkPeriodePembayaran, ojkKabupaten sql.NullString

	err := row.Scan(
		&l.ID, &l.LoanNumber, &l.CustomerID, &l.ProductID, &l.BranchID, &l.DisbursementAccountID, &l.LoanType, &l.Status,
		&l.PrincipalAmount, &l.AcquisitionCost, &l.DeferredMargin, &l.InterestRateAnnual, &l.MarginAmount, &l.ProfitSharingRatio,
		&l.TotalPayable, &l.TermMonths, &l.MonthlyInstallment, &l.OutstandingPrincipal, &l.PenaltyAccrued,
		&l.Collectibility, &l.DPD, &l.AccrualStatus, &l.CKPNMethod, &l.RequiredPPAP,
		&l.RequiredCKPN, &l.CKPNIndividualTarget,
		&l.IsRestructured, &l.RestructuredCount, &restructuredAt, &restructuringReason,
		&preRestructure,
		&akadNumber, &akadDate, &purpose,
		&aoID, &approvedBy, &approvedAt, &disbursedAt,
		&l.CreatedAt, &l.UpdatedAt,
		&l.BranchCode,
		&finalDue,
		&l.OriginalEIRMonthly, &eirMethod, &eirBasis, &eirCalculatedAt, &l.RestructureLossBalance,
		&rejectionReason,
		&l.WrittenOffAmount,
		&ojkJenisPenggunaan, &ojkPeriodePembayaran, &ojkKabupaten,
	)
	if err != nil {
		return nil, err
	}
	if eirMethod.Valid {
		l.OriginalEIRMethod = eirMethod.String
	}
	if eirBasis.Valid {
		l.OriginalEIRBasis = eirBasis.String
	}
	if eirCalculatedAt.Valid {
		l.OriginalEIRCalculatedAt = &eirCalculatedAt.Time
	}
	if aoID.Valid {
		id, _ := uuid.Parse(aoID.String)
		l.AOID = &id
	}
	if approvedBy.Valid {
		id, _ := uuid.Parse(approvedBy.String)
		l.ApprovedBy = &id
	}
	if approvedAt.Valid {
		l.ApprovedAt = &approvedAt.Time
	}
	if disbursedAt.Valid {
		l.DisbursedAt = &disbursedAt.Time
	}
	if restructuredAt.Valid {
		l.RestructuredAt = &restructuredAt.Time
	}
	if restructuringReason.Valid {
		l.RestructuringReason = restructuringReason.String
	}
	if preRestructure.Valid {
		l.PreRestructureCollectibility = domain.OJKCollectibility(preRestructure.String)
	}
	if akadNumber.Valid {
		l.AkadNumber = akadNumber.String
	}
	if akadDate.Valid {
		l.AkadDate = &akadDate.Time
	}
	if purpose.Valid {
		l.Purpose = purpose.String
	}
	if finalDue.Valid {
		l.FinalDueDate = &finalDue.Time
	}
	if rejectionReason.Valid {
		l.RejectionReason = rejectionReason.String
	}
	l.OJKJenisPenggunaanCode = ojkJenisPenggunaan.String
	l.OJKPeriodePembayaranCode = ojkPeriodePembayaran.String
	l.OJKKabupatenCode = ojkKabupaten.String
	return &l, nil
}

func (r *LoanRepository) Create(ctx context.Context, l *domain.Loan, schedules []domain.LoanSchedule) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	q := `INSERT INTO loans
		(id, loan_number, customer_id, product_id, branch_id, disbursement_account_id, loan_type, status,
		 principal_amount, acquisition_cost, deferred_margin, interest_rate_annual, margin_amount, profit_sharing_ratio,
		 total_payable, term_months, monthly_installment, outstanding_principal, penalty_accrued,
		 collectibility, dpd, accrual_status, required_ppap,
		 akad_number, akad_date, purpose, ao_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29)`

	_, err = tx.ExecContext(ctx, q,
		l.ID, l.LoanNumber, l.CustomerID, l.ProductID, l.BranchID, l.DisbursementAccountID, l.LoanType, l.Status,
		l.PrincipalAmount, l.AcquisitionCost, l.DeferredMargin, l.InterestRateAnnual, l.MarginAmount, l.ProfitSharingRatio,
		l.TotalPayable, l.TermMonths, l.MonthlyInstallment, l.OutstandingPrincipal, l.PenaltyAccrued,
		l.Collectibility, l.DPD, l.AccrualStatus, l.RequiredPPAP,
		l.AkadNumber, l.AkadDate, l.Purpose, l.AOID, l.CreatedAt, l.UpdatedAt,
	)
	if err != nil {
		return err
	}

	sq := `INSERT INTO loan_schedules
		(id, loan_id, installment_no, due_date, principal_amount, profit_amount, total_installment,
		 paid_principal, paid_profit, profit_type, outstanding_principal, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	for _, s := range schedules {
		if _, err := tx.ExecContext(ctx, sq,
			s.ID, s.LoanID, s.InstallmentNo, s.DueDate, s.PrincipalAmount, s.ProfitAmount, s.TotalInstallment,
			s.PaidPrincipal, s.PaidProfit, s.ProfitType, s.OutstandingPrincipal, s.Status, s.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *LoanRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Loan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+loanColumns+` FROM loans WHERE id = $1`, id)
	l, err := scanLoan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrLoanNotFound
	}
	if err != nil {
		return nil, err
	}

	schedules, err := r.GetSchedules(ctx, l.ID)
	if err == nil {
		l.Schedules = schedules
	}
	return l, nil
}

// LockLoanTx membaca kredit dengan SELECT ... FOR UPDATE di dalam transaksi pemanggil.
// Kunci ini menyerialkan seluruh jalur yang mengubah uang/jadwal kredit: pembayaran
// angsuran, koreksi nominal, pembatalan pencairan, akrual denda, dan akrual bunga.
// Jalur PPAP TIDAK memakai kunci ini — ia hanya memperbarui baris kredit tanpa
// SELECT ... FOR UPDATE, jadi jangan mengandalkannya menyerialkan PPAP.
// Tanpa kunci, pembayaran yang commit di antara baca dan hapus jadwal akan
// terhapus sementara jurnalnya tetap ada.
func (r *LoanRepository) LockLoanTx(ctx context.Context, tx any, id uuid.UUID) (*domain.Loan, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("loan: transaksi tidak valid")
	}
	row := sqlTx.QueryRowContext(ctx, `SELECT `+loanColumns+` FROM loans WHERE id = $1 FOR UPDATE`, id)
	l, err := scanLoan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrLoanNotFound
	}
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *LoanRepository) GetByNumber(ctx context.Context, loanNumber string) (*domain.Loan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+loanColumns+` FROM loans WHERE loan_number = $1`, loanNumber)
	l, err := scanLoan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrLoanNotFound
	}
	return l, err
}

func (r *LoanRepository) List(ctx context.Context, limit, offset int, actor domain.Actor) ([]domain.Loan, int, error) {
	where, whereArgs := branchReadClause("loans.branch_id", actor)
	// Buku kredit dibaca dari produk (subquery, bukan join) agar kolom loanColumns
	// yang tidak ter-kualifikasi tidak menjadi ambigu. Kredit lama tanpa produk
	// menghasilkan NULL dan tetap terlihat bagi semua buku (lihat bookReadClause).
	bookColumn := "(SELECT p.book FROM banking_products p WHERE p.id = loans.product_id)"
	if clause, args := bookReadClause(bookColumn, actor, len(whereArgs)+1); clause != "" {
		whereArgs = append(whereArgs, args...)
		where = andCondition(where, clause)
	}

	countQuery := "SELECT COUNT(*) FROM loans"
	if where != "" {
		countQuery += " WHERE " + where
	}
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + loanColumns + ` FROM loans`
	if where != "" {
		query += " WHERE " + where
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(whereArgs)+1, len(whereArgs)+2)
	args := append(append([]any{}, whereArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.Loan
	for rows.Next() {
		l, err := scanLoan(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *l)
	}
	return list, total, nil
}

// listLoanScheduleAggregatesQuery merangkum jadwal angsuran per kredit dalam satu
// query: tanggal angsuran pertama (MIN due_date), nominal tunggakan pokok+bunga
// yang sudah jatuh tempo sebelum asOf dan belum lunas, serta piutang bunga yang
// masih tercatat. Enum installment_status hanya mengenal PENDING, PAID, OVERDUE,
// PARTIAL: tidak ada nilai batal, dan hanya PAID yang berarti lunas, sehingga tiga
// status sisanya dihitung sebagai tunggakan. Nominal tunggakan per baris jadwal
// adalah (principal_amount - paid_principal) + (profit_amount - paid_profit);
// baris yang hasilnya <= 0 (mis. lebih bayar) tidak dijumlahkan.
//
// AccruedProfit menjumlahkan profit_accrued_amount (sisa akruan yang belum
// diselesaikan pembayaran) seluruh angsuran, yaitu piutang bunga 10400 yang masih
// tercatat. Tidak difilter asOf: akruan hanya terbentuk untuk angsuran yang sudah
// jatuh tempo pada saat EOD berjalan, dan sisa yang belum dibayar memang masih
// menjadi piutang bunga pada tanggal laporan.
//
// %s menerima filter cabang/buku aktor supaya cakupan data sama dengan List.
const listLoanScheduleAggregatesQuery = `
	SELECT
		l.loan_number,
		MIN(s.due_date) AS first_installment_date,
		COALESCE(SUM(
			CASE
				WHEN s.due_date < $%d
					AND s.status <> 'PAID'
					AND (s.principal_amount - s.paid_principal) + (s.profit_amount - s.paid_profit) > 0
				THEN (s.principal_amount - s.paid_principal) + (s.profit_amount - s.paid_profit)
				ELSE 0
			END
		), 0) AS overdue_unpaid,
		COALESCE(SUM(s.profit_accrued_amount), 0) AS accrued_profit
	FROM loans l
	JOIN loan_schedules s ON s.loan_id = l.id
	WHERE TRUE%s
	GROUP BY l.id
	ORDER BY l.loan_number`

// ListLoanScheduleAggregates mengambil agregat jadwal seluruh kredit dalam satu
// query GROUP BY loan_id. Filter cabang dan buku diterapkan seperti List agar aktor
// tidak melihat agregat kredit di luar kewenangannya.
func (r *LoanRepository) ListLoanScheduleAggregates(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.LoanScheduleAggregate, error) {
	var args []any
	filter := ""
	// branchReadClause memakai placeholder $1, jadi klausa cabang harus lebih dulu.
	if clause, bargs := branchReadClause("l.branch_id", actor); clause != "" {
		filter += " AND " + clause
		args = append(args, bargs...)
	}
	bookColumn := "(SELECT p.book FROM banking_products p WHERE p.id = l.product_id)"
	if clause, bargs := bookReadClause(bookColumn, actor, len(args)+1); clause != "" {
		filter += " AND " + clause
		args = append(args, bargs...)
	}
	asOfArg := len(args) + 1
	args = append(args, asOf)

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(listLoanScheduleAggregatesQuery, asOfArg, filter), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.LoanScheduleAggregate
	for rows.Next() {
		var a domain.LoanScheduleAggregate
		var firstInstallment sql.NullTime
		if err := rows.Scan(&a.LoanNumber, &firstInstallment, &a.OverdueUnpaid, &a.AccruedProfit); err != nil {
			return nil, err
		}
		if firstInstallment.Valid {
			t := firstInstallment.Time
			a.FirstInstallmentDate = &t
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (r *LoanRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.LoanStatus, approvedBy *uuid.UUID) error {
	return updateLoanStatus(ctx, r.db, id, status, approvedBy)
}

// UpdateStatusTx menjalankan update status di dalam transaksi pemanggil, agar
// perubahan status dan penulisan audit bisa commit bersama.
func (r *LoanRepository) UpdateStatusTx(ctx context.Context, tx any, id uuid.UUID, status domain.LoanStatus, approvedBy *uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return updateLoanStatus(ctx, sqlTx, id, status, approvedBy)
}

func updateLoanStatus(ctx context.Context, exec execer, id uuid.UUID, status domain.LoanStatus, approvedBy *uuid.UUID) error {
	q := `UPDATE loans SET status=$1, approved_by=$2, approved_at=NOW(), updated_at=NOW() WHERE id=$3`
	_, err := exec.ExecContext(ctx, q, status, approvedBy, id)
	return err
}

// RejectLoanTx menandai kredit REJECTED dan menyimpan alasannya dalam satu pernyataan,
// sehingga status tanpa alasan (atau sebaliknya) tidak mungkin tersimpan terpisah.
// Penjaga status ada di query, bukan hanya di service: penolakan yang membaca status
// basi (TOCTOU) tidak boleh menimpa kredit yang sudah APPROVED/DISBURSED. Kolom
// persetujuan (approved_by/approved_at) sengaja TIDAK diisi saat penolakan — kredit
// yang ditolak bukan kredit yang disetujui; jejaknya ada di rejection_reason,
// maker_checker_requests, dan audit log.
func (r *LoanRepository) RejectLoanTx(ctx context.Context, tx any, id uuid.UUID, reason string) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return rejectLoan(ctx, sqlTx, id, reason)
}

func rejectLoan(ctx context.Context, exec execer, id uuid.UUID, reason string) error {
	q := `UPDATE loans SET status=$1, rejection_reason=$2, updated_at=NOW()
		WHERE id=$3 AND status=$4`
	res, err := exec.ExecContext(ctx, q, domain.LoanStatusRejected, reason, id, domain.LoanStatusPendingApproval)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrLoanAlreadyApproved
	}
	return nil
}

func (r *LoanRepository) MarkDisbursed(ctx context.Context, id uuid.UUID, outstanding decimal.Decimal) error {
	return markLoanDisbursed(ctx, r.db, id, outstanding)
}

// MarkDisbursedTx menjalankan penandaan pencairan di dalam transaksi pemanggil.
func (r *LoanRepository) MarkDisbursedTx(ctx context.Context, tx any, id uuid.UUID, outstanding decimal.Decimal) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return markLoanDisbursed(ctx, sqlTx, id, outstanding)
}

func markLoanDisbursed(ctx context.Context, exec execer, id uuid.UUID, outstanding decimal.Decimal) error {
	// Guard status: penandaan hanya sah dari APPROVED. Service sudah memeriksa status
	// baris hasil kunci, tetapi guard di query menutup celah bila pemeriksaan itu
	// dilewati/kembali longgar: kredit yang sudah DISBURSED tidak bisa dicairkan lagi
	// (jurnal ganda), dan status lain tidak bisa "dipromosikan" menjadi cair.
	q := `UPDATE loans SET status=$1, disbursed_at=NOW(), outstanding_principal=$2, updated_at=NOW()
		WHERE id=$3 AND status=$4`
	res, err := exec.ExecContext(ctx, q, domain.LoanStatusDisbursed, outstanding, id, domain.LoanStatusApproved)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrLoanNotApproved
	}
	return nil
}

// queryer adalah sumber baris yang bisa berupa *sql.DB (di luar transaksi) atau
// *sql.Tx (di dalam transaksi pemanggil).
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

const schedulesQuery = `SELECT id, loan_id, installment_no, due_date, principal_amount, profit_amount,
	total_installment, paid_principal, paid_profit, profit_type, outstanding_principal, status::text, paid_at, created_at,
	profit_accrued_at, profit_accrued_amount, restructure_loss_amortized_at, restructure_loss_amortized_amount
	FROM loan_schedules WHERE loan_id = $1 ORDER BY installment_no ASC`

func (r *LoanRepository) GetSchedules(ctx context.Context, loanID uuid.UUID) ([]domain.LoanSchedule, error) {
	return getSchedules(ctx, r.db, loanID)
}

// GetSchedulesTx membaca jadwal di dalam transaksi pemanggil, sama seperti GetSchedules.
func (r *LoanRepository) GetSchedulesTx(ctx context.Context, tx any, loanID uuid.UUID) ([]domain.LoanSchedule, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("loan: transaksi tidak valid")
	}
	return getSchedules(ctx, sqlTx, loanID)
}

func getSchedules(ctx context.Context, q queryer, loanID uuid.UUID) ([]domain.LoanSchedule, error) {
	rows, err := q.QueryContext(ctx, schedulesQuery, loanID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.LoanSchedule
	for rows.Next() {
		var s domain.LoanSchedule
		var paidAt, profitAccruedAt, lossAmortizedAt sql.NullTime
		if err := rows.Scan(
			&s.ID, &s.LoanID, &s.InstallmentNo, &s.DueDate, &s.PrincipalAmount,
			&s.ProfitAmount, &s.TotalInstallment, &s.PaidPrincipal, &s.PaidProfit,
			&s.ProfitType, &s.OutstandingPrincipal, &s.Status, &paidAt, &s.CreatedAt,
			&profitAccruedAt, &s.ProfitAccruedAmount,
			&lossAmortizedAt, &s.RestructureLossAmortizedAmount,
		); err != nil {
			return nil, err
		}
		if paidAt.Valid {
			s.PaidAt = &paidAt.Time
		}
		if profitAccruedAt.Valid {
			s.ProfitAccruedAt = &profitAccruedAt.Time
		}
		if lossAmortizedAt.Valid {
			s.RestructureLossAmortizedAt = &lossAmortizedAt.Time
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// UpdateSchedulePayment mencatat pembayaran angsuran dan mengurangi sisa akruan bunga
// yang diselesaikan. GREATEST(..., 0) menjaga piutang bunga (10400) tidak pernah
// negatif bila ada pembayaran yang mencoba menyelesaikan lebih dari yang diakru.
func (r *LoanRepository) UpdateSchedulePayment(ctx context.Context, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status domain.InstallmentStatus) error {
	return updateSchedulePayment(ctx, r.db, scheduleID, paidPrincipal, paidProfit, settleAccrued, status)
}

// UpdateSchedulePaymentTx mencatat pembayaran angsuran di dalam transaksi pemanggil,
// agar jurnal dan perubahan jadwal tidak pernah terpisah.
func (r *LoanRepository) UpdateSchedulePaymentTx(ctx context.Context, tx any, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status domain.InstallmentStatus) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return updateSchedulePayment(ctx, sqlTx, scheduleID, paidPrincipal, paidProfit, settleAccrued, status)
}

func updateSchedulePayment(ctx context.Context, exec execer, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status domain.InstallmentStatus) error {
	// paid_at hanya diisi saat angsuran benar-benar lunas; angsuran sebagian tetap
	// menyimpan tanggal pembayaran pertamanya di kolom lain bila diperlukan kelak.
	q := `UPDATE loan_schedules
		SET paid_principal = paid_principal + $1, paid_profit = paid_profit + $2,
		    profit_accrued_amount = GREATEST(profit_accrued_amount - $3, 0),
		    status = $4,
		    paid_at = CASE WHEN $4::installment_status = 'PAID' THEN NOW() ELSE paid_at END
		WHERE id = $5`
	_, err := exec.ExecContext(ctx, q, paidPrincipal, paidProfit, settleAccrued, status, scheduleID)
	return err
}

// UpdateRestructure menyimpan hasil restrukturisasi loan beserta jadwal angsuran baru
// dalam satu transaksi: update parameter dan replace seluruh schedule lama.
func (r *LoanRepository) UpdateRestructure(ctx context.Context, l *domain.Loan, schedules []domain.LoanSchedule) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := r.updateRestructureTx(ctx, tx, l, schedules); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateRestructureTx menjalankan penyimpanan restrukturisasi di dalam transaksi
// pemanggil. Dipakai jalur kerugian restrukturisasi agar jurnal kerugian dan jadwal
// baru commit bersama; polanya sama dengan UpdateRestructure.
func (r *LoanRepository) UpdateRestructureTx(ctx context.Context, tx any, l *domain.Loan, schedules []domain.LoanSchedule) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return r.updateRestructureTx(ctx, sqlTx, l, schedules)
}

func (r *LoanRepository) updateRestructureTx(ctx context.Context, tx *sql.Tx, l *domain.Loan, schedules []domain.LoanSchedule) error {
	// restructure_loss_balance ikut ditulis agar jalur saklar-mati (UpdateRestructure)
	// tidak menghapus saldo kerugian yang sudah diakui: nilai pada l dibaca dari baris
	// yang sama, jadi tetap dipertahankan.
	q := `UPDATE loans SET
		term_months=$1, interest_rate_annual=$2, margin_amount=$3, total_payable=$4, monthly_installment=$5,
		collectibility=$6, accrual_status=$7, required_ppap=$8,
		is_restructured=$9, restructured_count=$10, restructured_at=$11, restructuring_reason=$12,
		pre_restructure_collectibility=$13, restructure_loss_balance=$14,
		updated_at=NOW()
		WHERE id=$15`

	_, err := tx.ExecContext(ctx, q,
		l.TermMonths, l.InterestRateAnnual, l.MarginAmount, l.TotalPayable, l.MonthlyInstallment,
		l.Collectibility, l.AccrualStatus, l.RequiredPPAP,
		l.IsRestructured, l.RestructuredCount, l.RestructuredAt, l.RestructuringReason,
		l.PreRestructureCollectibility, l.RestructureLossBalance,
		l.ID,
	)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM loan_schedules WHERE loan_id = $1`, l.ID); err != nil {
		return err
	}

	sq := `INSERT INTO loan_schedules
		(id, loan_id, installment_no, due_date, principal_amount, profit_amount, total_installment,
		 paid_principal, paid_profit, profit_type, outstanding_principal, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	for _, s := range schedules {
		if _, err := tx.ExecContext(ctx, sq,
			s.ID, s.LoanID, s.InstallmentNo, s.DueDate, s.PrincipalAmount, s.ProfitAmount, s.TotalInstallment,
			s.PaidPrincipal, s.PaidProfit, s.ProfitType, s.OutstandingPrincipal, s.Status, s.CreatedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

// UpdateOriginalEIRTx menyimpan suku bunga efektif orisinal beserta dasar auditnya saat
// pencairan, di dalam transaksi pemanggil. Kolom nullable diisi jsonb/nama metode.
func (r *LoanRepository) UpdateOriginalEIRTx(ctx context.Context, tx any, loanID uuid.UUID, monthly decimal.Decimal, method string, basis []byte, calculatedAt time.Time) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	const q = `UPDATE loans
		SET original_eir_monthly=$1, original_eir_method=$2, original_eir_basis=$3,
		    original_eir_calculated_at=$4, updated_at=NOW()
		WHERE id=$5`
	_, err := sqlTx.ExecContext(ctx, q, monthly, method, basis, calculatedAt, loanID)
	return err
}

// CorrectLoanAmountTx memperbarui nominal kredit beserta sisa pokok dan mengganti
// seluruh jadwal angsuran di dalam transaksi pemanggil. Pola ganti jadwalnya sama
// dengan UpdateRestructure, tetapi baris jadwal yang sudah dibayar ikut disimpan
// ulang apa adanya — termasuk akruan bunga dan tanggal bayarnya — karena koreksi
// nominal tidak boleh menghapus riwayat yang sudah berjalan.
func (r *LoanRepository) CorrectLoanAmountTx(ctx context.Context, tx any, l *domain.Loan, schedules []domain.LoanSchedule) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}

	q := `UPDATE loans SET
		principal_amount=$1, total_payable=$2, monthly_installment=$3, outstanding_principal=$4, updated_at=NOW()
		WHERE id=$5`
	if _, err := sqlTx.ExecContext(ctx, q,
		l.PrincipalAmount, l.TotalPayable, l.MonthlyInstallment, l.OutstandingPrincipal, l.ID,
	); err != nil {
		return err
	}

	if _, err := sqlTx.ExecContext(ctx, `DELETE FROM loan_schedules WHERE loan_id = $1`, l.ID); err != nil {
		return err
	}

	sq := `INSERT INTO loan_schedules
		(id, loan_id, installment_no, due_date, principal_amount, profit_amount, total_installment,
		 paid_principal, paid_profit, profit_type, outstanding_principal, status, paid_at, created_at,
		 profit_accrued_at, profit_accrued_amount)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`
	for _, s := range schedules {
		if _, err := sqlTx.ExecContext(ctx, sq,
			s.ID, s.LoanID, s.InstallmentNo, s.DueDate, s.PrincipalAmount, s.ProfitAmount, s.TotalInstallment,
			s.PaidPrincipal, s.PaidProfit, s.ProfitType, s.OutstandingPrincipal, s.Status, s.PaidAt, s.CreatedAt,
			s.ProfitAccruedAt, s.ProfitAccruedAmount,
		); err != nil {
			return err
		}
	}
	return nil
}

// NextCorrectionCountTx menaikkan penghitung koreksi nominal kredit dan mengembalikan
// nilai barunya di dalam transaksi pemanggil. Kenaikan berada di transaksi yang sama
// dengan jurnalnya: bila transaksi gagal, kenaikan ikut ter-rollback sehingga nomor
// koreksi tidak terpakai percuma. Nilai ini masuk ke kunci idempotensi jurnal koreksi
// agar rangkaian 10jt -> 12jt -> 10jt -> 12jt tidak bertabrakan dengan kunci lama.
func (r *LoanRepository) NextCorrectionCountTx(ctx context.Context, tx any, loanID uuid.UUID) (int, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return 0, errors.New("loan: transaksi tidak valid")
	}
	var count int
	if err := sqlTx.QueryRowContext(ctx,
		`UPDATE loans SET correction_count = correction_count + 1, updated_at = NOW()
		 WHERE id = $1 RETURNING correction_count`, loanID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LoanRepository) UpdateCollectibility(ctx context.Context, id uuid.UUID, col domain.OJKCollectibility, dpd int, accrual domain.AccrualStatus, ppap decimal.Decimal) error {
	q := `UPDATE loans SET collectibility=$1, dpd=$2, accrual_status=$3, required_ppap=$4, updated_at=NOW() WHERE id=$5`
	_, err := r.db.ExecContext(ctx, q, col, dpd, accrual, ppap, id)
	return err
}

func (r *LoanRepository) UpdateOutstanding(ctx context.Context, id uuid.UUID, outstanding, penalty decimal.Decimal) error {
	return updateOutstanding(ctx, r.db, id, outstanding, penalty)
}

// UpdateOutstandingTx menyimpan sisa pokok dan denda di dalam transaksi pemanggil.
func (r *LoanRepository) UpdateOutstandingTx(ctx context.Context, tx any, id uuid.UUID, outstanding, penalty decimal.Decimal) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return updateOutstanding(ctx, sqlTx, id, outstanding, penalty)
}

func updateOutstanding(ctx context.Context, exec execer, id uuid.UUID, outstanding, penalty decimal.Decimal) error {
	q := `UPDATE loans SET outstanding_principal=$1, penalty_accrued=$2, updated_at=NOW() WHERE id=$3`
	_, err := exec.ExecContext(ctx, q, outstanding, penalty, id)
	return err
}

// SetWrittenOffAmountTx menyimpan nilai hapus buku di dalam transaksi pemanggil. Nilai
// ini menjadi batas akumulasi pemulihan, sehingga harus commit bersama jurnal dan
// perubahan status: tidak boleh ada kredit WRITTEN_OFF tanpa nilai hapus buku.
func (r *LoanRepository) SetWrittenOffAmountTx(ctx context.Context, tx any, id uuid.UUID, amount decimal.Decimal) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE loans SET written_off_amount = $1, updated_at = NOW() WHERE id = $2`, amount, id)
	return err
}

// SumRecoveredAmountTx menjumlahkan nominal pemulihan kredit hapus buku dari jurnal
// RECOV-<nomor kredit>- , MENGECUALIKAN kunci idempotensi yang sedang diproses supaya
// pengulangan permintaan yang sama (idempoten) tidak ditolak oleh jurnalnya sendiri.
// Jurnal dipilih, bukan audit, karena idempotency_key unik mencegah satu penerimaan
// dihitung dua kali. Kaki DEBIT dipakai: pemetaan LOAN_RECOVERY menaruh nominal
// penerimaan di kedua kaki, sehingga satu jurnal menambah tepat sebesar nominalnya.
func (r *LoanRepository) SumRecoveredAmountTx(ctx context.Context, tx any, loanNumber, excludeIdempotencyKey string) (decimal.Decimal, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return decimal.Zero, errors.New("loan: transaksi tidak valid")
	}
	var total decimal.Decimal
	err := sqlTx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(jl.amount), 0)
		FROM journal_entries je
		JOIN journal_lines jl ON jl.journal_entry_id = je.id
		WHERE starts_with(je.idempotency_key, $1)
		  AND je.idempotency_key <> $2
		  AND jl.direction = 'DEBIT'`, "RECOV-"+loanNumber+"-", excludeIdempotencyKey).Scan(&total)
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}

// listPenaltyCandidatesQuery mengambil kredit aktif beserta pokok angsuran yang lewat
// jatuh tempo. Dasar denda adalah pokok angsuran yang belum dibayar, bukan seluruh
// sisa pokok: GREATEST(principal_amount - paid_principal, 0) tetap benar untuk
// angsuran sebagian. Cast ::text wajib untuk kolom enum status. final_due_date
// (jatuh tempo angsuran terakhir) ikut diambil karena penentuan kolektibilitas
// memakai dimensi jatuh tempo Kredit, sama seperti kandidat akrual bunga.
//
// %s menerima filter buku instalasi/aktor (bookReadClause) agar batch denda tidak
// memproses lini usaha yang tidak aktif.
const listPenaltyCandidatesQuery = `
	SELECT
		l.id,
		l.loan_number,
		l.product_id,
		l.disbursement_account_id,
		l.status::text,
		COALESCE(SUM(GREATEST(s.principal_amount - s.paid_principal, 0)), 0) AS overdue_principal,
		l.penalty_accrued,
		MIN(s.due_date) AS oldest_due_date,
		l.penalty_last_accrued_on,
		(SELECT MAX(sf.due_date) FROM loan_schedules sf
			WHERE sf.loan_id = l.id) AS final_due_date,
		l.outstanding_principal
	FROM loans l
	JOIN loan_schedules s
		ON s.loan_id = l.id
		AND s.status <> 'PAID'
		AND s.due_date <= $1
	WHERE l.status IN ('DISBURSED', 'DEFAULTED')
		AND l.outstanding_principal > 0%s
	GROUP BY l.id
	ORDER BY l.loan_number`

func (r *LoanRepository) ListPenaltyCandidates(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.LoanPenaltyCandidate, error) {
	bookColumn := "(SELECT p.book FROM banking_products p WHERE p.id = l.product_id)"
	args := []any{asOf}
	filter := ""
	if clause, bargs := bookReadClause(bookColumn, actor, len(args)+1); clause != "" {
		filter = " AND " + clause
		args = append(args, bargs...)
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(listPenaltyCandidatesQuery, filter), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.LoanPenaltyCandidate
	for rows.Next() {
		var c domain.LoanPenaltyCandidate
		var productID sql.NullString
		var status string
		var oldestDue, lastAccrued, finalDue sql.NullTime

		if err := rows.Scan(
			&c.LoanID, &c.LoanNumber, &productID, &c.DisbursementAccountID, &status,
			&c.OverduePrincipal, &c.PenaltyAccrued, &oldestDue, &lastAccrued, &finalDue,
			&c.OutstandingPrincipal,
		); err != nil {
			return nil, err
		}
		if productID.Valid {
			id, err := uuid.Parse(productID.String)
			if err != nil {
				return nil, err
			}
			c.ProductID = &id
		}
		c.Status = domain.LoanStatus(status)
		if oldestDue.Valid {
			t := oldestDue.Time
			c.OldestDueDate = &t
		}
		if lastAccrued.Valid {
			t := lastAccrued.Time
			c.LastAccruedOn = &t
		}
		if finalDue.Valid {
			t := finalDue.Time
			c.FinalDueDate = &t
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// AddPenaltyAccruedTx menambah penalty_accrued dan memajukan
// penalty_last_accrued_on, tetapi hanya bila jurnal dengan idempotency_key tersebut
// belum ada. Tanggal terakhir dikunci dengan GREATEST agar tidak pernah mundur bila
// ada eksekusi yang datang tidak berurutan. Penambahan dan jurnalnya berada pada
// transaksi yang sama, sehingga angka di loans dan jurnal tidak pernah terpisah.
// Baris loans terkunci oleh UPDATE, yang menyerialkan dua batch paralel untuk kredit
// yang sama: yang kedua akan melihat jurnal sudah ada dan tidak menambah lagi.
func (r *LoanRepository) AddPenaltyAccruedTx(ctx context.Context, tx any, loanID uuid.UUID, amount decimal.Decimal, idempotencyKey string, accruedOn time.Time) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("loan: transaksi tidak valid")
	}
	q := `UPDATE loans
		SET penalty_accrued = penalty_accrued + $1,
		    penalty_last_accrued_on = GREATEST(COALESCE(penalty_last_accrued_on, $2), $2),
		    updated_at = NOW()
		WHERE id = $3
		  AND NOT EXISTS (SELECT 1 FROM journal_entries WHERE idempotency_key = $4)`
	res, err := sqlTx.ExecContext(ctx, q, amount, accruedOn, loanID, idempotencyKey)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// listInterestAccrualCandidatesQuery mengambil angsuran yang layak diakru:
// sudah jatuh tempo pada asOf, belum dibayar, porsi bunganya belum habis, dan belum
// pernah diakru. Hanya kredit konvensional (loan_type) yang disertakan; produk
// syariah tidak punya pemetaan INTEREST_ACCRUAL dan tidak boleh diakru. Jatuh tempo
// angsuran tertua yang belum dibayar ikut diambil untuk menghitung DPD/kolektibilitas.
//
// %s menerima filter buku instalasi/aktor (bookReadClause) agar batch akrual tidak
// memproses lini usaha yang tidak aktif.
const listInterestAccrualCandidatesQuery = `
	SELECT
		l.id,
		l.loan_number,
		l.product_id,
		l.loan_type::text,
		l.status::text,
		s.id,
		s.installment_no,
		s.due_date,
		GREATEST(s.profit_amount - s.paid_profit, 0) AS outstanding_profit,
		(SELECT MIN(s2.due_date) FROM loan_schedules s2
			WHERE s2.loan_id = l.id AND s2.status <> 'PAID') AS oldest_due_date,
		(SELECT MAX(sf.due_date) FROM loan_schedules sf
			WHERE sf.loan_id = l.id) AS final_due_date
	FROM loans l
	JOIN loan_schedules s
		ON s.loan_id = l.id
		AND s.status <> 'PAID'
		AND s.due_date <= $1
		AND s.profit_accrued_at IS NULL
		AND s.profit_amount - s.paid_profit > 0
	WHERE l.status IN ('DISBURSED', 'DEFAULTED')
		AND l.loan_type::text IN ('CONVENTIONAL_FLAT', 'CONVENTIONAL_ANNUITY')%s
	ORDER BY l.loan_number, s.installment_no`

func (r *LoanRepository) ListInterestAccrualCandidates(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.LoanInterestAccrualCandidate, error) {
	bookColumn := "(SELECT p.book FROM banking_products p WHERE p.id = l.product_id)"
	args := []any{asOf}
	filter := ""
	if clause, bargs := bookReadClause(bookColumn, actor, len(args)+1); clause != "" {
		filter = " AND " + clause
		args = append(args, bargs...)
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(listInterestAccrualCandidatesQuery, filter), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.LoanInterestAccrualCandidate
	for rows.Next() {
		var c domain.LoanInterestAccrualCandidate
		var productID sql.NullString
		var loanType, status string
		var oldestDue, finalDue sql.NullTime

		if err := rows.Scan(
			&c.LoanID, &c.LoanNumber, &productID, &loanType, &status,
			&c.ScheduleID, &c.InstallmentNo, &c.DueDate, &c.OutstandingProfit, &oldestDue,
			&finalDue,
		); err != nil {
			return nil, err
		}
		if productID.Valid {
			id, err := uuid.Parse(productID.String)
			if err != nil {
				return nil, err
			}
			c.ProductID = &id
		}
		c.LoanType = domain.LoanType(loanType)
		c.LoanStatus = domain.LoanStatus(status)
		if oldestDue.Valid {
			t := oldestDue.Time
			c.OldestDueDate = &t
		}
		if finalDue.Valid {
			t := finalDue.Time
			c.FinalDueDate = &t
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// AddScheduleProfitAccruedTx menambah profit_accrued_amount dan mengisi
// profit_accrued_at satu angsuran, tetapi hanya bila jurnal dengan idempotency_key
// tersebut belum ada. UPDATE mengunci baris loan_schedules sehingga dua batch paralel
// untuk angsuran yang sama terserialisasi: yang kedua melihat jurnal sudah ada dan
// tidak menambah lagi. Penambahan dan jurnalnya berada pada transaksi yang sama,
// sehingga angka sisa akruan dan jurnal tidak pernah terpisah.
func (r *LoanRepository) AddScheduleProfitAccruedTx(ctx context.Context, tx any, scheduleID uuid.UUID, amount decimal.Decimal, idempotencyKey string, accruedAt time.Time) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("loan: transaksi tidak valid")
	}
	q := `UPDATE loan_schedules
		SET profit_accrued_amount = profit_accrued_amount + $1,
		    profit_accrued_at = $2
		WHERE id = $3
		  AND NOT EXISTS (SELECT 1 FROM journal_entries WHERE idempotency_key = $4)`
	res, err := sqlTx.ExecContext(ctx, q, amount, accruedAt, scheduleID, idempotencyKey)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// HasInstallmentPaymentTx mendeteksi angsuran yang sudah dibayar. Satu baris saja
// yang punya paid_principal/paid_profit bukan nol atau status selain PENDING sudah
// cukup: uang dan jadwal sudah berjalan sehingga pencairan tidak boleh dibatalkan.
func (r *LoanRepository) HasInstallmentPaymentTx(ctx context.Context, tx any, loanID uuid.UUID) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("loan: transaksi tidak valid")
	}
	q := `SELECT COUNT(*)
		FROM loan_schedules
		WHERE loan_id = $1
		  AND (paid_principal <> 0 OR paid_profit <> 0 OR status <> 'PENDING')`
	var count int
	if err := sqlTx.QueryRowContext(ctx, q, loanID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasAccrualPostingsTx mendeteksi akrual yang sudah terposting dari state yang
// tersimpan bersama jurnalnya. Karena penulisan state dan jurnal berada di transaksi
// yang sama, salah satu dari berikut menandakan akrual sudah ada:
//   - jadwal dengan profit_accrued_at terisi atau profit_accrued_amount tidak nol
//     (akrual bunga, jurnal ACCR-<nomor kredit>-<angsuran>);
//   - kredit dengan penalty_accrued tidak nol atau penalty_last_accrued_on terisi
//     (akrual denda, jurnal PENALTY-<nomor kredit>-<tanggal>);
//   - kredit dengan required_ppap tidak nol (cadangan PPAP sudah dibukukan).
//
// Pemeriksaan ini sengaja berbasis state yang tersimpan di baris kredit/jadwal, bukan
// pencocokan string kunci jurnal: format kunci bisa berubah tanpa jejak di skema.
// Pemanggil harus memegang LockLoanTx agar hasilnya otoritatif.
func (r *LoanRepository) HasAccrualPostingsTx(ctx context.Context, tx any, loanID uuid.UUID) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("loan: transaksi tidak valid")
	}
	q := `SELECT
		EXISTS (SELECT 1 FROM loan_schedules
			WHERE loan_id = $1
			  AND (profit_accrued_at IS NOT NULL OR profit_accrued_amount <> 0))
		OR EXISTS (SELECT 1 FROM loans
			WHERE id = $1
			  AND (penalty_accrued <> 0 OR penalty_last_accrued_on IS NOT NULL OR required_ppap <> 0))`
	var has bool
	if err := sqlTx.QueryRowContext(ctx, q, loanID).Scan(&has); err != nil {
		return false, err
	}
	return has, nil
}

// DeleteSchedulesTx menghapus seluruh jadwal angsuran satu kredit. Pola yang sama
// dipakai jalur restrukturisasi yang mengganti jadwal lama dengan jadwal baru.
func (r *LoanRepository) DeleteSchedulesTx(ctx context.Context, tx any, loanID uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `DELETE FROM loan_schedules WHERE loan_id = $1`, loanID)
	return err
}

// GetDisbursementJournalRefTx mengambil nomor referensi jurnal pencairan kredit.
// Jurnal pencairan tidak menyimpan nomor kredit, jadi pencariannya lewat
// idempotency_key yang dibentuk DisburseLoan ("DISB-"+nomor kredit).
func (r *LoanRepository) GetDisbursementJournalRefTx(ctx context.Context, tx any, loanNumber string) (string, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return "", errors.New("loan: transaksi tidak valid")
	}
	var ref string
	err := sqlTx.QueryRowContext(ctx,
		`SELECT reference_number FROM journal_entries WHERE idempotency_key = $1`,
		"DISB-"+loanNumber).Scan(&ref)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("jurnal pencairan kredit %s tidak ditemukan", loanNumber)
	}
	if err != nil {
		return "", err
	}
	return ref, nil
}

// listRestructureLossAmortizationCandidatesQuery mengambil kredit yang saldo kerugian
// restrukturisasinya belum nol dan masih perlu diamortisasi pada tanggal bisnis:
// kredit aktif yang punya angsuran jatuh tempo dan belum ditandai diamortisasi, atau
// kredit yang sudah lunas/dihapusbukukan tetapi saldonya belum nol (penutupan sisa).
// Kredit bersaldo nol tidak pernah masuk, sehingga batch tidak menyentuhnya lagi.
//
// %s menerima filter buku instalasi/aktor (bookReadClause).
const listRestructureLossAmortizationCandidatesQuery = `
	SELECT l.id, l.loan_number
	FROM loans l
	WHERE l.restructure_loss_balance <> 0
	  AND (
	       (l.status IN ('DISBURSED', 'DEFAULTED') AND EXISTS (
	            SELECT 1 FROM loan_schedules s
	            WHERE s.loan_id = l.id
	              AND s.restructure_loss_amortized_at IS NULL
	              AND s.due_date <= $1))
	    OR l.status IN ('PAID_OFF', 'WRITTEN_OFF')
	  )%s
	ORDER BY l.loan_number`

func (r *LoanRepository) ListRestructureLossAmortizationCandidates(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.LoanRestructureLossAmortizationCandidate, error) {
	bookColumn := "(SELECT p.book FROM banking_products p WHERE p.id = l.product_id)"
	args := []any{asOf}
	filter := ""
	if clause, bargs := bookReadClause(bookColumn, actor, len(args)+1); clause != "" {
		filter = " AND " + clause
		args = append(args, bargs...)
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(listRestructureLossAmortizationCandidatesQuery, filter), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.LoanRestructureLossAmortizationCandidate
	for rows.Next() {
		var c domain.LoanRestructureLossAmortizationCandidate
		if err := rows.Scan(&c.LoanID, &c.LoanNumber); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// ApplyRestructureLossAmortizationTx mengurangi restructure_loss_balance dan menandai
// angsuran (bila scheduleID diisi) sudah diamortisasi, tetapi hanya bila jurnal dengan
// idempotencyKey tersebut belum ada. Polanya sama dengan AddScheduleProfitAccruedTx:
// UPDATE baris loans mengunci baris kredit sehingga dua batch paralel terserialkan, dan
// pengulangan pada tanggal bisnis yang sama tidak menggandakan pengurangan saldo.
// Syarat restructure_loss_balance >= amount menolak pengurangan yang akan membuat saldo
// negatif; pemanggil menghitung amount dari state tersimpan, bukan dari asumsi urutan.
func (r *LoanRepository) ApplyRestructureLossAmortizationTx(ctx context.Context, tx any, loanID uuid.UUID, scheduleID *uuid.UUID, amount decimal.Decimal, idempotencyKey string, amortizedAt time.Time) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("loan: transaksi tidak valid")
	}
	q := `UPDATE loans
		SET restructure_loss_balance = restructure_loss_balance - $1,
		    updated_at = NOW()
		WHERE id = $2
		  AND restructure_loss_balance >= $1
		  AND NOT EXISTS (SELECT 1 FROM journal_entries WHERE idempotency_key = $3)`
	res, err := sqlTx.ExecContext(ctx, q, amount, loanID, idempotencyKey)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	if scheduleID != nil {
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE loan_schedules
			SET restructure_loss_amortized_at = $1,
			    restructure_loss_amortized_amount = restructure_loss_amortized_amount + $2
			WHERE id = $3`, amortizedAt, amount, *scheduleID); err != nil {
			return false, err
		}
	}
	return true, nil
}

var _ domain.RestructureLossAmortizationRepository = (*LoanRepository)(nil)

var _ domain.LoanRepository = (*LoanRepository)(nil)
