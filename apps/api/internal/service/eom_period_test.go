package service

import (
	"testing"
	"time"
)

// EOM menutup satu bulan penuh: hari terakhir bulan itu, atau hari pertama bulan
// berikutnya sesudah tutup hari terakhir. Tanggal lain ditolak karena akan membayarkan
// bunga untuk periode yang belum dijalani.
func TestEomPeriod_MenutupBulanYangSelesaiSaja(t *testing.T) {
	cases := []struct {
		name     string
		business time.Time
		want     string
		wantErr  bool
	}{
		{"hari terakhir bulan", time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), "2026-09", false},
		{"hari pertama bulan berikutnya", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "2026-09", false},
		{"hari terakhir Februari kabisat", time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC), "2028-02", false},
		{"pertengahan bulan", time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), "", true},
		{"hari pertama tahun", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), "2026-12", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			period, err := eomPeriod(tc.business)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("tanggal %s harus ditolak", tc.business.Format("2006-01-02"))
				}
				return
			}
			if err != nil {
				t.Fatalf("eomPeriod: %v", err)
			}
			if got := period.Format("2006-01"); got != tc.want {
				t.Fatalf("periode %s, ingin %s", got, tc.want)
			}
		})
	}
}
