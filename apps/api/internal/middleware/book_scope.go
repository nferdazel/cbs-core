package middleware

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
)

// BookScopeMiddleware mengisi cakupan buku tingkat instalasi dari system_config
// institution.book_scope ke klaim permintaan. Nilainya TIDAK disimpan di token
// supaya perubahan setelan (atau kesalahan setelan) langsung berlaku pada
// permintaan berikutnya, bukan menunggu token disegarkan.
//
// Bila cakupannya satu buku (KONVENSIONAL/SYARIAH), buku aktor dipaksa ke buku
// aktif itu. Ini diperlukan agar filter repository (bookReadClause/journalBookFilter
// lewat Actor.IsCrossBook) ikut menutup lini usaha yang tidak dilayani instalasi,
// termasuk bagi peran pengawas. Pada cakupan DUAL/nil middleware tidak mengubah
// apa pun sehingga perilaku lama identik.
//
// Harus dipasang SETELAH AuthMiddleware. Bila cfg nil (mis. uji) middleware tidak
// melakukan apa-apa dan semua aktor berperilaku DUAL.
func BookScopeMiddleware(cfg domain.SystemConfigService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg != nil {
				if claims, ok := domain.ClaimsFromContext(r.Context()); ok && claims != nil {
					raw := cfg.GetString(r.Context(), domain.ConfigKeyInstitutionBookScope, string(domain.ScopeDual))
					claims.BookScope = domain.ParseInstitutionBookScope(raw)
					if book := claims.BookScope.SingleBook(); book != "" {
						claims.Book = book
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
