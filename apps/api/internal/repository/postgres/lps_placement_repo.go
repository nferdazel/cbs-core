package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// LPSPlacementRepository membaca dan menulis penempatan pada bank lain yang menjadi
// dasar pengurang PPKA Pasal 23 POJK No. 1 Tahun 2024 sekaligus tempat asesmen CKPN per
// penempatan (Form 05.00 kolom XII/XXI) disimpan. Tanpa saklar ckpn.pabl.enabled, jalur
// tulis tidak pernah dipanggil service.
type LPSPlacementRepository struct {
	db *sql.DB
}

func NewLPSPlacementRepository(db *sql.DB) *LPSPlacementRepository {
	return &LPSPlacementRepository{db: db}
}

// listLPSPlacementsSelect mengambil penempatan yang tanggal penilaiannya tidak melewati
// asOf. Filter cabang ditambahkan pemanggil memakai branchReadClause; urutan deterministik
// diperlukan agar penolakan plafon dan ringkasan tidak berubah antar pemanggilan.
//
// Cast ::text wajib untuk kolom varchar/enum agar aman dipindai ke string. Kolom CKPN
// dibaca apa adanya: ckpn_assessed_at NULL berarti penempatan belum pernah diasesmen.
const listLPSPlacementsSelect = `
	SELECT
		p.id,
		p.coa_code,
		p.counterparty_bank,
		p.placement_type::text,
		p.outstanding,
		p.lps_guaranteed,
		p.collectibility::text,
		p.as_of,
		COALESCE(b.code, ''),
		p.ckpn_method,
		p.ckpn_significant,
		p.ckpn_objective_evidence,
		p.required_ckpn,
		COALESCE(p.ckpn_individual_target, 0),
		p.ckpn_assessed_at,
		p.start_date,
		p.maturity_date,
		COALESCE(p.interest_rate_annual, 0),
		COALESCE(p.ojk_kabupaten_code, ''),
		COALESCE(p.ojk_hubungan_bank_code, ''),
		p.blocked_amount,
		COALESCE(p.ojk_alasan_diblokir_code, ''),
		p.accrued_interest_receivable,
		p.accrued_interest_pending,
		COALESCE(p.counterparty_cif, ''),
		COALESCE(p.ojk_klasifikasi_aset_code, ''),
		p.customer_id
	FROM lps_placements p
	LEFT JOIN branches b ON b.id = p.branch_id`

// scanLPSPlacement memindai satu baris listLPSPlacementsSelect. CKPN hanya terisi bila
// ckpn_assessed_at tidak NULL; baris yang belum diasesmen dibiarkan nil sehingga form
// dapat menyatakan kolom XII/XXI belum tersedia.
func scanLPSPlacement(row rowScanner) (*domain.LPSPlacement, error) {
	var (
		p                                         domain.LPSPlacement
		placementType, collectibility, ckpnMethod string
		ckpnSignificant, ckpnObjectiveEvidence    bool
		requiredCKPN, individualTarget            decimal.Decimal
		assessedAt                                sql.NullTime
		startDate, maturityDate                   sql.NullTime
		blockedAmount                             decimal.NullDecimal
		accruedInterest, accruedInterestPending   decimal.NullDecimal
		customerID                                uuid.NullUUID
	)
	if err := row.Scan(
		&p.ID, &p.COACode, &p.CounterpartyBank, &placementType,
		&p.Outstanding, &p.LPSGuaranteed, &collectibility, &p.AsOf, &p.BranchCode,
		&ckpnMethod, &ckpnSignificant, &ckpnObjectiveEvidence,
		&requiredCKPN, &individualTarget, &assessedAt,
		&startDate, &maturityDate, &p.InterestRateAnnual, &p.OJKKabupatenCode,
		&p.OJKHubunganBankCode, &blockedAmount, &p.OJKAlasanDiblokirCode,
		&accruedInterest, &accruedInterestPending, &p.CounterpartyCIF,
		&p.OJKKlasifikasiAsetCode,
		&customerID,
	); err != nil {
		return nil, err
	}
	if customerID.Valid {
		v := customerID.UUID
		p.CustomerID = &v
	}
	if blockedAmount.Valid {
		v := blockedAmount.Decimal
		p.BlockedAmount = &v
	}
	if accruedInterest.Valid {
		v := accruedInterest.Decimal
		p.AccruedInterestReceivable = &v
	}
	if accruedInterestPending.Valid {
		v := accruedInterestPending.Decimal
		p.AccruedInterestPending = &v
	}
	p.PlacementType = domain.LPSPlacementType(placementType)
	p.Collectibility = domain.LPSPlacementCollectibility(collectibility)
	if startDate.Valid {
		t := startDate.Time
		p.StartDate = &t
	}
	if maturityDate.Valid {
		t := maturityDate.Time
		p.MaturityDate = &t
	}
	if assessedAt.Valid {
		t := assessedAt.Time
		p.CKPN = &domain.LPSPlacementCKPN{
			Method:            domain.PABLCKPNMethod(ckpnMethod),
			Significant:       ckpnSignificant,
			ObjectiveEvidence: ckpnObjectiveEvidence,
			RequiredCKPN:      requiredCKPN,
			IndividualTarget:  individualTarget,
			AssessedAt:        &t,
		}
	}
	return &p, nil
}

func (r *LPSPlacementRepository) ListPlacements(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.LPSPlacement, error) {
	args := []any{}
	branchFilter := ""
	if clause, branchArgs := branchReadClause("p.branch_id", actor); clause != "" {
		branchFilter = " AND " + clause
		args = append(args, branchArgs...)
	}
	// Placeholder asOf dibuat setelah argumen filter cabang agar penomoran posisi benar
	// saat klausa cabang memakai $1 (branchReadClause) maupun saat tidak ada filter.
	asOfPlaceholder := fmt.Sprintf("$%d", len(args)+1)
	args = append(args, asOf)

	query := listLPSPlacementsSelect +
		" WHERE p.as_of <= " + asOfPlaceholder + branchFilter +
		" ORDER BY p.counterparty_bank, p.id"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.LPSPlacement
	for rows.Next() {
		p, err := scanLPSPlacement(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	return list, rows.Err()
}

// LockPlacementTx membaca satu penempatan dengan SELECT ... FOR UPDATE di dalam
// transaksi pemanggil. Kunci menyerialkan asesmen CKPN dengan penulisan penempatan lain,
// sehingga asesmen tidak memakai angka outstanding yang sudah basi.
func (r *LPSPlacementRepository) LockPlacementTx(ctx context.Context, tx any, id uuid.UUID) (*domain.LPSPlacement, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("lps placement: transaksi tidak valid")
	}
	row := sqlTx.QueryRowContext(ctx,
		listLPSPlacementsSelect+" WHERE p.id = $1 FOR UPDATE OF p", id)
	p, err := scanLPSPlacement(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrLPSPlacementNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetPlacementByID membaca satu penempatan tanpa kunci, dipakai jalur baca/edit sandi
// OJK. Baris yang tidak ada ditandai ErrLPSPlacementNotFound.
func (r *LPSPlacementRepository) GetPlacementByID(ctx context.Context, id uuid.UUID) (*domain.LPSPlacement, error) {
	row := r.db.QueryRowContext(ctx, listLPSPlacementsSelect+" WHERE p.id = $1", id)
	p, err := scanLPSPlacement(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrLPSPlacementNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateOJKPlacementCodesTx menyimpan sandi OJK satu penempatan di dalam transaksi
// pemanggil. Hanya kolom yang dikirim (pointer non-nil) yang masuk SET; nilai kosong
// disimpan sebagai NULL. Nominal diurai memakai aturan domain yang sama dengan
// validasinya.
func (r *LPSPlacementRepository) UpdateOJKPlacementCodesTx(ctx context.Context, tx any, id uuid.UUID, input domain.UpdateOJKPlacementCodesInput) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("lps placement: transaksi tidak valid")
	}

	args := []any{id}
	set := make([]string, 0, 8)
	appendCode := func(column string, value *string) {
		if value == nil {
			return
		}
		trimmed := strings.TrimSpace(*value)
		args = append(args, sql.NullString{String: trimmed, Valid: trimmed != ""})
		set = append(set, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	appendAmount := func(column, field string, value *string) error {
		if value == nil {
			return nil
		}
		nd, err := domain.ParseOJKAmount(field, *value)
		if err != nil {
			return err
		}
		args = append(args, nd)
		set = append(set, fmt.Sprintf("%s = $%d", column, len(args)))
		return nil
	}

	appendCode("ojk_kabupaten_code", input.OJKKabupatenCode)
	appendCode("ojk_hubungan_bank_code", input.OJKHubunganBankCode)
	appendCode("ojk_alasan_diblokir_code", input.OJKAlasanDiblokirCode)
	appendCode("counterparty_cif", input.CounterpartyCIF)
	appendCode("ojk_klasifikasi_aset_code", input.OJKKlasifikasiAsetCode)
	amountColumns := []struct {
		column string
		field  string
		value  *string
	}{
		{"blocked_amount", domain.OJKBlockedAmountField, input.BlockedAmount},
		{"accrued_interest_receivable", domain.OJKAccruedInterestReceivableField, input.AccruedInterestReceivable},
		{"accrued_interest_pending", domain.OJKAccruedInterestPendingField, input.AccruedInterestPending},
	}
	for _, item := range amountColumns {
		if err := appendAmount(item.column, item.field, item.value); err != nil {
			return err
		}
	}
	if len(set) == 0 {
		return nil
	}
	set = append(set, "updated_at = NOW()")

	query := "UPDATE lps_placements SET " + strings.Join(set, ", ") + " WHERE id = $1"
	if _, err := sqlTx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("menyimpan sandi OJK penempatan: %w", err)
	}
	return nil
}

// RecordCKPNTx menyimpan asesmen CKPN satu penempatan: memperbarui kolom CKPN pada
// lps_placements dan menulis jejak keputusan ke pabl_ckpn_assessments. Jurnal TIDAK
// diposting di sini (belum ada keputusan bank tentang pencatatannya).
func (r *LPSPlacementRepository) RecordCKPNTx(ctx context.Context, tx any, placementID uuid.UUID, input domain.PABLCKPNInput, carrying decimal.Decimal, actorName string) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("lps placement: transaksi tidak valid")
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE lps_placements
		SET ckpn_method = $2,
		    ckpn_significant = $3,
		    ckpn_objective_evidence = $4,
		    required_ckpn = $5,
		    ckpn_individual_target = $6,
		    ckpn_assessed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`,
		placementID, string(input.Method), input.Significant, input.ObjectiveEvidence,
		input.RequiredCKPN, input.IndividualTarget)
	if err != nil {
		return fmt.Errorf("menyimpan CKPN penempatan: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrLPSPlacementNotFound
	}
	if _, err := sqlTx.ExecContext(ctx, `
		INSERT INTO pabl_ckpn_assessments
			(placement_id, as_of, method, significant, objective_evidence,
			 carrying, target, decided_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		placementID, input.AsOf.UTC(), string(input.Method), input.Significant,
		input.ObjectiveEvidence, carrying, input.RequiredCKPN, actorName); err != nil {
		return fmt.Errorf("menulis jejak asesmen CKPN penempatan: %w", err)
	}
	return nil
}

// RecordCollectiveCKPNTx menyimpan hasil mesin kolektif satu penempatan. Berbeda dari
// RecordCKPNTx, ckpn_method TIDAK diubah: metode yang tersimpan (dibaca dari baris
// terkunci) yang dipakai untuk jejak asesmen, sedangkan carrying adalah outstanding
// saat itu. Jurnal TIDAK diposting. Baris sudah dikunci pemanggil lewat LockPlacementTx
// pada transaksi yang sama, sehingga bacaan ini konsisten.
func (r *LPSPlacementRepository) RecordCollectiveCKPNTx(ctx context.Context, tx any, placementID uuid.UUID, asOf time.Time, target decimal.Decimal, basis []byte, actorName string) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("lps placement: transaksi tidak valid")
	}
	var (
		method      string
		outstanding decimal.Decimal
	)
	err := sqlTx.QueryRowContext(ctx, `
		SELECT ckpn_method, outstanding
		FROM lps_placements
		WHERE id = $1
		FOR UPDATE`, placementID).Scan(&method, &outstanding)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrLPSPlacementNotFound
	}
	if err != nil {
		return fmt.Errorf("membaca penempatan untuk CKPN kolektif: %w", err)
	}

	res, err := sqlTx.ExecContext(ctx, `
		UPDATE lps_placements
		SET required_ckpn = $2,
		    ckpn_assessed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`,
		placementID, target)
	if err != nil {
		return fmt.Errorf("menyimpan CKPN kolektif penempatan: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrLPSPlacementNotFound
	}
	if _, err := sqlTx.ExecContext(ctx, `
		INSERT INTO pabl_ckpn_assessments
			(placement_id, as_of, method, significant, objective_evidence,
			 carrying, target, basis, decided_by)
		VALUES ($1, $2, $3, FALSE, FALSE, $4, $5, $6, $7)`,
		placementID, asOf.UTC(), method, outstanding, target, basis, actorName); err != nil {
		return fmt.Errorf("menulis jejak CKPN kolektif penempatan: %w", err)
	}
	return nil
}

var _ domain.LPSPlacementRepository = (*LPSPlacementRepository)(nil)
