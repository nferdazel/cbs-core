package domain

import (
	"context"
	"encoding/json"
	"time"
)

// AuditEvent adalah catatan bisnis atas sebuah aksi: siapa melakukan apa, terhadap
// objek apa, dan apa yang berubah. Berbeda dari application log yang bersifat teknis,
// audit log disimpan di database, bersifat append-only, dan menjadi bukti kepatuhan.
type AuditEvent struct {
	ActorID       string         `json:"actor_id"`
	ActorRole     string         `json:"actor_role"`
	ActorUsername string         `json:"actor_username,omitempty"`
	Action        string         `json:"action"`
	ResourceType  string         `json:"resource_type"`
	ResourceID    string         `json:"resource_id"`
	IPAddress     string         `json:"ip_address,omitempty"`
	UserAgent     string         `json:"user_agent,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
	Changes       map[string]any `json:"changes,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

// ChangesJSON mengembalikan changes sebagai JSON untuk kolom JSONB.
func (e AuditEvent) ChangesJSON() []byte {
	if len(e.Changes) == 0 {
		return []byte("{}")
	}
	raw, err := json.Marshal(e.Changes)
	if err != nil {
		// Perubahan yang tidak bisa diserialkan tidak boleh menggagalkan aksi bisnis;
		// dicatat sebagai objek kosong, dan pemanggil tetap menyimpan event-nya.
		return []byte("{}")
	}
	return raw
}

// AuditEventFromActor membangun audit event dengan identitas pelaku dari JWT.
// IP dan request id diambil dari Actor dan context agar korelasi ke application log
// tetap terjaga.
func AuditEventFromActor(actor Actor, action, resourceType, resourceID string, changes map[string]any) AuditEvent {
	return AuditEvent{
		ActorID:       actor.UserID.String(),
		ActorRole:     string(actor.Role),
		ActorUsername: actor.Username,
		Action:        action,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		IPAddress:     actor.IPAddress,
		RequestID:     actor.RequestID,
		Changes:       changes,
		CreatedAt:     time.Now().UTC(),
	}
}

// AuditRepository menyimpan dan membaca audit log. Tidak ada operasi ubah atau hapus:
// audit log bersifat append-only.
type AuditRepository interface {
	Write(ctx context.Context, tx any, event AuditEvent) error
	List(ctx context.Context, resourceType, resourceID string, limit int) ([]AuditEvent, error)
}
