package domain_test

import (
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// Debit hanya boleh dari rekening ACTIVE. DORMANT punya sentinel sendiri agar
// pesannya menyuruh nasabah reaktivasi; FROZEN/CLOSED tetap ErrAccountInactive.
func TestAccountDebitAllowed(t *testing.T) {
	cases := []struct {
		status domain.AccountStatus
		want   error
	}{
		{domain.AccountStatusActive, nil},
		{domain.AccountStatusDormant, domain.ErrAccountDormant},
		{domain.AccountStatusFrozen, domain.ErrAccountInactive},
		{domain.AccountStatusClosed, domain.ErrAccountInactive},
	}
	for _, c := range cases {
		err := domain.AccountDebitAllowed(c.status)
		if !errors.Is(err, c.want) {
			t.Fatalf("debit %s: dapat %v, mau %v", c.status, err, c.want)
		}
	}
}

// Kredit diterima untuk ACTIVE dan DORMANT: dana masuk tidak boleh tertahan.
// FROZEN/CLOSED tetap ditolak.
func TestAccountCreditAllowed(t *testing.T) {
	cases := []struct {
		status domain.AccountStatus
		want   error
	}{
		{domain.AccountStatusActive, nil},
		{domain.AccountStatusDormant, nil},
		{domain.AccountStatusFrozen, domain.ErrAccountInactive},
		{domain.AccountStatusClosed, domain.ErrAccountInactive},
	}
	for _, c := range cases {
		err := domain.AccountCreditAllowed(c.status)
		if !errors.Is(err, c.want) {
			t.Fatalf("kredit %s: dapat %v, mau %v", c.status, err, c.want)
		}
	}
}

func TestDormantCutoff(t *testing.T) {
	asOf := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	got := domain.DormantCutoff(asOf, 12)
	want := time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("cutoff = %s, mau %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

// Dasar waktu: COALESCE(last_activity_at, opened_at, created_at). Bila ketiganya
// kosong, rekening dilewati (ok=false), bukan dikarang waktunya.
func TestAccountActivityBase(t *testing.T) {
	last := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	opened := time.Date(2025, 5, 6, 0, 0, 0, 0, time.UTC)
	created := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

	t.Run("memakai last_activity_at", func(t *testing.T) {
		got, ok := domain.AccountActivityBase(&last, &opened, created)
		if !ok || !got.Equal(last) {
			t.Fatalf("dapat %s ok=%v, mau %s", got, ok, last)
		}
	})
	t.Run("jatuh ke opened_at saat last_activity_at NULL", func(t *testing.T) {
		got, ok := domain.AccountActivityBase(nil, &opened, created)
		if !ok || !got.Equal(opened) {
			t.Fatalf("dapat %s ok=%v, mau %s", got, ok, opened)
		}
	})
	t.Run("jatuh ke created_at saat opened_at NULL", func(t *testing.T) {
		got, ok := domain.AccountActivityBase(nil, nil, created)
		if !ok || !got.Equal(created) {
			t.Fatalf("dapat %s ok=%v, mau %s", got, ok, created)
		}
	})
	t.Run("tanpa waktu sama sekali dilewati", func(t *testing.T) {
		got, ok := domain.AccountActivityBase(nil, nil, time.Time{})
		if ok {
			t.Fatalf("harus ok=false, dapat %s", got)
		}
	})
}

// Batas memakai perbandingan ketat: aktivitas tepat di ambang BELUM dormant.
func TestIsDormantDue(t *testing.T) {
	cutoff := time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC)

	if domain.IsDormantDue(cutoff, cutoff) {
		t.Fatal("aktivitas tepat di ambang tidak boleh dianggap dormant")
	}
	justBefore := cutoff.Add(-time.Second)
	if !domain.IsDormantDue(justBefore, cutoff) {
		t.Fatal("aktivitas sebelum ambang harus dormant")
	}
	justAfter := cutoff.Add(time.Second)
	if domain.IsDormantDue(justAfter, cutoff) {
		t.Fatal("aktivitas setelah ambang tidak boleh dormant")
	}
}
