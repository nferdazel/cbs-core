package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// collateralRepoStub menyimpan agunan di memori dan meniru perilaku kolom generated:
// bound_amount dihitung dari taksasi dan haircut, sama seperti database.
type collateralRepoStub struct {
	domain.CollateralRepository
	created     []domain.LoanCollateral
	branchCodes []string
	existing    []domain.LoanCollateral
	sums        map[uuid.UUID]decimal.Decimal
	sumErr      error
	sumCalled   bool
}

func (r *collateralRepoStub) Create(ctx context.Context, c *domain.LoanCollateral, branchCode string) error {
	c.ID = uuid.New()
	c.BoundAmount = c.AppraisalValue.Mul(decimal.NewFromInt(1).Sub(c.HaircutPercent.Div(decimal.NewFromInt(100))))
	r.created = append(r.created, *c)
	r.branchCodes = append(r.branchCodes, branchCode)
	return nil
}

func (r *collateralRepoStub) GetByID(ctx context.Context, id uuid.UUID) (*domain.LoanCollateral, error) {
	for _, c := range r.existing {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, domain.ErrCollateralNotFound
}

// SumActiveBoundByLoan dipakai jalur PPAP. Nilai dikembalikan dari peta tetap agar test
// dapat memeriksa perilaku saklar tanpa database.
func (r *collateralRepoStub) SumActiveBoundByLoan(ctx context.Context, loanIDs []uuid.UUID) (map[uuid.UUID]decimal.Decimal, error) {
	r.sumCalled = true
	if r.sumErr != nil {
		return nil, r.sumErr
	}
	return r.sums, nil
}

func (r *collateralRepoStub) ListByLoan(ctx context.Context, loanID uuid.UUID) ([]domain.LoanCollateral, error) {
	list := make([]domain.LoanCollateral, 0)
	for _, c := range r.existing {
		if c.LoanID == loanID {
			list = append(list, c)
		}
	}
	return list, nil
}

type collateralConfigStub struct {
	domain.SystemConfigService
	values map[string]string
}

func (c *collateralConfigStub) GetString(ctx context.Context, key, fallback string) string {
	if v, ok := c.values[key]; ok {
		return v
	}
	return fallback
}

func collateralInput() domain.CollateralInput {
	return domain.CollateralInput{
		LoanID:         uuid.New(),
		CollateralType: domain.CollateralTanahBangunan,
		Description:    "SHM No. 123",
		DocumentNumber: "SKMHT-2026-001",
		OwnerName:      "Budi Santoso",
		AppraisalValue: decimal.NewFromInt(500000000),
		AppraisalDate:  time.Now().UTC().AddDate(0, 0, -1),
	}
}

func collateralActor(branch string) domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "ao1", Role: domain.RoleSupervisor, BranchCode: branch}
}

// Tanpa kebijakan di konfigurasi, haircut bawaan 100 dipakai: agunan belum mengurangi
// eksposur. Ini mengunci janji bahwa rilis modul agunan tidak menggeser angka PPAP.
func TestCollateralCreate_HaircutBawaanTanpaPengurangan(t *testing.T) {
	repo := &collateralRepoStub{}
	audit := &reversalAuditRepo{}
	svc := NewCollateralService(repo, &collateralConfigStub{}, audit)

	c, err := svc.Create(context.Background(), collateralInput(), collateralActor("001"))
	if err != nil {
		t.Fatalf("pencatatan agunan gagal: %v", err)
	}
	if !c.HaircutPercent.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("haircut %s, mau 100", c.HaircutPercent)
	}
	if !c.BoundAmount.IsZero() {
		t.Fatalf("nilai pengurang %s, mau 0 saat haircut 100", c.BoundAmount)
	}
	if c.Status != domain.CollateralActive {
		t.Fatalf("status %s, mau ACTIVE", c.Status)
	}
	if repo.branchCodes[0] != "001" {
		t.Fatalf("cabang yang dikirim %q, mau dari aktor (001)", repo.branchCodes[0])
	}
	if c.CreatedBy != collateralActor("001").Username {
		t.Fatalf("pencatat %q, mau nama aktor", c.CreatedBy)
	}
}

func TestCollateralCreate_MemakaiKebijakanHaircutDariKonfigurasi(t *testing.T) {
	repo := &collateralRepoStub{}
	audit := &reversalAuditRepo{}
	config := &collateralConfigStub{values: map[string]string{"collateral.haircut.tanah_bangunan": "80"}}
	svc := NewCollateralService(repo, config, audit)

	c, err := svc.Create(context.Background(), collateralInput(), collateralActor("001"))
	if err != nil {
		t.Fatalf("pencatatan agunan gagal: %v", err)
	}
	if !c.HaircutPercent.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("haircut %s, mau 80 dari konfigurasi", c.HaircutPercent)
	}
	// 500.000.000 dikurangi haircut 80% = 100.000.000 nilai pengurang.
	if !c.BoundAmount.Equal(decimal.NewFromInt(100000000)) {
		t.Fatalf("nilai pengurang %s, mau 100000000", c.BoundAmount)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "CREATE_COLLATERAL" {
		t.Fatalf("audit pencatatan agunan tidak sesuai: %+v", audit.events)
	}
	if audit.events[0].Changes["bound_amount"] != "100000000.00" {
		t.Fatalf("nilai pengurang saat pencatatan harus terekam: %+v", audit.events[0].Changes)
	}
}

func TestCollateralCreate_MenolakHaircutDanKebijakanTidakSah(t *testing.T) {
	operator := decimal.NewFromInt(120)
	cases := []struct {
		name      string
		config    map[string]string
		haircut   *decimal.Decimal
		wantValid bool
	}{
		{name: "operator mengisi 120 persen", haircut: &operator},
		{name: "kebijakan konfigurasi rusak", config: map[string]string{"collateral.haircut.tanah_bangunan": "delapan puluh"}},
		{name: "kebijakan konfigurasi di luar rentang", config: map[string]string{"collateral.haircut.tanah_bangunan": "150"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewCollateralService(&collateralRepoStub{}, &collateralConfigStub{values: tc.config}, &reversalAuditRepo{})
			input := collateralInput()
			input.HaircutPercent = tc.haircut

			if _, err := svc.Create(context.Background(), input, collateralActor("001")); err == nil {
				t.Fatal("kebijakan haircut tidak sah seharusnya ditolak, bukan dipakai diam-diam")
			}
		})
	}
}

func TestCollateralCreate_MenolakDataTidakLengkap(t *testing.T) {
	cases := []struct {
		name    string
		ubah    func(*domain.CollateralInput)
		actor   domain.Actor
		wantErr error
	}{
		{
			name:    "tanpa nomor bukti ikatan",
			ubah:    func(i *domain.CollateralInput) { i.DocumentNumber = " " },
			wantErr: domain.ErrCollateralDocumentRequired,
		},
		{
			name:    "taksasi nol",
			ubah:    func(i *domain.CollateralInput) { i.AppraisalValue = decimal.Zero },
			wantErr: domain.ErrCollateralAppraisalInvalid,
		},
		{
			name:    "aktor tanpa cabang",
			actor:   domain.Actor{UserID: uuid.New(), Username: "admin", Role: domain.RoleAdmin},
			wantErr: nil, // diperiksa lewat pesan di bawah
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewCollateralService(&collateralRepoStub{}, &collateralConfigStub{}, &reversalAuditRepo{})
			input := collateralInput()
			if tc.ubah != nil {
				tc.ubah(&input)
			}
			actor := tc.actor
			if actor.UserID == uuid.Nil {
				actor = collateralActor("001")
			}

			_, err := svc.Create(context.Background(), input, actor)
			if err == nil {
				t.Fatal("input tidak sah seharusnya ditolak")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("kesalahan %v, ingin %v", err, tc.wantErr)
			}
		})
	}
}

// Agunan cabang lain dilaporkan sebagai tidak ditemukan, bukan dibedakan: membedakannya
// memberi tahu pemanggil bahwa agunan dengan id tertentu memang ada di bank ini.
func TestCollateralGetByID_MenyamarkanCabangLain(t *testing.T) {
	id := uuid.New()
	repo := &collateralRepoStub{existing: []domain.LoanCollateral{{
		ID: id, LoanID: uuid.New(), BranchCode: "999", Status: domain.CollateralActive,
	}}}
	svc := NewCollateralService(repo, &collateralConfigStub{}, &reversalAuditRepo{})

	_, err := svc.GetByID(context.Background(), id, collateralActor("001"))
	if !errors.Is(err, domain.ErrCollateralNotFound) {
		t.Fatalf("kesalahan %v, ingin ErrCollateralNotFound", err)
	}
}

func TestCollateralListByLoan_MenyaringCabangLain(t *testing.T) {
	loanID := uuid.New()
	repo := &collateralRepoStub{existing: []domain.LoanCollateral{
		{ID: uuid.New(), LoanID: loanID, BranchCode: "001"},
		{ID: uuid.New(), LoanID: loanID, BranchCode: "999"},
	}}
	svc := NewCollateralService(repo, &collateralConfigStub{}, &reversalAuditRepo{})

	list, err := svc.ListByLoan(context.Background(), loanID, collateralActor("001"))
	if err != nil {
		t.Fatalf("daftar agunan gagal: %v", err)
	}
	if len(list) != 1 || list[0].BranchCode != "001" {
		t.Fatalf("agunan cabang lain ikut terkirim: %+v", list)
	}
}
