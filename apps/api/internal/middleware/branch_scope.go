package middleware

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
)

// BranchScopeMiddleware mengisi cakupan unit organisasi aktor (cabang + seluruh
// turunannya pada hierarki area/wilayah) dari tabel branches setiap permintaan.
// Inilah satu tempat yang menjawab "unit apa yang boleh saya lihat/proses",
// dipakai bersama oleh jalur baca (branchReadClause) dan jalur tulis
// (Actor.CanAccessBranch), sehingga tidak ada mekanisme cakupan unit kedua.
//
// Nilai TIDAK ditanam di token supaya perubahan susunan hierarki langsung berlaku
// dan token lama tidak dapat menahan cakupan yang sudah dipersempit. Harus dipasang
// SETELAH AuthMiddleware. Peran lintas cabang dilewati (CanAccessBranch sudah
// mengizinkan semuanya). Bila resolver nil (mis. uji), tidak ada yang berubah:
// aktor berperilaku seperti sebelum hierarki ada.
//
// Bila resolusi gagal, cakupan dibiarkan belum terpasang sehingga aturannya jatuh
// ke perbandingan kode cabang lama. Ini disengaja: kegagalan sementara database
// tidak boleh mengunci pengguna dari unitnya sendiri.
func BranchScopeMiddleware(resolver domain.BranchScopeResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if resolver != nil {
				if claims, ok := domain.ClaimsFromContext(r.Context()); ok && claims != nil {
					if !(domain.Actor{Role: claims.Role}).IsCrossBranch() {
						if codes, err := resolver.ResolveScopeCodes(r.Context(), claims.BranchCode); err == nil {
							claims.BranchScope = domain.NewBranchScope(codes)
						}
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
