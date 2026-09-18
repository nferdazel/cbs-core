package service

import (
	"context"

	"cbs-core/apps/core-api/internal/domain"
)

// writeAudit menulis event audit di dalam transaksi bisnis bila auditRepo tersedia.
// auditRepo boleh nil (mis. pada test yang tidak menyiapkan audit): penulisan
// dilewati tanpa error agar perilaku bisnis tidak berubah.
func writeAudit(ctx context.Context, auditRepo domain.AuditRepository, tx any, actor domain.Actor, action, resourceType, resourceID string, changes map[string]any) error {
	if auditRepo == nil {
		return nil
	}
	return auditRepo.Write(ctx, tx, domain.AuditEventFromActor(actor, action, resourceType, resourceID, changes))
}
