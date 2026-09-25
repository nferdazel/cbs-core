package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// lpsConfigStub membaca system_config dari peta; GetBool benar-benar membaca nilai agar
// jalur saklar hidup dan mati dapat diuji.
type lpsConfigStub struct {
	domain.SystemConfigService
	values map[string]string
}

func (c *lpsConfigStub) GetString(_ context.Context, key, fallback string) string {
	if v, ok := c.values[key]; ok {
		return v
	}
	return fallback
}

func (c *lpsConfigStub) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	if v, ok := c.values[key]; ok {
		if parsed, err := decimal.NewFromString(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func (c *lpsConfigStub) GetBool(_ context.Context, key string, fallback bool) bool {
	if v, ok := c.values[key]; ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "ya":
			return true
		case "false", "0", "tidak":
			return false
		}
	}
	return fallback
}

func (c *lpsConfigStub) GetInt(_ context.Context, _ string, fallback int) int { return fallback }
func (c *lpsConfigStub) Invalidate(string)                                    {}

// lpsRepoStub merekam apakah pembacaan terjadi dan aktor apa yang diteruskan, sehingga
// bukti saklar mati tidak menjalankan query dapat diperiksa.
type lpsRepoStub struct {
	placements []domain.LPSPlacement
	listCalled bool
	gotActor   domain.Actor
	gotAsOf    time.Time
	err        error

	// Jejak asesmen CKPN (AssessCKPN).
	lockCalled   bool
	recordCalled bool
	gotLockID    uuid.UUID
	gotRecordID  uuid.UUID
	gotInput     domain.PABLCKPNInput
	gotCarrying  decimal.Decimal
	gotActorName string
	lockErr      error
	recordErr    error
	lockOverride *domain.LPSPlacement

	// Jejak run kolektif CKPN (RunCollectiveCKPN).
	collectiveCalled    bool
	gotCollectiveID     uuid.UUID
	gotCollectiveTarget decimal.Decimal
	gotCollectiveBasis  []byte
	gotCollectiveActor  string
	collectiveErr       error
}

func (r *lpsRepoStub) ListPlacements(_ context.Context, asOf time.Time, actor domain.Actor) ([]domain.LPSPlacement, error) {
	r.listCalled = true
	r.gotActor = actor
	r.gotAsOf = asOf
	if r.err != nil {
		return nil, r.err
	}
	return r.placements, nil
}

func (r *lpsRepoStub) LockPlacementTx(_ context.Context, _ any, id uuid.UUID) (*domain.LPSPlacement, error) {
	r.lockCalled = true
	r.gotLockID = id
	if r.lockErr != nil {
		return nil, r.lockErr
	}
	if r.lockOverride != nil {
		return r.lockOverride, nil
	}
	for i := range r.placements {
		if r.placements[i].ID == id {
			p := r.placements[i]
			return &p, nil
		}
	}
	return nil, domain.ErrLPSPlacementNotFound
}

func (r *lpsRepoStub) RecordCKPNTx(_ context.Context, _ any, placementID uuid.UUID, input domain.PABLCKPNInput, carrying decimal.Decimal, actorName string) error {
	r.recordCalled = true
	r.gotRecordID = placementID
	r.gotInput = input
	r.gotCarrying = carrying
	r.gotActorName = actorName
	return r.recordErr
}

func (r *lpsRepoStub) RecordCollectiveCKPNTx(_ context.Context, _ any, placementID uuid.UUID, _ time.Time, target decimal.Decimal, basis []byte, actorName string) error {
	r.collectiveCalled = true
	r.gotCollectiveID = placementID
	r.gotCollectiveTarget = target
	r.gotCollectiveBasis = basis
	r.gotCollectiveActor = actorName
	return r.collectiveErr
}

func lpsServicePlacement(outstanding, guaranteed int64, k domain.LPSPlacementCollectibility) domain.LPSPlacement {
	return domain.LPSPlacement{
		ID:               uuid.New(),
		COACode:          "10200",
		CounterpartyBank: "Bank Y",
		PlacementType:    domain.LPSPlacementDeposito,
		Outstanding:      decimal.NewFromInt(outstanding),
		LPSGuaranteed:    decimal.NewFromInt(guaranteed),
		Collectibility:   k,
		AsOf:             time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
	}
}

// Saklar mati berarti tidak ada query sama sekali dan tidak ada angka yang dihitung;
// perilakunya harus identik dengan sebelum modul ini ada.
func TestLPSPlacementCalculate_SaklarMatiTidakMembaca(t *testing.T) {
	repo := &lpsRepoStub{placements: []domain.LPSPlacement{lpsServicePlacement(1_000_000, 500_000, domain.LPSLancar)}}
	svc := &lpsPlacementService{
		repo:   repo,
		config: &lpsConfigStub{values: map[string]string{domain.LPSPlacementEnabledKey: "false"}},
	}

	summary, err := svc.Calculate(context.Background(), time.Now().UTC(), domain.Actor{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if summary.Enabled {
		t.Fatal("ringkasan mengaku aktif padahal saklar mati")
	}
	if repo.listCalled {
		t.Fatal("penempatan tidak boleh dibaca selama saklar mati")
	}
	if summary.Total != 0 || len(summary.Items) != 0 {
		t.Fatalf("ringkasan tidak boleh berisi data: total=%d items=%d", summary.Total, len(summary.Items))
	}
}

// Tanpa config, saklar tidak dapat dibaca; perlakukan sebagai mati agar pemanggil lama
// tidak berubah perilakunya.
func TestLPSPlacementCalculate_TanpaConfigTetapMati(t *testing.T) {
	repo := &lpsRepoStub{}
	svc := &lpsPlacementService{repo: repo}

	summary, err := svc.Calculate(context.Background(), time.Now().UTC(), domain.Actor{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if summary.Enabled || repo.listCalled {
		t.Fatal("tanpa config modul harus tetap mati tanpa query")
	}
}

// Saklar hidup: pengurang Pasal 23 diterapkan pada PPKA umum (Lancar) dan PPKA khusus
// (Kurang Lancar). Angka mengikuti Penjelasan Pasal 23.
func TestLPSPlacementCalculate_SaklarHidupMenghitungUmumDanKhusus(t *testing.T) {
	// Dua bank lawan berbeda agar plafon LPS per bank (2 miliar) tidak saling menjumlah.
	lancar := lpsServicePlacement(10_000_000_000, 2_000_000_000, domain.LPSLancar)
	lancar.CounterpartyBank = "Bank Y"
	khusus := lpsServicePlacement(4_000_000_000, 1_000_000_000, domain.LPSKurangLancar)
	khusus.CounterpartyBank = "Bank Z"
	repo := &lpsRepoStub{placements: []domain.LPSPlacement{lancar, khusus}}
	svc := &lpsPlacementService{
		repo:   repo,
		config: &lpsConfigStub{values: map[string]string{domain.LPSPlacementEnabledKey: "true"}},
	}

	summary, err := svc.Calculate(context.Background(), time.Now().UTC(), domain.Actor{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if !repo.listCalled {
		t.Fatal("penempatan harus dibaca saat saklar hidup")
	}
	if !summary.Enabled || summary.Total != 2 || summary.Processed != 2 {
		t.Fatalf("ringkasan enabled=%v total=%d processed=%d", summary.Enabled, summary.Total, summary.Processed)
	}
	if !summary.GuaranteeCap.Equal(domain.LPSGuaranteeCapDefault()) {
		t.Fatalf("plafon %s, mau bawaan %s", summary.GuaranteeCap, domain.LPSGuaranteeCapDefault())
	}
	if !summary.TotalOutstanding.Equal(decimal.NewFromInt(14_000_000_000)) {
		t.Fatalf("total penempatan %s, mau 14000000000", summary.TotalOutstanding)
	}
	if !summary.TotalDeduction.Equal(decimal.NewFromInt(3_000_000_000)) {
		t.Fatalf("total pengurang %s, mau 3000000000", summary.TotalDeduction)
	}
	// Umum: 0,5% x 8 miliar = 40 juta. Khusus: 10% x 3 miliar = 300 juta.
	if !summary.TotalPPKA.Equal(decimal.NewFromInt(340_000_000)) {
		t.Fatalf("total PPKA %s, mau 340000000", summary.TotalPPKA)
	}
	for _, item := range summary.Items {
		switch item.Collectibility {
		case domain.KolLancar:
			if !item.GeneralPPKA.Equal(decimal.NewFromInt(40_000_000)) || !item.SpecificPPKA.IsZero() {
				t.Fatalf("item Lancar umum=%s khusus=%s", item.GeneralPPKA, item.SpecificPPKA)
			}
		case domain.KolKurangLancar:
			if !item.SpecificPPKA.Equal(decimal.NewFromInt(300_000_000)) || !item.GeneralPPKA.IsZero() {
				t.Fatalf("item Kurang Lancar umum=%s khusus=%s", item.GeneralPPKA, item.SpecificPPKA)
			}
		}
	}
}

// Penempatan yang TIDAK dijamin LPS tidak berkurang sedikit pun.
func TestLPSPlacementCalculate_PenempatanTanpaJaminanTidakMengurangi(t *testing.T) {
	repo := &lpsRepoStub{placements: []domain.LPSPlacement{
		lpsServicePlacement(1_000_000, 0, domain.LPSLancar),
	}}
	svc := &lpsPlacementService{
		repo:   repo,
		config: &lpsConfigStub{values: map[string]string{domain.LPSPlacementEnabledKey: "true"}},
	}

	summary, err := svc.Calculate(context.Background(), time.Now().UTC(), domain.Actor{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("item %d, mau 1", len(summary.Items))
	}
	item := summary.Items[0]
	if !item.Deduction.IsZero() || !item.Base.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("pengurang %s dasar %s, mau 0 dan penuh", item.Deduction, item.Base)
	}
	if !item.PPKA.Equal(decimal.NewFromInt(5_000)) {
		t.Fatalf("PPKA %s, mau 5000", item.PPKA)
	}
}

// Parameter plafon yang salah format atau di luar rentang DITOLAK dengan pesan jelas,
// bukan dijatuhkan menjadi nol.
func TestLPSPlacementCalculate_MenolakParameterCapTidakSah(t *testing.T) {
	kasus := []struct {
		nama  string
		value string
		pesan string
	}{
		{"salah format koma", "0,5", "bukan angka desimal"},
		{"huruf", "dua miliar", "bukan angka desimal"},
		{"nol", "0", "lebih besar dari nol"},
		{"negatif", "-1", "lebih besar dari nol"},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			repo := &lpsRepoStub{placements: []domain.LPSPlacement{lpsServicePlacement(1_000_000, 500_000, domain.LPSLancar)}}
			svc := &lpsPlacementService{
				repo: repo,
				config: &lpsConfigStub{values: map[string]string{
					domain.LPSPlacementEnabledKey: "true",
					domain.LPSGuaranteeCapKey:     k.value,
				}},
			}
			_, err := svc.Calculate(context.Background(), time.Now().UTC(), domain.Actor{})
			if !errors.Is(err, domain.ErrLPSParameterInvalid) {
				t.Fatalf("error %v, mau ErrLPSParameterInvalid", err)
			}
			if !strings.Contains(err.Error(), k.pesan) {
				t.Fatalf("pesan %q tidak memuat %q", err, k.pesan)
			}
			if repo.listCalled {
				t.Fatal("parameter tidak sah harus ditolak sebelum membaca penempatan")
			}
		})
	}
}

// Plafon diuji atas jumlah jaminan per bank lawan; kelebihan ditolak, bukan dipotong
// diam-diam sehingga angka PPKA tampak sah.
func TestLPSPlacementCalculate_MenolakJaminanMelebihiPlafon(t *testing.T) {
	bank := "Bank Y"
	p1 := lpsServicePlacement(3_000_000_000, 2_000_000_000, domain.LPSLancar)
	p2 := lpsServicePlacement(3_000_000_000, 2_000_000_000, domain.LPSLancar)
	p1.CounterpartyBank, p2.CounterpartyBank = bank, bank

	repo := &lpsRepoStub{placements: []domain.LPSPlacement{p1, p2}}
	svc := &lpsPlacementService{
		repo: repo,
		config: &lpsConfigStub{values: map[string]string{
			domain.LPSPlacementEnabledKey: "true",
			domain.LPSGuaranteeCapKey:     "3000000000",
		}},
	}
	_, err := svc.Calculate(context.Background(), time.Now().UTC(), domain.Actor{})
	if !errors.Is(err, domain.ErrLPSGuaranteeOverCap) {
		t.Fatalf("error %v, mau ErrLPSGuaranteeOverCap", err)
	}
	if !strings.Contains(err.Error(), bank) {
		t.Fatalf("pesan %q tidak menyebut bank lawan %q", err, bank)
	}
}

// Aktor diteruskan apa adanya ke repositori agar cakupan cabang ditegakkan pada operasi baca.
func TestLPSPlacementCalculate_MeneruskanAktorKeRepo(t *testing.T) {
	aktor := domain.Actor{Username: "ao.cabang", Role: domain.RoleAO, BranchCode: "007"}
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	repo := &lpsRepoStub{}
	svc := &lpsPlacementService{
		repo:   repo,
		config: &lpsConfigStub{values: map[string]string{domain.LPSPlacementEnabledKey: "true"}},
	}

	if _, err := svc.Calculate(context.Background(), asOf, aktor); err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if repo.gotActor != aktor {
		t.Fatalf("aktor diteruskan %+v, mau %+v", repo.gotActor, aktor)
	}
	if !repo.gotAsOf.Equal(asOf) {
		t.Fatalf("asOf diteruskan %s, mau %s", repo.gotAsOf, asOf)
	}
}

// Galat pembacaan tidak boleh menjadi laporan kosong yang tampak sah.
func TestLPSPlacementCalculate_GalatRepoDikembalikan(t *testing.T) {
	repo := &lpsRepoStub{err: errors.New("koneksi putus")}
	svc := &lpsPlacementService{
		repo:   repo,
		config: &lpsConfigStub{values: map[string]string{domain.LPSPlacementEnabledKey: "true"}},
	}

	_, err := svc.Calculate(context.Background(), time.Now().UTC(), domain.Actor{})
	if err == nil || !strings.Contains(err.Error(), "koneksi putus") {
		t.Fatalf("error %v, mau galat repo diteruskan", err)
	}
}

// Saklar hidup tanpa repositori adalah kesalahan perakitan, bukan alasan mengembalikan nol.
func TestLPSPlacementCalculate_RepoTidakTersediaSaatHidup(t *testing.T) {
	svc := &lpsPlacementService{
		config: &lpsConfigStub{values: map[string]string{domain.LPSPlacementEnabledKey: "true"}},
	}
	if _, err := svc.Calculate(context.Background(), time.Now().UTC(), domain.Actor{}); err == nil {
		t.Fatal("repositori tidak tersedia harus menghasilkan error")
	}
}

// Satu baris tidak sah tidak menggagalkan laporan, tetapi dicatat sebagai kegagalan dan
// tidak ikut menambah total.
func TestLPSPlacementCalculate_BarisTidakSahDicatatGagal(t *testing.T) {
	valid := lpsServicePlacement(1_000_000, 0, domain.LPSLancar)
	invalid := lpsServicePlacement(1_000_000, 0, domain.LPSLancar)
	invalid.CounterpartyBank = ""

	repo := &lpsRepoStub{placements: []domain.LPSPlacement{valid, invalid}}
	svc := &lpsPlacementService{
		repo:   repo,
		config: &lpsConfigStub{values: map[string]string{domain.LPSPlacementEnabledKey: "true"}},
	}

	summary, err := svc.Calculate(context.Background(), time.Now().UTC(), domain.Actor{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if summary.Total != 2 || summary.Processed != 1 || summary.Failed != 1 {
		t.Fatalf("total=%d processed=%d failed=%d, mau 2/1/1", summary.Total, summary.Processed, summary.Failed)
	}
	if len(summary.Failures) != 1 || !strings.Contains(summary.Failures[0].Error, domain.ErrLPSPlacementInvalid.Error()) {
		t.Fatalf("kegagalan %+v, mau memuat %q", summary.Failures, domain.ErrLPSPlacementInvalid)
	}
}

// lpsAssessmentInput adalah masukan asesmen yang sah; BranchCode/Outstanding diisi
// pemanggil lewat penempatan stub.
func lpsAssessmentInput() domain.PABLCKPNInput {
	return domain.PABLCKPNInput{
		Method:            domain.PABLCKPNMethodIndividualDCF,
		Significant:       true,
		ObjectiveEvidence: true,
		RequiredCKPN:      decimal.NewFromInt(250_000),
		IndividualTarget:  decimal.NewFromInt(250_000),
		AsOf:              time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
	}
}

// Saklar mati menolak asesmen dengan ErrCKPNPABLDisabled dan TIDAK menyentuh repositori:
// permintaan tidak boleh diam-diam sukses tanpa menyimpan apa pun.
func TestLPSPlacementAssessCKPN_SaklarMatiMenolak(t *testing.T) {
	repo := &lpsRepoStub{}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: map[string]string{domain.CKPNPABLEnabledKey: "false"}},
		txRunner: stubTxRunner{},
	}
	_, err := svc.AssessCKPN(context.Background(), uuid.New(), lpsAssessmentInput(), domain.Actor{})
	if !errors.Is(err, domain.ErrCKPNPABLDisabled) {
		t.Fatalf("error %v, mau ErrCKPNPABLDisabled", err)
	}
	if repo.lockCalled || repo.recordCalled {
		t.Fatal("saklar mati tidak boleh menyentuh repositori")
	}
}

// Masukan tidak sah ditolak sebelum kunci baris dibuka.
func TestLPSPlacementAssessCKPN_MasukanTidakSahDitolak(t *testing.T) {
	repo := &lpsRepoStub{}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: map[string]string{domain.CKPNPABLEnabledKey: "true"}},
		txRunner: stubTxRunner{},
	}
	input := lpsAssessmentInput()
	input.Method = "TIDAK_DIKENAL"
	_, err := svc.AssessCKPN(context.Background(), uuid.New(), input, domain.Actor{})
	if !errors.Is(err, domain.ErrCKPNPABLInputInvalid) {
		t.Fatalf("error %v, mau ErrCKPNPABLInputInvalid", err)
	}
	if repo.lockCalled {
		t.Fatal("masukan tidak sah harus ditolak sebelum mengunci baris")
	}
}

// Penempatan di luar cakupan cabang aktor ditolak ErrCrossBranchAccess, bukan disimpan.
func TestLPSPlacementAssessCKPN_LintasCabangDitolak(t *testing.T) {
	placement := lpsServicePlacement(1_000_000, 0, domain.LPSLancar)
	placement.BranchCode = "001"
	repo := &lpsRepoStub{placements: []domain.LPSPlacement{placement}}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: map[string]string{domain.CKPNPABLEnabledKey: "true"}},
		txRunner: stubTxRunner{},
	}
	actor := domain.Actor{Username: "ao.002", Role: domain.RoleAO, BranchCode: "002"}
	_, err := svc.AssessCKPN(context.Background(), placement.ID, lpsAssessmentInput(), actor)
	if !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("error %v, mau ErrCrossBranchAccess", err)
	}
	if repo.recordCalled {
		t.Fatal("penempatan lintas cabang tidak boleh disimpan")
	}
}

// Asesmen sah menyimpan carrying = outstanding, meneruskan aktor, dan mengembalikan
// penempatan dengan CKPN terisi.
func TestLPSPlacementAssessCKPN_MenyimpanCarryingDanCKPN(t *testing.T) {
	placement := lpsServicePlacement(1_000_000, 0, domain.LPSLancar)
	placement.BranchCode = "001"
	repo := &lpsRepoStub{placements: []domain.LPSPlacement{placement}}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: map[string]string{domain.CKPNPABLEnabledKey: "true"}},
		txRunner: stubTxRunner{},
	}
	actor := domain.Actor{Username: "ao.001", Role: domain.RoleAO, BranchCode: "001"}

	got, err := svc.AssessCKPN(context.Background(), placement.ID, lpsAssessmentInput(), actor)
	if err != nil {
		t.Fatalf("AssessCKPN: %v", err)
	}
	if !repo.lockCalled || !repo.recordCalled {
		t.Fatal("asesmen harus mengunci lalu menulis")
	}
	if !repo.gotCarrying.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("carrying %s, mau 1000000", repo.gotCarrying)
	}
	if repo.gotActorName != "ao.001" {
		t.Fatalf("aktor %q, mau ao.001", repo.gotActorName)
	}
	if got.CKPN == nil || got.CKPN.Method != domain.PABLCKPNMethodIndividualDCF {
		t.Fatalf("CKPN hasil %+v, mau metode INDIVIDUAL_DCF", got.CKPN)
	}
	if !got.CKPN.RequiredCKPN.Equal(decimal.NewFromInt(250_000)) {
		t.Fatalf("required CKPN %s, mau 250000", got.CKPN.RequiredCKPN)
	}
}

// Penempatan tidak ditemukan dikembalikan apa adanya agar handler memetakan 404.
func TestLPSPlacementAssessCKPN_TidakDitemukan(t *testing.T) {
	repo := &lpsRepoStub{}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: map[string]string{domain.CKPNPABLEnabledKey: "true"}},
		txRunner: stubTxRunner{},
	}
	_, err := svc.AssessCKPN(context.Background(), uuid.New(), lpsAssessmentInput(), domain.Actor{})
	if !errors.Is(err, domain.ErrLPSPlacementNotFound) {
		t.Fatalf("error %v, mau ErrLPSPlacementNotFound", err)
	}
}

// Transaction runner yang tidak dikonfigurasi adalah kesalahan perakitan saat saklar hidup.
func TestLPSPlacementAssessCKPN_TanpaTxRunnerDitolak(t *testing.T) {
	cfg := &lpsConfigStub{values: map[string]string{domain.CKPNPABLEnabledKey: "true"}}
	svc := &lpsPlacementService{repo: &lpsRepoStub{}, config: cfg}
	if _, err := svc.AssessCKPN(context.Background(), uuid.New(), lpsAssessmentInput(), domain.Actor{}); err == nil {
		t.Fatal("transaction runner yang tidak dikonfigurasi harus menghasilkan error")
	}
}

// pablParamsConfig adalah konfigurasi sah mesin kolektif: PD gol 1/3/5 = 1%/10%/50%,
// LGD 50%, aset baik tidak membentuk CKPN.
func pablParamsConfig() map[string]string {
	return map[string]string{
		domain.CKPNPABLEnabledKey:            "true",
		domain.CKPNPABLPDFracGol1Key:         "0.01",
		domain.CKPNPABLPDFracGol3Key:         "0.10",
		domain.CKPNPABLPDFracGol5Key:         "0.50",
		domain.CKPNPABLLGDFracKey:            "0.5",
		domain.CKPNPABLAsetBaikBentukCKPNKey: "false",
	}
}

// lpsPlacementWithMethod melengkapi penempatan stub dengan metode CKPN tersimpan.
func lpsPlacementWithMethod(p domain.LPSPlacement, method domain.PABLCKPNMethod) domain.LPSPlacement {
	p.CKPN = &domain.LPSPlacementCKPN{Method: method}
	return p
}

// Saklar mati menolak run kolektif dan TIDAK membaca penempatan.
func TestRunCollectiveCKPN_SaklarMatiMenolak(t *testing.T) {
	repo := &lpsRepoStub{}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: map[string]string{domain.CKPNPABLEnabledKey: "false"}},
		txRunner: stubTxRunner{},
	}
	_, err := svc.RunCollectiveCKPN(context.Background(), time.Now().UTC(), domain.Actor{})
	if !errors.Is(err, domain.ErrCKPNPABLDisabled) {
		t.Fatalf("error %v, mau ErrCKPNPABLDisabled", err)
	}
	if repo.listCalled {
		t.Fatal("saklar mati tidak boleh membaca penempatan")
	}
}

// Parameter kosong/tidak sah menolak seluruh run sebelum penempatan dibaca.
func TestRunCollectiveCKPN_ParameterKosongMenolak(t *testing.T) {
	repo := &lpsRepoStub{}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: map[string]string{domain.CKPNPABLEnabledKey: "true"}},
		txRunner: stubTxRunner{},
	}
	_, err := svc.RunCollectiveCKPN(context.Background(), time.Now().UTC(), domain.Actor{})
	if !errors.Is(err, domain.ErrPABLCKPNParameterInvalid) {
		t.Fatalf("error %v, mau ErrPABLCKPNParameterInvalid", err)
	}
	if repo.listCalled {
		t.Fatal("parameter tidak sah harus ditolak sebelum membaca penempatan")
	}
}

// EXCLUDED_ASET_BAIK disimpan dengan target nol (bukan dilewati): cadangan lama dilepas.
func TestRunCollectiveCKPN_ExcludedDisimpanTargetNol(t *testing.T) {
	p := lpsPlacementWithMethod(lpsServicePlacement(1_000_000, 0, domain.LPSLancar), domain.PABLCKPNMethodExcludedAsetBaik)
	p.BranchCode = "001"
	repo := &lpsRepoStub{placements: []domain.LPSPlacement{p}}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: pablParamsConfig()},
		txRunner: stubTxRunner{},
	}
	actor := domain.Actor{Username: "ao.001", Role: domain.RoleAO, BranchCode: "001"}

	summary, err := svc.RunCollectiveCKPN(context.Background(), time.Now().UTC(), actor)
	if err != nil {
		t.Fatalf("RunCollectiveCKPN: %v", err)
	}
	if summary.Total != 1 || summary.Processed != 1 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("total=%d processed=%d skipped=%d failed=%d, mau 1/1/0/0",
			summary.Total, summary.Processed, summary.Skipped, summary.Failed)
	}
	if !repo.collectiveCalled || !repo.gotCollectiveTarget.IsZero() {
		t.Fatalf("target tersimpan %s, mau 0 dan tersimpan", repo.gotCollectiveTarget)
	}
	if !summary.TotalTarget.IsZero() {
		t.Fatalf("total target %s, mau 0", summary.TotalTarget)
	}
}

// Asesmen INDIVIDUAL_* dipertahankan: dilewati tanpa menulis apa pun.
func TestRunCollectiveCKPN_IndividualDilewati(t *testing.T) {
	p := lpsPlacementWithMethod(lpsServicePlacement(1_000_000, 0, domain.LPSLancar), domain.PABLCKPNMethodIndividualDCF)
	repo := &lpsRepoStub{placements: []domain.LPSPlacement{p}}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: pablParamsConfig()},
		txRunner: stubTxRunner{},
	}
	summary, err := svc.RunCollectiveCKPN(context.Background(), time.Now().UTC(), domain.Actor{})
	if err != nil {
		t.Fatalf("RunCollectiveCKPN: %v", err)
	}
	if summary.Skipped != 1 || summary.Processed != 0 {
		t.Fatalf("skipped=%d processed=%d, mau 1/0", summary.Skipped, summary.Processed)
	}
	if repo.collectiveCalled {
		t.Fatal("penempatan individual tidak boleh ditimpa")
	}
}

// Penempatan COLLECTIVE (termasuk yang belum pernah diasesmen) dihitung: dasar adalah
// outstanding dikurangi bagian yang dijamin LPS.
func TestRunCollectiveCKPN_CollectiveDihitung(t *testing.T) {
	belum := lpsServicePlacement(10_000_000, 2_000_000, domain.LPSLancar)
	repo := &lpsRepoStub{placements: []domain.LPSPlacement{belum}}
	svc := &lpsPlacementService{
		repo:     repo,
		config:   &lpsConfigStub{values: pablParamsConfig()},
		txRunner: stubTxRunner{},
	}
	summary, err := svc.RunCollectiveCKPN(context.Background(), time.Now().UTC(), domain.Actor{})
	if err != nil {
		t.Fatalf("RunCollectiveCKPN: %v", err)
	}
	if summary.Processed != 1 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("processed=%d skipped=%d failed=%d, mau 1/0/0", summary.Processed, summary.Skipped, summary.Failed)
	}
	// (10 juta - 2 juta) x 1% x 50% = 40.000.
	if !repo.gotCollectiveTarget.Equal(decimal.NewFromInt(40_000)) {
		t.Fatalf("target tersimpan %s, mau 40000", repo.gotCollectiveTarget)
	}
	if !summary.TotalTarget.Equal(decimal.NewFromInt(40_000)) {
		t.Fatalf("total target %s, mau 40000", summary.TotalTarget)
	}
	if len(repo.gotCollectiveBasis) == 0 || !strings.Contains(string(repo.gotCollectiveBasis), "collective_run") {
		t.Fatalf("basis %q tidak memuat asal run", repo.gotCollectiveBasis)
	}
}
