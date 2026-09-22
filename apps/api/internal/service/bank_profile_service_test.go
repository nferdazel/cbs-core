package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// stubBankProfileStore menyimpan satu baris profil in-memory dan merekam bahwa
// pembacaan/penulisan terjadi di dalam transaksi.
type stubBankProfileStore struct {
	profile *domain.BankProfile
	lastTx  any
	writes  int
}

func (s *stubBankProfileStore) Get(context.Context) (*domain.BankProfile, error) {
	if s.profile == nil {
		return nil, nil
	}
	cp := *s.profile
	return &cp, nil
}

func (s *stubBankProfileStore) GetForUpdate(_ context.Context, tx any) (*domain.BankProfile, error) {
	s.lastTx = tx
	return s.Get(context.Background())
}

func (s *stubBankProfileStore) UpdateTx(_ context.Context, tx any, p *domain.BankProfile) error {
	s.lastTx = tx
	cp := *p
	s.profile = &cp
	s.writes++
	return nil
}

func newBankProfileTestService(profile *domain.BankProfile) (domain.BankProfileService, *stubBankProfileStore, *stubAuditRepo) {
	store := &stubBankProfileStore{profile: profile}
	audit := &stubAuditRepo{}
	return &bankProfileService{repo: store, auditRepo: audit, txRunner: stubTxRunner{}}, store, audit
}

func profileStr(v string) *string { return &v }

// Nama kosong harus ditolak sebelum menyentuh penyimpanan maupun audit: profil
// kosong tidak boleh tersimpan sebagai "berhasil".
func TestBankProfileServiceTolakNamaKosong(t *testing.T) {
	svc, store, audit := newBankProfileTestService(nil)
	actor := domain.Actor{Username: "superadmin.uji", Role: domain.RoleSuperAdmin}

	_, err := svc.Update(context.Background(), domain.UpdateBankProfileInput{
		Name:    profileStr("   "),
		Address: profileStr("Jl. Contoh 1"),
	}, actor)
	if !errors.Is(err, domain.ErrBankProfileNameRequired) {
		t.Fatalf("err = %v, ingin ErrBankProfileNameRequired", err)
	}
	if store.writes != 0 {
		t.Fatalf("penyimpanan dipanggil %d kali, ingin 0 saat validasi gagal", store.writes)
	}
	if len(audit.events) != 0 {
		t.Fatalf("audit %d event, ingin 0 saat validasi gagal", len(audit.events))
	}
}

// Perubahan yang sah tersimpan dan meninggalkan audit before -> after dalam satu
// transaksi yang sama dengan penulisannya.
func TestBankProfileServiceUpdateTersimpanDanTeraudit(t *testing.T) {
	svc, store, audit := newBankProfileTestService(&domain.BankProfile{
		Name: "BPR Lama", City: "Bandung",
	})
	actor := domain.Actor{UserID: uuid.New(), Username: "superadmin.uji", Role: domain.RoleSuperAdmin}

	got, err := svc.Update(context.Background(), domain.UpdateBankProfileInput{
		Name:  profileStr("  BPR Baru Sejahtera  "),
		City:  profileStr("Jakarta"),
		Phone: profileStr("+62 21 5551234"),
	}, actor)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "BPR Baru Sejahtera" || got.City != "Jakarta" || got.Phone != "+62 21 5551234" {
		t.Fatalf("hasil = %+v, ingin nilai yang dirapikan", got)
	}
	if store.profile == nil || store.profile.Name != "BPR Baru Sejahtera" {
		t.Fatalf("store = %+v, ingin tersimpan", store.profile)
	}
	if store.lastTx != nil {
		t.Fatalf("transaksi = %v, ingin stubTxRunner (nil)", store.lastTx)
	}
	if len(audit.events) != 1 {
		t.Fatalf("audit %d event, ingin 1", len(audit.events))
	}
	event := audit.events[0]
	if event.Action != "UPDATE_BANK_PROFILE" || event.ResourceType != "bank_profile" || event.ResourceID != "1" {
		t.Fatalf("event audit tidak sesuai: %+v", event)
	}
	if event.ActorUsername != "superadmin.uji" || event.ActorRole != string(domain.RoleSuperAdmin) {
		t.Fatalf("identitas aktor tidak terekam: %+v", event)
	}
	nameChange, ok := event.Changes["name"].(map[string]any)
	if !ok || nameChange["before"] != "BPR Lama" || nameChange["after"] != "BPR Baru Sejahtera" {
		t.Fatalf("changes.name = %#v, ingin before/after", event.Changes["name"])
	}
}

// PUT dengan nilai sama tidak menulis apa pun: tidak ada perubahan, jadi tidak ada
// audit yang bisa disalahbaca sebagai perubahan identitas.
func TestBankProfileServiceUpdateIdempotenTanpaAudit(t *testing.T) {
	svc, store, audit := newBankProfileTestService(&domain.BankProfile{Name: "BPR Tetap", City: "Bogor"})
	actor := domain.Actor{Username: "superadmin.uji", Role: domain.RoleSuperAdmin}

	if _, err := svc.Update(context.Background(), domain.UpdateBankProfileInput{
		Name: profileStr("BPR Tetap"),
		City: profileStr("Bogor"),
	}, actor); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if store.writes != 0 {
		t.Fatalf("penyimpanan dipanggil %d kali, ingin 0 untuk nilai sama", store.writes)
	}
	if len(audit.events) != 0 {
		t.Fatalf("audit %d event, ingin 0 untuk nilai sama", len(audit.events))
	}
}

// Baris profil yang belum ada dinormalkan menjadi profil kosong, bukan nil.
func TestBankProfileServiceGetBarisKosong(t *testing.T) {
	svc, _, _ := newBankProfileTestService(nil)
	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.Configured() {
		t.Fatalf("Get = %+v, ingin profil kosong (bukan nil)", got)
	}
}
