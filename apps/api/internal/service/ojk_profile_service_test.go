package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// stubOJKProfileStore menyimpan satu profil in-memory dan merekam penulisan.
type stubOJKProfileStore struct {
	profile *domain.OJKProfile
	writes  int
}

func (s *stubOJKProfileStore) Get(context.Context) (*domain.OJKProfile, error) {
	if s.profile == nil {
		return &domain.OJKProfile{}, nil
	}
	cp := *s.profile
	return &cp, nil
}

func (s *stubOJKProfileStore) GetTx(ctx context.Context, _ any) (*domain.OJKProfile, error) {
	return s.Get(ctx)
}

func (s *stubOJKProfileStore) SaveTx(_ context.Context, _ any, p *domain.OJKProfile, _ uuid.UUID) error {
	cp := *p
	s.profile = &cp
	s.writes++
	return nil
}

func newOJKProfileTestService(profile *domain.OJKProfile) (domain.OJKProfileService, *stubOJKProfileStore, *stubAuditRepo) {
	store := &stubOJKProfileStore{profile: profile}
	audit := &stubAuditRepo{}
	return &ojkProfileService{repo: store, auditRepo: audit, txRunner: stubTxRunner{}}, store, audit
}

func ojkPtr(v string) *string { return &v }

// Payload kosong ditolak sebelum menyentuh penyimpanan/audit: aksi kosong bukan perubahan.
func TestOJKProfileServiceTolakPayloadKosong(t *testing.T) {
	svc, store, audit := newOJKProfileTestService(nil)
	_, err := svc.Update(context.Background(), domain.UpdateOJKProfileInput{}, domain.Actor{Role: domain.RoleSuperAdmin})
	if !errors.Is(err, domain.ErrOJKProfileEmpty) {
		t.Fatalf("err = %v, ingin ErrOJKProfileEmpty", err)
	}
	if store.writes != 0 || len(audit.events) != 0 {
		t.Fatalf("writes=%d audit=%d, ingin 0", store.writes, len(audit.events))
	}
}

// Perubahan sah tersimpan dan meninggalkan audit before -> after. Empat belas butir
// Form 00.00 yang baru pun dapat diisi lewat layanan ini.
func TestOJKProfileServiceUpdateTersimpanDanTeraudit(t *testing.T) {
	svc, store, audit := newOJKProfileTestService(&domain.OJKProfile{BankEmail: "lama@contoh.id"})
	actor := domain.Actor{UserID: uuid.New(), Username: "superadmin.uji", Role: domain.RoleSuperAdmin}

	got, err := svc.Update(context.Background(), domain.UpdateOJKProfileInput{
		BankEmail:            ojkPtr("  baru@contoh.id  "),
		BankWebsite:          ojkPtr("https://bpr.contoh.id"),
		BankCityCode:         ojkPtr("3171"),
		AuditInfo:            ojkPtr("KAP Contoh, opini WTP"),
		UltimateShareholders: ojkPtr("Budi Santoso"),
	}, actor)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.BankEmail != "baru@contoh.id" || got.AuditInfo != "KAP Contoh, opini WTP" {
		t.Fatalf("hasil tidak dirapikan: %+v", got)
	}
	if store.writes != 1 {
		t.Fatalf("writes = %d, ingin 1", store.writes)
	}
	if len(audit.events) != 1 {
		t.Fatalf("audit %d event, ingin 1", len(audit.events))
	}
	event := audit.events[0]
	if event.Action != "UPDATE_OJK_PROFILE" || event.ResourceType != "ojk_profile" || event.ResourceID != "1" {
		t.Fatalf("event audit tidak sesuai: %+v", event)
	}
	emailChange, ok := event.Changes[domain.OJKBankEmailKey].(map[string]any)
	if !ok || emailChange["before"] != "lama@contoh.id" || emailChange["after"] != "baru@contoh.id" {
		t.Fatalf("changes[surel] = %#v", event.Changes[domain.OJKBankEmailKey])
	}
}

// Bentuk salah ditolak sebelum menulis; alasan belum tersedia tetap sah.
func TestOJKProfileServiceTolakBentukSalahTanpaTulis(t *testing.T) {
	svc, store, audit := newOJKProfileTestService(nil)
	_, err := svc.Update(context.Background(), domain.UpdateOJKProfileInput{
		BankEmail: ojkPtr("bukan-surel"),
	}, domain.Actor{Role: domain.RoleSuperAdmin})
	if !errors.Is(err, domain.ErrOJKProfileEmailInvalid) {
		t.Fatalf("err = %v, ingin ErrOJKProfileEmailInvalid", err)
	}
	if store.writes != 0 || len(audit.events) != 0 {
		t.Fatalf("writes=%d audit=%d, ingin 0 saat validasi gagal", store.writes, len(audit.events))
	}
}

// Bank boleh mengosongkan bidang: "diisi -> dikosongkan" tersimpan dan teraudit,
// bukan ditolak atau diabaikan.
func TestOJKProfileServiceBolehMengosongkanBidang(t *testing.T) {
	svc, store, audit := newOJKProfileTestService(&domain.OJKProfile{BankEmail: "lama@contoh.id"})
	got, err := svc.Update(context.Background(), domain.UpdateOJKProfileInput{
		BankEmail: ojkPtr(""),
	}, domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.BankEmail != "" || store.profile.BankEmail != "" {
		t.Fatalf("bidang tidak dikosongkan: %+v", got)
	}
	if len(audit.events) != 1 {
		t.Fatalf("audit %d event, ingin 1", len(audit.events))
	}
}

// PUT dengan nilai sama tidak menulis/audit.
func TestOJKProfileServiceUpdateIdempotenTanpaAudit(t *testing.T) {
	svc, store, audit := newOJKProfileTestService(&domain.OJKProfile{BankEmail: "sama@contoh.id"})
	if _, err := svc.Update(context.Background(), domain.UpdateOJKProfileInput{
		BankEmail: ojkPtr("sama@contoh.id"),
	}, domain.Actor{Role: domain.RoleSuperAdmin}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if store.writes != 0 || len(audit.events) != 0 {
		t.Fatalf("writes=%d audit=%d, ingin 0 untuk nilai sama", store.writes, len(audit.events))
	}
}
