package service

import (
	"context"
	"fmt"
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

// CKPNReadinessWarnings menghasilkan peringatan tegas saat start bila CKPN masih mati
// padahal instalasi sudah beroperasi (sudah punya jurnal atau tanggal bisnis). CKPN
// SAK EP wajib sejak 1 Januari 2025 (POJK No. 1 Tahun 2024 bagi BPR; POJK No. 24
// Tahun 2024 bagi BPRS), sehingga bank yang sudah beroperasi dengan ckpn.enabled=false
// sedang melanggar kewajiban itu — bukan sekadar belum siap.
//
// Mengapa bukan pembalikan otomatis: seluruh instalasi menjalankan migrasi yang sama,
// sehingga meng-UPDATE ckpn.enabled di migrasi akan menyalakan CKPN juga pada instalasi
// yang sedang berjalan — yang dilarang karena akan membentuk/menjurnal CKPN secara
// mundur. Karena itu mekanismenya adalah peringatan yang tidak mengubah perilaku, dan
// penyalakan dilakukan bank lewat langkah onboarding di docs/CKPN-SIAP-RILIS.md. Fungsi
// ini tidak menulis, tidak menjurnal, dan tidak mengubah setelan apa pun.
//
// cfg atau activity nil berarti tidak ada peringatan (lingkungan uji tanpa database).
func CKPNReadinessWarnings(ctx context.Context, cfg domain.SystemConfigService, activity domain.OperationalActivityReader) []string {
	if cfg == nil || activity == nil {
		return nil
	}
	// CKPN sudah menyala: tidak ada kewajiban yang tertunda.
	if cfg.GetBool(ctx, cfgCKPNEnabled, false) {
		return nil
	}
	operational, err := activity.HasOperationalActivity(ctx)
	if err != nil {
		// Kegagalan membaca tidak boleh disamarkan sebagai "belum beroperasi":
		// sebutkan agar operator memeriksa, bukan menyimpulkan aman.
		return []string{fmt.Sprintf("kesiapan CKPN tidak dapat dipastikan: status operasional instalasi gagal dibaca (%v)", err)}
	}
	if !operational {
		return nil
	}
	return []string{"CKPN belum dinyalakan (ckpn.enabled=false) padahal instalasi sudah memiliki jurnal/tanggal bisnis: bank sudah beroperasi tanpa membentuk CKPN, padahal CKPN SAK EP wajib sejak 1 Januari 2025 (POJK No. 1 Tahun 2024 / POJK No. 24 Tahun 2024). Selesaikan daftar periksa docs/CKPN-SIAP-RILIS.md lalu setel ckpn.enabled=true. Peringatan ini TIDAK mengubah perilaku apa pun; mode bayangan (ckpn.shadow_mode.enabled) hanya menghitung dan melaporkan tanpa menjurnal."}
}
