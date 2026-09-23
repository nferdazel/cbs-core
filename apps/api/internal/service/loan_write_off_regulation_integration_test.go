package service_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi syarat hapus buku menurut POJK 1/2024 Pasal 42-43 (BPR) dan
// POJK 24/2024 Pasal 50-51 (BPRS) terhadap PostgreSQL sungguhan:
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiHapusBuku -v
//
// Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola moneyflow_integration_test.go.
// Tiap syarat dibuktikan MENOLAK pada jalur service (bukan hanya konfigurasi), dan
// jalur yang sah dibuktikan tetap berhasil beserta jejak buktinya.

// writeOffIntegrationBranch menghasilkan kode cabang unik (maks 8 karakter) agar uji
// yang diulang pada database yang sama tidak menumpuk kredit di cabang yang sama.
func writeOffIntegrationBranch(prefix string) string {
	return (prefix + fmt.Sprintf("%d", time.Now().UnixNano()))[:8]
}

// writeOffIntegrationLoan menyiapkan satu kredit konvensional yang sudah dicairkan
// beserta aktornya.
func writeOffIntegrationLoan(t *testing.T, e *bookWriteEnv, prefix string) (*domain.Loan, domain.Actor) {
	t.Helper()
	branchCode := writeOffIntegrationBranch(prefix)
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Hapus Buku")
	actor := domain.Actor{
		UserID: e.actor.UserID, Username: "admin.ujihapusbuku",
		Role: domain.RoleAdmin, BranchCode: branchCode, Book: domain.BookConventional,
	}
	cust := e.newCustomer(t, "Nasabah Hapus Buku", fmt.Sprintf("woff-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(1_000_000), 6)
	return loan, actor
}

// woffSetState menyetel kualitas aset dan cadangan kredit langsung di database.
func woffSetState(t *testing.T, e *bookWriteEnv, loanID uuid.UUID, collectibility string, requiredPPAP int64) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET collectibility = $2, required_ppap = $3 WHERE id = $1`,
		loanID, collectibility, requiredPPAP); err != nil {
		t.Fatalf("menyetel state kredit: %v", err)
	}
}

// woffBypassApproval menaikkan ambang maker-checker agar uji fokus pada syarat, bukan
// pada alur persetujuan (alur itu sudah diuji terpisah).
func woffBypassApproval(t *testing.T, e *bookWriteEnv, action string) {
	t.Helper()
	woffSetThreshold(t, e, action, "999999999999")
}

// woffSnapshotConfig mencatat nilai system_config sebuah kunci SEBELUM uji
// mengubahnya dan mendaftarkan pemulihannya lewat t.Cleanup. Nilai yang dipulihkan
// adalah nilai lama yang benar-benar dibaca, bukan nilai yang diasumsikan, termasuk
// mengembalikan status "tidak ada baris" bila kunci itu memang belum ada. Ini
// menjaga isolasi: uji berikutnya dan pengulangan suite memulai dari keadaan yang
// sama, tidak mewarisi nilai yang disetel uji sebelumnya.
func woffSnapshotConfig(t *testing.T, e *bookWriteEnv, key string) {
	t.Helper()
	var oldValue string
	err := e.db.QueryRowContext(e.ctx, `SELECT value FROM system_config WHERE key = $1`, key).Scan(&oldValue)
	existed := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("membaca konfigurasi %s: %v", key, err)
	}
	t.Cleanup(func() {
		var restoreErr error
		if existed {
			_, restoreErr = e.db.ExecContext(e.ctx,
				`UPDATE system_config SET value = $2 WHERE key = $1`, key, oldValue)
		} else {
			_, restoreErr = e.db.ExecContext(e.ctx,
				`DELETE FROM system_config WHERE key = $1`, key)
		}
		if restoreErr != nil {
			t.Errorf("memulihkan konfigurasi %s: %v", key, restoreErr)
		}
		e.configSvc.Invalidate(key)
	})
}

// woffSetThreshold menyetel ambang maker-checker satu jenis aksi langsung di database
// uji, lalu membuang cache konfigurasinya agar service membaca nilai baru. Nilai lama
// ikut dipulihkan saat uji selesai (lihat woffSnapshotConfig) supaya kunci ini tidak
// tercemar ke uji lain.
func woffSetThreshold(t *testing.T, e *bookWriteEnv, action, value string) {
	t.Helper()
	key := "maker_checker." + action + ".threshold"
	woffSnapshotConfig(t, e, key)
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ($1, $2, 'uji integrasi hapus buku')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value); err != nil {
		t.Fatalf("menyetel ambang %s: %v", key, err)
	}
	e.configSvc.Invalidate(key)
}

// woffJournalCount menghitung jurnal hapus buku/recovery untuk satu nomor kredit.
func woffJournalCount(t *testing.T, e *bookWriteEnv, prefix, loanNumber string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT COUNT(*) FROM journal_entries WHERE idempotency_key LIKE $1 || $2 || '%'`,
		prefix, loanNumber).Scan(&n); err != nil {
		t.Fatalf("menghitung jurnal %s%s: %v", prefix, loanNumber, err)
	}
	return n
}

// Kredit yang belum Macet ditolak dengan galat spesifik, dan tidak ada jurnal/status
// yang berubah.
func TestIntegrasiHapusBukuMenolakKreditTidakMacet(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := writeOffIntegrationLoan(t, e, "WM")

	_, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loan.ID, Reason: "coba", CollectionEfforts: "somasi",
	}, actor)
	if !errors.Is(err, domain.ErrWriteOffNotMacet) {
		t.Fatalf("kredit tidak macet harus ditolak sebagai bukan macet, dapat: %v", err)
	}
	if n := woffJournalCount(t, e, "WOFF-", loan.LoanNumber); n != 0 {
		t.Fatalf("jurnal hapus buku %d, ingin 0", n)
	}
	if got := e.loanStatus(t, loan.ID); got != string(domain.LoanStatusDisbursed) {
		t.Fatalf("status berubah menjadi %q meski ditolak", got)
	}
}

// Kredit Macet yang cadangannya belum 100% tetap ditolak.
func TestIntegrasiHapusBukuMenolakCadanganBelumPenuh(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := writeOffIntegrationLoan(t, e, "WR")
	woffSetState(t, e, loan.ID, string(domain.CollectibilityKol5), 0)

	_, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loan.ID, Reason: "coba", CollectionEfforts: "somasi",
	}, actor)
	if !errors.Is(err, domain.ErrWriteOffReserveIncomplete) {
		t.Fatalf("cadangan belum 100%% harus ditolak, dapat: %v", err)
	}
	if n := woffJournalCount(t, e, "WOFF-", loan.LoanNumber); n != 0 {
		t.Fatalf("jurnal hapus buku %d, ingin 0", n)
	}
}

// Permintaan hapus buku sebagian (nominal < seluruh sisa pokok) ditolak.
func TestIntegrasiHapusBukuMenolakPermintaanSebagian(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := writeOffIntegrationLoan(t, e, "WP")
	woffSetState(t, e, loan.ID, string(domain.CollectibilityKol5), 1_000_000)

	_, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loan.ID, Reason: "coba", CollectionEfforts: "somasi",
		Amount: decimal.NewFromInt(400_000),
	}, actor)
	if !errors.Is(err, domain.ErrWriteOffPartial) {
		t.Fatalf("hapus buku sebagian harus ditolak, dapat: %v", err)
	}
	if n := woffJournalCount(t, e, "WOFF-", loan.LoanNumber); n != 0 {
		t.Fatalf("jurnal hapus buku %d, ingin 0", n)
	}
}

// Jalur yang sah berhasil: seluruh pokok dilepas dari neraca, status WRITTEN_OFF,
// dan audit menyimpan bukti syarat saat keputusan.
func TestIntegrasiHapusBukuSahBerhasilDanRekamJejak(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := writeOffIntegrationLoan(t, e, "WS")
	woffSetState(t, e, loan.ID, string(domain.CollectibilityKol5), 1_000_000)
	woffBypassApproval(t, e, "loan_write_off")

	out, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loan.ID, Reason: "debitor pailit",
		CollectionEfforts: "somasi 3x + kunjungan lapangan + surat peringatan",
	}, actor)
	if err != nil {
		t.Fatalf("hapus buku sah: %v", err)
	}
	if out == nil || out.Status != domain.LoanStatusWrittenOff {
		t.Fatalf("status kredit %+v, ingin WRITTEN_OFF", out)
	}
	if got := e.loanStatus(t, loan.ID); got != string(domain.LoanStatusWrittenOff) {
		t.Fatalf("status tersimpan %q, ingin WRITTEN_OFF", got)
	}
	if n := woffJournalCount(t, e, "WOFF-", loan.LoanNumber); n != 1 {
		t.Fatalf("jurnal hapus buku %d, ingin 1", n)
	}

	var collectibility, efforts string
	var requiredPPAP decimal.Decimal
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT changes->>'collectibility', changes->>'collection_efforts', (changes->>'required_ppap')::numeric
		FROM audit_logs
		WHERE action = 'WRITE_OFF_LOAN' AND resource_id = $1`, loan.ID.String()).
		Scan(&collectibility, &efforts, &requiredPPAP); err != nil {
		t.Fatalf("membaca audit hapus buku: %v", err)
	}
	if collectibility != string(domain.CollectibilityKol5) {
		t.Fatalf("audit kualitas aset %q, ingin 5_MACET", collectibility)
	}
	if !requiredPPAP.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("audit cadangan %s, ingin 1000000", requiredPPAP)
	}
	if efforts == "" {
		t.Fatal("audit tidak menyimpan upaya penagihan")
	}
}

// Hapus buku tidak menghapus hak tagih: recovery setelahnya tetap bisa dicatat.
func TestIntegrasiHapusBukuSahRecoveryTetapBisa(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := writeOffIntegrationLoan(t, e, "WX")
	woffSetState(t, e, loan.ID, string(domain.CollectibilityKol5), 1_000_000)
	woffBypassApproval(t, e, "loan_write_off")
	woffBypassApproval(t, e, "loan_recovery")

	if _, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loan.ID, Reason: "debitor pailit", CollectionEfforts: "somasi",
	}, actor); err != nil {
		t.Fatalf("hapus buku: %v", err)
	}
	if _, err := e.loanSvc.RecoverWrittenOffLoan(e.ctx, domain.RecoverWrittenOffLoanInput{
		LoanID: loan.ID, RecoveryAmount: decimal.NewFromInt(200_000), IdempotencyKey: "kuitansi-1",
	}, actor); err != nil {
		t.Fatalf("recovery setelah hapus buku harus tetap bisa: %v", err)
	}
	if n := woffJournalCount(t, e, "RECOV-", loan.LoanNumber); n != 1 {
		t.Fatalf("jurnal recovery %d, ingin 1", n)
	}
	var auditCount int
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE action = 'RECOVER_WRITTEN_OFF_LOAN' AND resource_id = $1`, loan.ID.String()).
		Scan(&auditCount); err != nil {
		t.Fatalf("membaca audit recovery: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit recovery %d, ingin 1", auditCount)
	}
}

// woffSetCKPNEnabled menyalakan/mematikan ckpn.enabled pada database uji, lalu
// membuang cache konfigurasinya agar loanService membaca nilai baru. Nilai lama
// dipulihkan saat uji selesai (lihat woffSnapshotConfig), bukan diasumsikan "false",
// supaya saklar CKPN tidak tertinggal menyala/mati setelah suite.
func woffSetCKPNEnabled(t *testing.T, e *bookWriteEnv, enabled bool) {
	t.Helper()
	woffSnapshotConfig(t, e, "ckpn.enabled")
	value := "false"
	if enabled {
		value = "true"
	}
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ('ckpn.enabled', $1, 'uji integrasi hapus buku CKPN')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, value); err != nil {
		t.Fatalf("menyetel ckpn.enabled: %v", err)
	}
	e.configSvc.Invalidate("ckpn.enabled")
}

// woffSetRequiredCKPN menyetel target CKPN kredit (loans.required_ckpn) langsung di
// database uji — meniru hasil jalur pembentukan CKPN tanpa harus menjalankannya.
func woffSetRequiredCKPN(t *testing.T, e *bookWriteEnv, loanID uuid.UUID, amount int64) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE loans SET required_ckpn = $2 WHERE id = $1`, loanID, amount); err != nil {
		t.Fatalf("menyetel required_ckpn: %v", err)
	}
}

// woffGLAccountID mencari id akun GL internal satu kode COA, memakai pola yang sama
// dengan LedgerRepository.ResolveGLAccount (akun INTERNAL_GL pertama menurut nomor).
func woffGLAccountID(t *testing.T, e *bookWriteEnv, coaCode string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT a.id FROM accounts a
		JOIN chart_of_accounts coa ON coa.id = a.coa_id
		WHERE coa.code = $1 AND a.account_type = 'INTERNAL_GL'
		ORDER BY a.account_number LIMIT 1`, coaCode).Scan(&id); err != nil {
		t.Fatalf("mencari akun GL COA %s: %v", coaCode, err)
	}
	return id
}

func woffSetGLBalance(t *testing.T, e *bookWriteEnv, accountID uuid.UUID, amount decimal.Decimal) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE accounts SET balance = $2, available_balance = $2 WHERE id = $1`, accountID, amount); err != nil {
		t.Fatalf("menyetel saldo akun GL: %v", err)
	}
}

func woffGLBalance(t *testing.T, e *bookWriteEnv, accountID uuid.UUID) decimal.Decimal {
	t.Helper()
	var balance decimal.Decimal
	if err := e.db.QueryRowContext(e.ctx, `SELECT balance FROM accounts WHERE id = $1`, accountID).Scan(&balance); err != nil {
		t.Fatalf("membaca saldo akun GL: %v", err)
	}
	return balance
}

// woffJournalAmount membaca jumlah satu baris jurnal hapus buku (kunci WOFF-<nomor>)
// pada kode COA dan arah tertentu. Nol berarti baris itu tidak ada.
func woffJournalAmount(t *testing.T, e *bookWriteEnv, loanNumber, coaCode, direction string) decimal.Decimal {
	t.Helper()
	var amount decimal.Decimal
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT COALESCE(SUM(jl.amount), 0)
		FROM journal_entries je
		JOIN journal_lines jl ON jl.journal_entry_id = je.id
		JOIN accounts a ON a.id = jl.account_id
		JOIN chart_of_accounts coa ON coa.id = a.coa_id
		WHERE je.idempotency_key = 'WOFF-' || $1 AND coa.code = $2 AND jl.direction = $3`,
		loanNumber, coaCode, direction).Scan(&amount); err != nil {
		t.Fatalf("membaca nominal jurnal %s/%s: %v", coaCode, direction, err)
	}
	return amount
}

// woffDebitAccounts mengembalikan himpunan kode COA yang didebit jurnal hapus buku
// (kunci WOFF-) untuk satu nomor kredit.
func woffDebitAccounts(t *testing.T, e *bookWriteEnv, loanNumber string) map[string]bool {
	t.Helper()
	rows, err := e.db.QueryContext(e.ctx, `
		SELECT coa.code
		FROM journal_entries je
		JOIN journal_lines jl ON jl.journal_entry_id = je.id
		JOIN accounts a ON a.id = jl.account_id
		JOIN chart_of_accounts coa ON coa.id = a.coa_id
		WHERE je.idempotency_key = 'WOFF-' || $1 AND jl.direction = 'DEBIT'`, loanNumber)
	if err != nil {
		t.Fatalf("membaca baris jurnal hapus buku: %v", err)
	}
	defer func() { _ = rows.Close() }()
	accounts := map[string]bool{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			t.Fatalf("memindai akun jurnal: %v", err)
		}
		accounts[code] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("baris jurnal hapus buku: %v", err)
	}
	return accounts
}

// Hapus buku harus melepas akun cadangan yang benar menurut saklar CKPN: saat CKPN
// aktif, cadangan yang dibukukan adalah CKPN 10950; saat CKPN mati, perilaku lama
// (PPAP 10900) tidak boleh berubah. Keduanya dibuktikan pada database sungguhan.
func TestIntegrasiHapusBukuMelepasCadanganSesuaiSaklarCKPN(t *testing.T) {
	e := newBookWriteEnv(t)

	// CKPN mati: pemetaan produk melepas PPAP 10900, tidak menyentuh CKPN 10950.
	woffSetCKPNEnabled(t, e, false)
	loanOff, actorOff := writeOffIntegrationLoan(t, e, "WC")
	woffSetState(t, e, loanOff.ID, string(domain.CollectibilityKol5), 1_000_000)
	woffBypassApproval(t, e, "loan_write_off")
	if _, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loanOff.ID, Reason: "debitor pailit", CollectionEfforts: "somasi 3x",
	}, actorOff); err != nil {
		t.Fatalf("hapus buku dengan CKPN mati: %v", err)
	}
	off := woffDebitAccounts(t, e, loanOff.LoanNumber)
	if !off["10900"] {
		t.Fatalf("CKPN mati harus melepas PPAP 10900, dapat %v", off)
	}
	if off["10950"] {
		t.Fatalf("CKPN mati tidak boleh menyentuh akun CKPN 10950: %v", off)
	}
	// Perilaku CKPN mati harus PERSIS seperti sebelumnya: PPAP dilepas sebesar pokok
	// penuh dan akun beban CKPN tidak muncul.
	if got := woffJournalAmount(t, e, loanOff.LoanNumber, "10900", "DEBIT"); !got.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("CKPN mati melepas PPAP %s, ingin tetap 1000000", got)
	}
	if got := woffJournalAmount(t, e, loanOff.LoanNumber, "50301", "DEBIT"); !got.IsZero() {
		t.Fatalf("CKPN mati tidak boleh memakai beban CKPN 50301, dapat %s", got)
	}

	// CKPN aktif: kaki debit cadangan diarahkan ke CKPN 10950, bukan PPAP 10900.
	woffSetCKPNEnabled(t, e, true)
	loanOn, actorOn := writeOffIntegrationLoan(t, e, "WK")
	woffSetState(t, e, loanOn.ID, string(domain.CollectibilityKol5), 1_000_000)
	// Cadangan CKPN kredit diisi penuh menutup pokok, sehingga seluruhnya dilepas.
	woffSetRequiredCKPN(t, e, loanOn.ID, 1_000_000)
	woffBypassApproval(t, e, "loan_write_off")
	if _, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loanOn.ID, Reason: "debitor pailit", CollectionEfforts: "somasi 3x",
	}, actorOn); err != nil {
		t.Fatalf("hapus buku dengan CKPN aktif: %v", err)
	}
	on := woffDebitAccounts(t, e, loanOn.LoanNumber)
	if !on["10950"] {
		t.Fatalf("CKPN aktif harus melepas CKPN 10950, dapat %v", on)
	}
	if on["10900"] {
		t.Fatalf("CKPN aktif tidak boleh melepas PPAP 10900: %v", on)
	}
}

// Saat CKPN aktif, JUMLAH yang dilepas dari akun CKPN harus sebesar cadangan yang
// terkait kredit itu (loans.required_ckpn), bukan pokok penuh. Selisih pokok yang belum
// dicadangkan diakui sebagai beban kerugian penurunan nilai, dan pokok tetap dilepas
// penuh. Diuji pada database sungguhan dengan angka jurnal nyata, sekaligus
// membuktikan saldo akun CKPN tidak menjadi negatif maupun menyisakan saldo.
func TestIntegrasiHapusBukuCKPNMelepasSejumlahCadangan(t *testing.T) {
	e := newBookWriteEnv(t)
	woffSetCKPNEnabled(t, e, true)

	loan, actor := writeOffIntegrationLoan(t, e, "WQ")
	// Pokok 1.000.000; cadangan CKPN kredit hanya 600.000 (PD x LGD < 100%), sedangkan
	// syarat PPAP 100% tetap dipenuhi lewat required_ppap.
	woffSetState(t, e, loan.ID, string(domain.CollectibilityKol5), 1_000_000)
	woffSetRequiredCKPN(t, e, loan.ID, 600_000)

	// Danai akun CKPN tepat sebesar cadangan yang akan dilepas. Pelepasan sebesar
	// pokok (1.000.000) akan membuat saldo -400.000, sehingga cacat lama terdeteksi.
	ckpnAcc := woffGLAccountID(t, e, "10950")
	woffSetGLBalance(t, e, ckpnAcc, decimal.NewFromInt(600_000))

	woffBypassApproval(t, e, "loan_write_off")
	if _, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loan.ID, Reason: "debitor pailit", CollectionEfforts: "somasi 3x",
	}, actor); err != nil {
		t.Fatalf("hapus buku CKPN aktif: %v", err)
	}

	// Angka jurnal nyata dan hitungan manual: 600.000 (CKPN) + 400.000 (beban)
	// = 1.000.000 (pokok). Bila jumlah pelepasan masih memakai pokok, uji ini gagal.
	ckpnReleased := woffJournalAmount(t, e, loan.LoanNumber, "10950", "DEBIT")
	loss := woffJournalAmount(t, e, loan.LoanNumber, "50301", "DEBIT")
	principal := woffJournalAmount(t, e, loan.LoanNumber, "10301", "CREDIT")
	if !ckpnReleased.Equal(decimal.NewFromInt(600_000)) {
		t.Fatalf("pelepasan CKPN %s, ingin 600000 (= required_ckpn)", ckpnReleased)
	}
	if !loss.Equal(decimal.NewFromInt(400_000)) {
		t.Fatalf("beban selisih %s, ingin 400000 (pokok - required_ckpn)", loss)
	}
	if !principal.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("pokok dilepas %s, ingin tetap 1000000", principal)
	}
	if !ckpnReleased.Add(loss).Equal(principal) {
		t.Fatalf("hitungan manual tidak seimbang: %s + %s != %s", ckpnReleased, loss, principal)
	}
	// Saldo akun CKPN pasca hapus buku harus NOL: tidak negatif (pelepasan berlebih)
	// dan tidak menyisakan cadangan.
	if got := woffGLBalance(t, e, ckpnAcc); got.IsNegative() || !got.IsZero() {
		t.Fatalf("saldo akun CKPN %s, ingin tepat 0 (tidak negatif, tidak menyisakan)", got)
	}
}

// Isolasi uji: helper penyetel konfigurasi harus memulihkan nilai SEBELUMNYA saat uji
// selesai, bukan meninggalkan nilai uji atau mengembalikan nilai yang diasumsikan.
// t.Cleanup subtest berjalan sebelum subtest berikutnya, sehingga pemulihan dapat
// diperiksa tepat setelah blok yang mengubah. Tanpa snapshot+restore, kunci ini
// tertinggal tercemar dan regresi bisa tersamarkan pada pengulangan suite.
func TestIntegrasiKonfigurasiHelperDipulihkan(t *testing.T) {
	e := newBookWriteEnv(t)

	baca := func(key string) string {
		t.Helper()
		var value string
		if err := e.db.QueryRowContext(e.ctx, `SELECT value FROM system_config WHERE key = $1`, key).Scan(&value); err != nil {
			t.Fatalf("membaca konfigurasi %s: %v", key, err)
		}
		return value
	}

	const ambangKey = "maker_checker.loan_recovery.threshold"
	ambangAsli := baca(ambangKey)
	t.Run("ambang maker-checker", func(t *testing.T) {
		woffSetThreshold(t, e, "loan_recovery", "123456789")
		if got := baca(ambangKey); got != "123456789" {
			t.Fatalf("helper tidak menyetel ambang: %q", got)
		}
	})
	if got := baca(ambangKey); got != ambangAsli {
		t.Fatalf("ambang tidak dipulihkan: %q, ingin %q", got, ambangAsli)
	}

	const ckpnKey = "ckpn.enabled"
	ckpnAsli := baca(ckpnKey)
	t.Run("saklar CKPN", func(t *testing.T) {
		woffSetCKPNEnabled(t, e, ckpnAsli != "true")
		if got := baca(ckpnKey); got == ckpnAsli {
			t.Fatalf("helper tidak mengubah saklar CKPN: %q", got)
		}
	})
	if got := baca(ckpnKey); got != ckpnAsli {
		t.Fatalf("saklar CKPN tidak dipulihkan: %q, ingin %q", got, ckpnAsli)
	}
}
