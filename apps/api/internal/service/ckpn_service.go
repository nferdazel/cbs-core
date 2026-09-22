package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi kebijakan CKPN. Tidak ada angka asumsi yang ditanam di kode:
// PD dan LGD wajib diisi bank dari data historisnya (SEOJK No. 21/SEOJK.03/2024
// Bab XII butir 12.4.g.2.c dan 12.6–12.7).
const (
	cfgCKPNEnabled        = "ckpn.enabled"
	cfgCKPNShadowEnabled  = "ckpn.shadow_mode.enabled"
	cfgCKPNLGD            = "ckpn.lgd"
	cfgCKPNAsetBaikMaxDPD = "ckpn.aset_baik.max_dpd"
	cfgCKPNExpenseCOA     = "ckpn.coa.expense"
	cfgCKPNReserveCOA     = "ckpn.coa.reserve"
	// Kunci per unit syariah. KOSONG = ikut kunci global di atas, sehingga bank yang
	// belum mengisinya tidak mengalami perubahan perilaku. Diisi bank hanya bila akun
	// beban/cadangan CKPN pembiayaan berbeda dari akun konvensional.
	cfgCKPNExpenseCOASyariah = "ckpn.coa.expense.syariah"
	cfgCKPNReserveCOASyariah = "ckpn.coa.reserve.syariah"
)

// ckpnCollectibilityOrder adalah urutan golongan kolektibilitas untuk enumerasi PD
// (kunci ckpn.pd.<n>) dan penyusunan asumsi mode bayangan. Urutannya tetap agar pesan
// yang dibaca bank konsisten.
var ckpnCollectibilityOrder = []domain.Collectibility{
	domain.KolLancar, domain.KolDPK, domain.KolKurangLancar, domain.KolDiragukan, domain.KolMacet,
}

// Fallback kode COA CKPN bila bank belum mengisi pemetaannya. Kode ini di-seed
// migrasi 000042; 10900/50200 tetap milik PPAP dan tidak boleh dicampur agar
// perbandingan CKPN vs PPKA tidak saling mengurangi.
const (
	fallbackCKPNExpenseConventional = "50301" // Beban Kerugian Penurunan Nilai - Kredit
	fallbackCKPNExpenseSyariah      = "15901" // Beban Kerugian Penurunan Nilai - Pembiayaan
	fallbackCKPNReserveConventional = "10950" // CKPN - Kredit
	fallbackCKPNReserveSyariah      = "11950" // CKPN - Pembiayaan
)

// ckpnTxRunner membuka transaksi per kredit; sama polanya dengan ppapTxRunner agar
// pemrosesan dapat diuji tanpa database.
type ckpnTxRunner interface {
	Run(ctx context.Context, fn func(tx any) error) error
}

type sqlCKPNTxRunner struct{ db *sql.DB }

func (r sqlCKPNTxRunner) Run(ctx context.Context, fn func(tx any) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ckpnLoanLocker adalah irisan domain.LoanRepository yang dibutuhkan CKPN. Kredit
// yang menyentuh baris loans wajib dikunci lebih dulu (SELECT ... FOR UPDATE) sesuai
// disiplin LockLoanTx, tetapi service ini tidak perlu memegang seluruh antarmuka.
type ckpnLoanLocker interface {
	LockLoanTx(ctx context.Context, tx any, id uuid.UUID) (*domain.Loan, error)
}

type ckpnService struct {
	txRunner    ckpnTxRunner
	repo        domain.CKPNRepository
	productRepo domain.ProductRepository
	resolver    domain.AccountResolver
	poster      *ProductPoster
	posting     domain.PostingService
	config      domain.SystemConfigService
	locker      ckpnLoanLocker
	// runMarker menyimpan/membaca tanggal bisnis run PPAP terakhir. Boleh nil pada
	// lingkungan uji tanpa database; bila nil gerbang tanggal bisnis dilewati.
	runMarker domain.PPAPRunMarker
}

func NewCKPNService(
	db *sql.DB,
	repo domain.CKPNRepository,
	productRepo domain.ProductRepository,
	resolver domain.AccountResolver,
	poster *ProductPoster,
	posting domain.PostingService,
	config domain.SystemConfigService,
	locker ckpnLoanLocker,
	runMarker domain.PPAPRunMarker,
) domain.CKPNService {
	return &ckpnService{
		txRunner:    sqlCKPNTxRunner{db: db},
		repo:        repo,
		productRepo: productRepo,
		resolver:    resolver,
		poster:      poster,
		posting:     posting,
		config:      config,
		locker:      locker,
		runMarker:   runMarker,
	}
}

// Compare menghitung dan membandingkan CKPN vs PPKA tanpa memposting atau menulis state.
// actor dipakai membatasi kredit yang dibaca pada cabangnya; aktor lintas cabang membaca
// seluruh bank.
func (s *ckpnService) Compare(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.CKPNComparisonSummary, error) {
	return s.run(ctx, asOf, actor, false)
}

// Run menghitung CKPN, memposting selisihnya, lalu menyimpan target per kredit.
func (s *ckpnService) Run(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.CKPNComparisonSummary, error) {
	return s.run(ctx, asOf, actor, true)
}

func (s *ckpnService) run(ctx context.Context, asOf time.Time, actor domain.Actor, post bool) (domain.CKPNComparisonSummary, error) {
	asOf = asOf.UTC()

	policy := s.policy(ctx)

	// Mode bayangan tidak pernah memposting dan tidak pernah menulis state: ia hanya
	// menghitung lalu melaporkan. Bila ckpn.enabled true, saklar bayangan diabaikan
	// demi perilaku lama (mode resmi) yang berlaku.
	if policy.ShadowMode && !policy.Enabled {
		post = false
	}

	// ShadowMode yang dilaporkan hanya true bila bayangan benar-benar yang dipakai.
	// Saat kedua saklar menyala, mode resmi (ckpn.enabled) yang berlaku dan bayangan
	// diabaikan; melaporkan ShadowMode=true akan bertentangan dengan makna bidang itu
	// (lihat domain.CKPNComparisonSummary.ShadowMode: "Enabled harus false") dan lewat
	// HTTP dapat dibaca sebagai dua angka yang dihitung.
	shadowEffective := policy.ShadowMode && !policy.Enabled

	summary := domain.CKPNComparisonSummary{
		Enabled:    policy.Enabled,
		ShadowMode: shadowEffective,
		AsOf:       asOf,
		Preview:    !post,
	}

	// Kedua saklar mati berarti tidak ada query tambahan sama sekali: kredit tidak
	// dibaca dan tidak ada state/jurnal yang disentuh. Ini pola yang sama dengan
	// ppap.collateral.enabled.
	if !policy.Enabled && !policy.ShadowMode {
		return summary, nil
	}

	// required_ppap per kredit diisi jalur PPAP. Bila PPAP terakhir tidak berjalan pada
	// tanggal bisnis ini, angka itu milik run sebelumnya. Menolak di sini menutup jalur
	// pemanggilan manual di luar tutup hari, bukan hanya gerbang di dalam tutup hari.
	if err := s.requireFreshPPAP(ctx, asOf); err != nil {
		return summary, err
	}

	// Daftar kredit dibaca di luar transaksi hanya untuk menentukan kredit mana yang
	// perlu diproses, dan filter cabang diterapkan di query memakai actor. Angka yang
	// dipakai untuk posting selalu dihitung ulang dari baris yang dikunci (lihat apply):
	// snapshot di sini bisa basi.
	snapshots, err := s.repo.ListActiveLoans(ctx, actor)
	if err != nil {
		return summary, fmt.Errorf("mengambil daftar kredit untuk CKPN: %w", err)
	}
	summary.Total = len(snapshots)

	for _, snap := range snapshots {
		var itemSnap domain.CKPNLoanSnapshot
		var calc domain.CKPNCalculation

		if post {
			// Jalur tulis: kunci kredit lalu hitung ulang dari data segar di dalam
			// transaksi, bukan dari snapshot di atas.
			applied, err := s.apply(ctx, snap, policy, asOf, actor)
			if err != nil {
				summary.Failed++
				summary.Failures = append(summary.Failures, domain.CKPNRunFailure{
					LoanID:     snap.LoanID,
					LoanNumber: snap.LoanNumber,
					Error:      err.Error(),
				})
				continue
			}
			itemSnap, calc = applied.snapshot, applied.calc
		} else {
			// Jalur baca-saja: perbandingan memakai snapshot apa adanya. Tidak ada
			// kunci dan tidak ada penulisan.
			c, err := domain.CalculateCKPN(snap, policy)
			if err != nil {
				summary.Failed++
				summary.Failures = append(summary.Failures, domain.CKPNRunFailure{
					LoanID:     snap.LoanID,
					LoanNumber: snap.LoanNumber,
					Error:      err.Error(),
				})
				continue
			}
			itemSnap, calc = snap, c
		}

		// Perbandingan memakai target, bukan saldo GL: target PPKA per kredit sudah
		// tersimpan di loans.required_ppap hasil jalur PPAP, dan target CKPN dihitung
		// di sini. Saldo GL bersifat agregat portofolio sehingga tidak dapat dipakai
		// per kredit.
		item := ckpnCompare(itemSnap, calc)
		summary.Processed++
		summary.Items = append(summary.Items, item)
		// Aset baik (butir 12.3.a.2.a) menghasilkan target nol TANPA memakai PD/LGD.
		// Dihitung terpisah supaya laporan dapat menjelaskan TotalCKPN nol: nol karena
		// pengecualian yang sah, bukan karena model belum dijalankan.
		if calc.IsAsetBaik {
			summary.AsetBaikCount++
			summary.AsetBaikOutstanding = summary.AsetBaikOutstanding.Add(itemSnap.Outstanding)
		}
		summary.TotalCKPN = summary.TotalCKPN.Add(calc.Target)
		summary.TotalPPKA = summary.TotalPPKA.Add(itemSnap.RequiredPPAP)
		if diff := itemSnap.RequiredPPAP.Sub(calc.Target); diff.IsPositive() {
			summary.ModalIntiDeduction = summary.ModalIntiDeduction.Add(diff)
		}

		// Basis pembanding KEDUA "setara PPKA": aset baik TETAP dinilai EAD x PD x
		// LGD supaya basisnya sebanding dengan PPKA yang dihitung atas seluruh kredit.
		// Hanya untuk pelaporan: tidak ada jurnal dan required_ckpn tidak ditulis.
		// Kegagalan hanya-basis-2 (biasanya PD/LGD aset baik belum lengkap) dihitung
		// terpisah supaya tidak mengubah Processed/Failed basis pertama.
		setara, setaraErr := domain.CalculateCKPNSetaraPPKA(itemSnap, policy)
		if setaraErr != nil {
			summary.SetaraPPKAFailed++
		} else {
			summary.SetaraPPKAProcessed++
			summary.SetaraPPKATotalPPKA = summary.SetaraPPKATotalPPKA.Add(itemSnap.RequiredPPAP)
			summary.SetaraPPKATotalCKPN = summary.SetaraPPKATotalCKPN.Add(setara.Target)
			if diff := itemSnap.RequiredPPAP.Sub(setara.Target); diff.IsPositive() {
				summary.SetaraPPKAModalIntiDeduction = summary.SetaraPPKAModalIntiDeduction.Add(diff)
			}
		}
	}

	// Selisih agregat dan pihak yang lebih tinggi. PPKA diperlakukan sebagai lantai
	// pembanding PER KREDIT, karena pengurang modal inti dihitung per kredit. Karena
	// itu summary.ModalIntiDeduction (lihat loop di atas) menjumlahkan max(PPKA-CKPN,0)
	// tiap kredit, sedangkan summary.Difference di bawah hanya selisih AGREGAT
	// (TotalPPKA - TotalCKPN) yang bisa 0/negatif pada portofolio campuran walau ada
	// kelebihan per kredit. Pada mode bayangan keduanya hanya DILAPORKAN; pengurangan
	// modal inti belum dilakukan.
	summary.Difference = summary.TotalPPKA.Sub(summary.TotalCKPN)
	switch {
	case summary.Difference.IsPositive():
		summary.Higher = domain.CKPNLargerPPKA
	case summary.Difference.IsNegative():
		summary.Higher = domain.CKPNLargerCKPN
	default:
		summary.Higher = domain.CKPNLargerSame
	}

	// Selisih dan pihak lebih tinggi basis KEDUA (setara PPKA). Dihitung agregat atas
	// kredit yang dapat dihitung, serupa basis pertama; pengurang modal intinya tetap
	// Σ max(PPKA-CKPN,0) per kredit, bukan selisih agregat.
	summary.SetaraPPKADifference = summary.SetaraPPKATotalPPKA.Sub(summary.SetaraPPKATotalCKPN)
	switch {
	case summary.SetaraPPKADifference.IsPositive():
		summary.SetaraPPKAHigher = domain.CKPNLargerPPKA
	case summary.SetaraPPKADifference.IsNegative():
		summary.SetaraPPKAHigher = domain.CKPNLargerCKPN
	default:
		summary.SetaraPPKAHigher = domain.CKPNLargerSame
	}

	// Asumsi dan kekurangan parameter hanya relevan (dan hanya diisi) untuk mode
	// bayangan yang benar-benar dipakai (shadowEffective), supaya bentuk respons jalur
	// resmi tidak berubah. ParameterGaps tidak menebak nilai: ia menyebut kunci yang
	// harus diisi bank.
	if shadowEffective {
		summary.Assumptions = ckpnShadowAssumptions(policy)
		summary.ParameterGaps = ckpnPolicyGaps(policy)
		summary.BasisNote = ckpnDuaBasisNote(summary)
	}

	return summary, nil
}

// ckpnDuaBasisNote menjelaskan mengapa dua basis perbandingan bisa berbeda dan
// menegaskan keduanya belum menjadi kebijakan bank. Bila basis setara PPKA tidak
// dapat menghitung sebagian kredit (PD/LGD belum lengkap), itu disebutkan supaya
// totalnya yang lebih kecil tidak disalahbaca sebagai CKPN yang sudah dihitung penuh.
func ckpnDuaBasisNote(s domain.CKPNComparisonSummary) string {
	note := "DUA BASIS CKPN: (1) sesuai kebijakan — aset baik dikecualikan sehingga CKPN-nya nol (butir 12.3.a.2.a); (2) setara PPKA — aset baik tetap dinilai EAD x PD x LGD supaya basisnya sebanding dengan PPKA yang dihitung atas SELURUH kredit. Kedua angka berbeda karena perlakuan aset baik, bukan karena rumus berbeda. Keduanya BUKAN kebijakan bank yang berlaku: tidak ada jurnal yang ditulis, required_ckpn tidak tersimpan, dan pengurangan modal inti belum dilakukan."
	if s.SetaraPPKAFailed > 0 {
		note += fmt.Sprintf(" Basis setara PPKA belum dapat menghitung %d kredit (PD/LGD belum lengkap), sehingga totalnya hanya mencakup kredit yang dapat dihitung.", s.SetaraPPKAFailed)
	}
	return note
}

// ckpnShadowAssumptions merangkum asumsi yang BENAR-BENAR dipakai perhitungan mode
// bayangan, supaya pembaca tahu angkanya hitungan sementara beralasan, bukan kebijakan
// final. Isinya mengikuti rumus yang ada (CKPN = EAD x PD x LGD); tidak ada parameter
// yang dikarang di sini maupun di perhitungan.
func ckpnShadowAssumptions(policy domain.CKPNPolicy) []string {
	out := []string{
		"Rumus CKPN = EAD x PD x LGD (SAK EP via SEOJK No. 21/SEOJK.03/2024 Bab XII butir 12.9); angka BAYANGAN, belum menjadi kebijakan final bank.",
		"Satuan PD dan LGD adalah FRAKSI 0..1, BUKAN persen: 5% ditulis 0.05 (bukan 5), 45% ditulis 0.45 (bukan 45). Nilai di luar 0..1 ditolak dan dilaporkan sebagai kekurangan parameter.",
	}
	for _, c := range ckpnCollectibilityOrder {
		if v, ok := policy.PD[c]; ok {
			out = append(out, fmt.Sprintf("PD golongan %s (%s) = %s.", c.Label(), ckpnPDKey(c), v))
		}
	}
	if policy.LGDIsSet {
		out = append(out, fmt.Sprintf("LGD = %s (all account, berlaku untuk semua golongan).", policy.LGD))
	}
	out = append(out,
		fmt.Sprintf("Aset baik: tunggakan <= %d hari dan belum pernah direstrukturisasi dikecualikan dari CKPN (butir 12.3.a.2.a).", policy.AsetBaikMaxDPD),
		"EAD = sisa pokok dikurangi saldo kerugian restrukturisasi yang belum diamortisasi; dasar ini sama dengan PPKA.",
		"Perlakuan agunan: nilai realisasi agunan TIDAK dikurangkan langsung dari EAD di rumus ini; agunan diperhitungkan bank di dalam penetapan LGD (butir 12.7).",
		"PPKA sebagai lantai PER KREDIT: potensi pengurang modal inti = Σ max(PPKA_i - CKPN_i, 0), bukan selisih agregat total PPKA - total CKPN (butir 1.1.6). Kredit dengan CKPN lebih besar tidak mengurangi kelebihan kredit lain; pengurangannya belum dilakukan.",
	)
	return out
}

// ckpnPolicyGaps menyebutkan kunci parameter kebijakan yang belum diisi atau diisi
// tetapi tidak sah. Daftar ini dipakai mode bayangan untuk melaporkan bahwa CKPN tidak
// dapat dihitung beserta apa yang harus diisi, TANPA mengarang nilai dan TANPA
// menggagalkan tutup hari.
func ckpnPolicyGaps(policy domain.CKPNPolicy) []string {
	var gaps []string
	for _, c := range ckpnCollectibilityOrder {
		if _, invalid := policy.PDErrors[c]; invalid {
			gaps = append(gaps, fmt.Sprintf("%s (diisi tetapi tidak sah; satuan harus FRAKSI 0..1, mis. 5%% = 0.05)", ckpnPDKey(c)))
			continue
		}
		if _, ok := policy.PD[c]; !ok {
			gaps = append(gaps, fmt.Sprintf("%s (PD golongan %s belum diisi)", ckpnPDKey(c), c.Label()))
		}
	}
	switch {
	case policy.LGDError != nil:
		gaps = append(gaps, fmt.Sprintf("%s (diisi tetapi tidak sah; satuan harus FRAKSI 0..1, mis. 45%% = 0.45)", cfgCKPNLGD))
	case !policy.LGDIsSet:
		gaps = append(gaps, fmt.Sprintf("%s (LGD belum diisi)", cfgCKPNLGD))
	}
	return gaps
}

// requireFreshPPAP menolak perbandingan CKPN bila tidak ada run PPAP yang berhasil
// pada tanggal bisnis asOf. Tanpa gerbang ini, perbandingan tetap "berhasil" memakai
// required_ppap run sebelumnya dan laporan tampak sah padahal dasarnya kemarin.
//
// Penanda boleh nil pada lingkungan uji tanpa database; di produksi selalu terpasang.
func (s *ckpnService) requireFreshPPAP(ctx context.Context, asOf time.Time) error {
	if s.runMarker == nil {
		return nil
	}
	last, ok, err := s.runMarker.LastRunBusinessDate(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: belum ada run PPAP yang berhasil; jalankan PPAP untuk tanggal bisnis %s lebih dulu",
			domain.ErrCKPNStalePPAP, asOf.Format(layoutTanggalBisnis))
	}
	if !sameBusinessDay(last, asOf) {
		return fmt.Errorf("%w: tanggal bisnis PPAP terakhir %s, tanggal bisnis perbandingan %s; required_ppap masih milik run sebelumnya",
			domain.ErrCKPNStalePPAP, last.Format(layoutTanggalBisnis), asOf.Format(layoutTanggalBisnis))
	}
	return nil
}

// ckpnCompare menyusun perbandingan satu kredit. Difference = PPKA - CKPN; positif
// berarti PPKA lebih besar dan selisih itu menjadi pengurang modal inti (SEOJK
// No. 21/SEOJK.03/2024 butir 1.1.6).
func ckpnCompare(snap domain.CKPNLoanSnapshot, calc domain.CKPNCalculation) domain.CKPNComparisonItem {
	larger := domain.CKPNLargerSame
	switch {
	case snap.RequiredPPAP.GreaterThan(calc.Target):
		larger = domain.CKPNLargerPPKA
	case calc.Target.GreaterThan(snap.RequiredPPAP):
		larger = domain.CKPNLargerCKPN
	}
	return domain.CKPNComparisonItem{
		LoanID:         snap.LoanID,
		LoanNumber:     snap.LoanNumber,
		Outstanding:    snap.Outstanding,
		Collectibility: snap.Collectibility,
		IsAsetBaik:     calc.IsAsetBaik,
		CKPN:           calc.Target,
		PPKA:           snap.RequiredPPAP,
		Difference:     snap.RequiredPPAP.Sub(calc.Target),
		Larger:         larger,
	}
}

// ckpnApplied adalah hasil satu kredit yang diproses jalur tulis. snapshot dan calc
// berasal dari baris yang sudah dikunci, sehingga otoritatif untuk ringkasan.
type ckpnApplied struct {
	snapshot domain.CKPNLoanSnapshot
	calc     domain.CKPNCalculation
}

// apply mengunci kredit, menghitung ulang dari data segar di dalam transaksi,
// memposting selisih, lalu menyimpan target. Snapshot di luar transaksi hanya dipakai
// untuk menunjuk kredit; seluruh angka dihitung dari baris yang dikunci. Ini disiplin
// yang sama dengan LoanService.CancelDisbursementLoan/correctLoanAmountTx.
func (s *ckpnService) apply(ctx context.Context, snap domain.CKPNLoanSnapshot, policy domain.CKPNPolicy, asOf time.Time, actor domain.Actor) (ckpnApplied, error) {
	var out ckpnApplied
	err := s.txRunner.Run(ctx, func(tx any) error {
		fresh := snap
		if s.locker != nil {
			// Nilai hasil kunci tidak boleh dibuang: status, sisa pokok, dan
			// required_ckpn yang otoritatif hanya ada di sini.
			loan, err := s.locker.LockLoanTx(ctx, tx, snap.LoanID)
			if err != nil {
				return fmt.Errorf("mengunci kredit %s: %w", snap.LoanNumber, err)
			}
			fresh = ckpnSnapshotFromLoan(loan)
		}

		calc, err := domain.CalculateCKPN(fresh, policy)
		if err != nil {
			return err
		}
		out = ckpnApplied{snapshot: fresh, calc: calc}

		// Target sama dengan yang tersimpan berarti tidak ada yang perlu diubah.
		// Tidak ada posting dan tidak ada penulisan, termasuk ketika status segar
		// berubah menjadi tidak aktif tanpa cadangan tersisa.
		if calc.Adjustment.IsZero() {
			return nil
		}
		if err := s.postAdjustment(ctx, tx, fresh, calc, asOf, actor); err != nil {
			return err
		}
		return s.repo.UpdateRequiredCKPN(ctx, tx, fresh.LoanID, calc.Target)
	})
	if err != nil {
		return ckpnApplied{}, fmt.Errorf("kredit %s: %w", snap.LoanNumber, err)
	}
	return out, nil
}

// ckpnSnapshotFromLoan menyusun snapshot CKPN dari baris kredit yang sudah dikunci.
// required_ckpn dibaca dari baris yang sama agar selisih dihitung dari nilai yang
// benar-benar tersimpan, bukan dari nilai yang dibaca di luar transaksi.
func ckpnSnapshotFromLoan(l *domain.Loan) domain.CKPNLoanSnapshot {
	return domain.CKPNLoanSnapshot{
		LoanID:          l.ID,
		LoanNumber:      l.LoanNumber,
		ProductID:       l.ProductID,
		BranchCode:      l.BranchCode,
		Status:          l.Status,
		Outstanding:     l.OutstandingPrincipal,
		Collectibility:  domain.CollectibilityFromOJK(l.Collectibility),
		DPD:             l.DPD,
		IsRestructured:  l.IsRestructured,
		RequiredPPAP:    l.RequiredPPAP,
		RestructureLoss: l.RestructureLossBalance,
		RequiredCKPN:    l.RequiredCKPN,
	}
}

// postAdjustment memposting selisih CKPN. Positif = pembentukan (debit beban, kredit
// CKPN); negatif = pemulihan (debit CKPN, kredit beban), sesuai SEOJK 21/2024 butir
// 12.5 dan contoh jurnal butir 12.10.
//
// Jurnal diatribusikan ke cabang KREDIT (snap.BranchCode), bukan cabang aktor yang
// menjalankan. Memakai cabang aktor membuat buku cabang salah ketika run lintas
// cabang dijalankan dari kantor pusat.
func (s *ckpnService) postAdjustment(ctx context.Context, tx any, snap domain.CKPNLoanSnapshot, calc domain.CKPNCalculation, asOf time.Time, actor domain.Actor) error {
	event := domain.EventCKPNProvision
	if calc.Adjustment.IsNegative() {
		event = domain.EventCKPNReversal
	}

	meta := PostingMeta{
		TransactionType: domain.TxTypeAdjustment,
		Description:     fmt.Sprintf("CKPN kredit %s golongan %s (%s)", snap.LoanNumber, calc.Collectibility.Label(), calc.Adjustment.String()),
		IdempotencyKey:  fmt.Sprintf("CKPN-%s-%s-%s", snap.LoanNumber, asOf.Format("2006-01-02"), calc.Adjustment.String()),
		CreatedBy:       actor.DisplayName(),
		BranchCode:      snap.BranchCode,
	}
	amount := calc.Adjustment.Abs()

	// Jalur utama: pemetaan jurnal produk untuk peristiwa CKPN_*. Bila produk belum
	// punya pemetaan (mis. migrasi 000042 tidak dapat menanamnya karena nilai enum baru
	// tidak boleh dipakai dalam transaksi yang sama), jatuh ke COA konfigurasi.
	var product *domain.BankingProduct
	if snap.ProductID != nil {
		p, err := s.productRepo.GetByID(ctx, *snap.ProductID)
		if err == nil {
			product = p
		} else {
			slog.WarnContext(ctx, "produk kredit tidak terbaca; memakai fallback COA CKPN",
				"loan", snap.LoanNumber, "error", err)
		}
	}

	if product != nil {
		rules, err := s.productRepo.GetMapping(ctx, product.ID, event)
		if err != nil {
			// Galat pembacaan pemetaan BUKAN "produk belum dipetakan". Menjatuhkannya
			// ke COA fallback membuat produk yang sudah memetakan jurnal CKPN-nya tanpa
			// jejak terjurnal ke akun bawaan saat ada galat sesaat. Tidak ada pemetaan
			// (len==0) tetap memakai COA konfigurasi di bawah.
			return fmt.Errorf("membaca pemetaan jurnal CKPN produk %s: %w", product.Code, err)
		}
		if len(rules) > 0 {
			if _, err := s.poster.PostEventTx(ctx, tx, product, event, Amounts{
				Principal: amount,
				Total:     amount,
			}, meta); err != nil {
				return fmt.Errorf("jurnal CKPN produk %s: %w", product.Code, err)
			}
			return nil
		}
	}

	book := domain.BookConventional
	if product != nil && product.Book == domain.BookSyariah {
		book = domain.BookSyariah
	}
	return s.postAdjustmentFallback(ctx, tx, amount, calc.Adjustment.IsPositive(), book, meta)
}

// postAdjustmentFallback memposting CKPN memakai COA dari konfigurasi
// (ckpn.coa.expense/ckpn.coa.reserve) dengan fallback per buku produk.
func (s *ckpnService) postAdjustmentFallback(ctx context.Context, tx any, amount decimal.Decimal, provision bool, book domain.COABook, meta PostingMeta) error {
	expenseCOA := s.expenseCOA(ctx, book)
	reserveCOA := s.reserveCOA(ctx, book)

	expenseAcc, err := s.resolver.ResolveGLAccount(ctx, tx, expenseCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrCKPNExpenseNotFound, expenseCOA, err)
	}
	reserveAcc, err := s.resolver.ResolveGLAccount(ctx, tx, reserveCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrCKPNReserveNotFound, reserveCOA, err)
	}

	var debitAcc, creditAcc string
	if provision {
		debitAcc, creditAcc = expenseAcc, reserveAcc
	} else {
		debitAcc, creditAcc = reserveAcc, expenseAcc
	}

	lines := []domain.PostingLine{
		{AccountNumber: debitAcc, Direction: domain.DirectionDebit, Amount: amount, Description: meta.Description},
		{AccountNumber: creditAcc, Direction: domain.DirectionCredit, Amount: amount, Description: meta.Description},
	}
	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: meta.TransactionType,
		Description:     meta.Description,
		IdempotencyKey:  meta.IdempotencyKey,
		CreatedBy:       meta.CreatedBy,
		BranchCode:      meta.BranchCode,
		Lines:           lines,
	})
	return err
}

// policy membaca parameter kebijakan bank dari konfigurasi. Kunci PD/LGD yang belum
// diisi sengaja tidak diberi nilai default: kredit yang membutuhkannya akan gagal
// dengan ErrCKPNParameterMissing, bukan dihitung dengan nol. Nilai yang DIISI tetapi
// salah (format atau rentang) disimpan sebagai PDErrors/LGDError agar kegagalannya
// memakai ErrCKPNParameterInvalid, bukan disamakan dengan "belum diisi" atau
// dijatuhkan menjadi nol.
func (s *ckpnService) policy(ctx context.Context) domain.CKPNPolicy {
	p := domain.CKPNPolicy{
		PD:             make(map[domain.Collectibility]decimal.Decimal),
		PDErrors:       make(map[domain.Collectibility]error),
		AsetBaikMaxDPD: domain.CKPNAsetBaikMaxDPDDefault,
	}
	if s.config == nil {
		return p
	}

	p.Enabled = s.config.GetBool(ctx, cfgCKPNEnabled, false)
	p.ShadowMode = s.config.GetBool(ctx, cfgCKPNShadowEnabled, false)
	// Kedua saklar mati: berhenti sebelum membaca parameter apa pun, persis seperti
	// perilaku sebelum modul CKPN ada. Bila mode bayangan menyala, parameter tetap
	// dibaca supaya dapat dilaporkan apa yang belum diisi.
	if !p.Enabled && !p.ShadowMode {
		return p
	}

	if v := s.config.GetInt(ctx, cfgCKPNAsetBaikMaxDPD, domain.CKPNAsetBaikMaxDPDDefault); v >= 0 {
		p.AsetBaikMaxDPD = v
	}
	for _, c := range ckpnCollectibilityOrder {
		key := ckpnPDKey(c)
		v, set, err := configDecimal(ctx, s.config, key)
		if err != nil {
			p.PDErrors[c] = fmt.Errorf("%w: probability of default golongan %s (kunci %s): %v",
				domain.ErrCKPNParameterInvalid, c.Label(), key, err)
			continue
		}
		if !set {
			continue // belum diisi -> ErrCKPNParameterMissing saat dihitung
		}
		if v.LessThan(decimal.Zero) || v.GreaterThan(decimal.NewFromInt(1)) {
			p.PDErrors[c] = fmt.Errorf("%w: probability of default golongan %s (kunci %s) di luar rentang 0 s.d. 1 (satuan FRAKSI 0..1, mis. 5%% = 0.05; bukan persen): %s",
				domain.ErrCKPNParameterInvalid, c.Label(), key, v)
			continue
		}
		p.PD[c] = v
	}

	v, set, err := configDecimal(ctx, s.config, cfgCKPNLGD)
	switch {
	case err != nil:
		p.LGDError = fmt.Errorf("%w: loss given default (kunci %s): %v",
			domain.ErrCKPNParameterInvalid, cfgCKPNLGD, err)
	case !set:
		// belum diisi -> ErrCKPNParameterMissing saat dihitung
	case v.LessThan(decimal.Zero) || v.GreaterThan(decimal.NewFromInt(1)):
		p.LGDError = fmt.Errorf("%w: loss given default (kunci %s) di luar rentang 0 s.d. 1 (satuan FRAKSI 0..1, mis. 45%% = 0.45; bukan persen): %s",
			domain.ErrCKPNParameterInvalid, cfgCKPNLGD, v)
	default:
		p.LGD = v
		p.LGDIsSet = true
	}
	return p
}

func ckpnPDKey(c domain.Collectibility) string {
	return "ckpn.pd." + strconv.Itoa(int(c))
}

// configDecimal membaca nilai desimal dan membedakan tiga keadaan: kunci kosong/tidak
// ada (set=false, err=nil), nilai sah, dan nilai ada tetapi salah format (err != nil).
// GetDecimal tidak dapat membedakan ketiganya karena fallback dikembalikan pada semua
// keadaan. Nilai salah format TIDAK boleh menjadi nol diam-diam — pemanggil wajib
// menolaknya.
func configDecimal(ctx context.Context, config domain.SystemConfigService, key string) (decimal.Decimal, bool, error) {
	raw := strings.TrimSpace(config.GetString(ctx, key, ""))
	if raw == "" {
		return decimal.Zero, false, nil
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, true, fmt.Errorf("nilai %q bukan angka desimal yang sah", raw)
	}
	return d, true, nil
}

// resolveCKPNCOA memilih kode COA satu sisi jurnal CKPN. Urutannya: kunci unit
// syariah (hanya bila buku kredit SYARIAH dan nilainya tidak kosong), lalu kunci
// global. Bila kedua kunci kosong, fallback per buku dipertahankan — perilaku
// sebelum kunci syariah ada. Selama kunci syariah kosong, hasilnya karena itu
// identik dengan sebelumnya.
//
// Fungsi ini murni memilih kode; pemeriksaan apakah kode itu benar-benar ada di
// bagan akun dilakukan ResolveGLAccount di pemanggil (postAdjustmentFallback),
// supaya pemetaan yang salah ditolak, bukan terjurnal ke akun yang tidak ada.
func resolveCKPNCOA(ctx context.Context, config domain.SystemConfigService, book domain.COABook, syariahKey, globalKey, syariahFallback, conventionalFallback string) string {
	if book == domain.BookSyariah {
		if v := strings.TrimSpace(configStringOr(ctx, config, syariahKey, "")); v != "" {
			return v
		}
		return configStringOr(ctx, config, globalKey, syariahFallback)
	}
	return configStringOr(ctx, config, globalKey, conventionalFallback)
}

func (s *ckpnService) expenseCOA(ctx context.Context, book domain.COABook) string {
	return resolveCKPNCOA(ctx, s.config, book,
		cfgCKPNExpenseCOASyariah, cfgCKPNExpenseCOA,
		fallbackCKPNExpenseSyariah, fallbackCKPNExpenseConventional)
}

func (s *ckpnService) reserveCOA(ctx context.Context, book domain.COABook) string {
	return resolveCKPNCOA(ctx, s.config, book,
		cfgCKPNReserveCOASyariah, cfgCKPNReserveCOA,
		fallbackCKPNReserveSyariah, fallbackCKPNReserveConventional)
}

var _ domain.CKPNService = (*ckpnService)(nil)
