package service

import (
	"context"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
)

// InstallationValidationWarnings mengembalikan peringatan konfigurasi yang mengikuti
// cakupan buku tingkat instalasi. Tujuannya membuat ketidakcocokan terlihat, bukan
// membiarkannya diam sampai jurnal/angka salah:
//
//   - Instalasi yang MELAYANI buku syariah — SYARIAH maupun DUAL — dengan pemetaan akun
//     CKPN syariah (ckpn.coa.expense.syariah / ckpn.coa.reserve.syariah) masih KOSONG:
//     jurnal CKPN pembiayaan syariah akan jatuh ke akun global
//     ckpn.coa.expense/ckpn.coa.reserve yang berisi akun konvensional (50301/10950),
//     sehingga dana UUS tercampur buku konvensional. DUAL ikut diperiksa karena DUAL
//     justru nilai awal produksi (migrasi 000074) dan tetap memproses pembiayaan
//     syariah; memeriksa SYARIAH saja membuat peringatan ini tidak pernah muncul di
//     instalasi nyata.
//     Saklar CKPN tidak diubah di sini; peringatan tetap dicatat agar bank sempat
//     memetakan akun sebelum CKPN dinyalakan.
//
// Fungsi ini tidak mengubah angka atau status apa pun. Bila cfg nil, tidak ada
// peringatan (lingkungan uji tanpa konfigurasi).
func InstallationValidationWarnings(ctx context.Context, cfg domain.SystemConfigService) []string {
	if cfg == nil {
		return nil
	}
	scope := domain.ParseInstitutionBookScope(
		cfg.GetString(ctx, domain.ConfigKeyInstitutionBookScope, string(domain.ScopeDual)))

	// Cakupan yang memuat buku syariah: SYARIAH dan DUAL (lihat ActiveBooks/AllowsBook).
	// KONVENSIONAL tidak melayani pembiayaan syariah, jadi tidak perlu diperiksa.
	if !scope.AllowsBook(domain.BookSyariah) {
		return nil
	}

	var warnings []string
	if strings.TrimSpace(cfg.GetString(ctx, "ckpn.coa.expense.syariah", "")) == "" {
		warnings = append(warnings, "instalasi melayani buku syariah (SYARIAH/DUAL): ckpn.coa.expense.syariah masih kosong; jurnal beban CKPN pembiayaan syariah akan jatuh ke akun global ckpn.coa.expense (konvensional 50301). Petakan akun beban CKPN syariah sebelum mengaktifkan CKPN.")
	}
	if strings.TrimSpace(cfg.GetString(ctx, "ckpn.coa.reserve.syariah", "")) == "" {
		warnings = append(warnings, "instalasi melayani buku syariah (SYARIAH/DUAL): ckpn.coa.reserve.syariah masih kosong; jurnal cadangan CKPN pembiayaan syariah akan jatuh ke akun global ckpn.coa.reserve (konvensional 10950). Petakan akun cadangan CKPN syariah sebelum mengaktifkan CKPN.")
	}
	return warnings
}
