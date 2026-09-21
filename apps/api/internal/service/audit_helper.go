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

// writeAuditWithMetadata menulis event audit dengan metadata konteks di samping
// changes. Dipakai aksi yang penjelasannya diharapkan pembaca lama pada metadata
// (mis. alasan penolakan kredit), sementara pemanggil writeAudit yang lain tetap
// tidak berubah. auditRepo boleh nil dengan perilaku yang sama.
func writeAuditWithMetadata(ctx context.Context, auditRepo domain.AuditRepository, tx any, actor domain.Actor, action, resourceType, resourceID string, changes, metadata map[string]any) error {
	if auditRepo == nil {
		return nil
	}
	return auditRepo.Write(ctx, tx, domain.AuditEventFromActorWithMetadata(actor, action, resourceType, resourceID, changes, metadata))
}
