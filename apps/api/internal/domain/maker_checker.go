package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrMakerCheckerNotFound   = errors.New("permintaan maker-checker tidak ditemukan")
	ErrMakerCheckerNotPending = errors.New("permintaan maker-checker sudah diproses")
	ErrCannotSelfApprove      = errors.New("pembuat permintaan tidak boleh menyetujui permintaannya sendiri")
	ErrNoExecutorForAction    = errors.New("tidak ada eksekutor untuk jenis aksi maker-checker ini")
)

// PendingApprovalError dikembalikan ketika sebuah transaksi tidak diposting langsung
// karena melewati ambang persetujuan, melainkan masuk antrean maker-checker. Handler
// menerjemahkannya menjadi 202 Accepted, bukan error.
type PendingApprovalError struct {
	RequestID  uuid.UUID
	ActionType string
}

func (e *PendingApprovalError) Error() string {
	return "transaksi menunggu persetujuan pejabat berwenang"
}

// MakerCheckerExecutor mengeksekusi transaksi yang sudah disetujui. Dipanggil di dalam
// transaksi yang sama dengan perubahan status permintaan, sehingga persetujuan dan
// posting jurnalnya commit bersama: tidak mungkin ada persetujuan tanpa efek, atau
// efek tanpa jejak persetujuan.
type MakerCheckerExecutor interface {
	ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor Actor) error
}

type MakerCheckerStatus string

const (
	MakerCheckerPending  MakerCheckerStatus = "PENDING"
	MakerCheckerApproved MakerCheckerStatus = "APPROVED"
	MakerCheckerRejected MakerCheckerStatus = "REJECTED"
)

// MakerCheckerRequest adalah permintaan persetujuan berjenjang. Identitas pembuat dan
// pemeriksa disimpan sebagai string uuid agar sesuai kolom maker_id/checker_id.
type MakerCheckerRequest struct {
	ID           uuid.UUID
	ActionType   string
	Payload      map[string]any
	Status       MakerCheckerStatus
	MakerID      string
	CheckerID    *string
	MakerNotes   string
	CheckerNotes string
	// BranchCode adalah kode cabang pengajuan. Diisi dari Actor.BranchCode saat
	// pembuatan; dibaca kembali lewat join untuk menolak pemeriksa cabang lain.
	// Kosong berarti cabang tidak diketahui (data pra-migrasi) dan aksesnya
	// mengikuti semantik Actor.CanAccessBranch.
	BranchCode string
	// BusinessDate adalah tanggal bisnis bank (WIB) saat pengajuan dibuat. Dipakai
	// akumulasi batas harian untuk mengaitkan pengajuan PENDING ke hari bisnis yang
	// benar; nol berarti tidak diketahui (data pra-migrasi) dan pembaca jatuh ke
	// tanggal kalender WIB created_at.
	BusinessDate time.Time
	ReviewedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateMakerCheckerInput struct {
	ActionType string
	Amount     decimal.Decimal
	Payload    map[string]any
	Notes      string
}

type MakerCheckerRepository interface {
	CreateTx(ctx context.Context, tx any, req *MakerCheckerRequest) error
	GetByID(ctx context.Context, id uuid.UUID) (*MakerCheckerRequest, error)
	UpdateStatusTx(ctx context.Context, tx any, id uuid.UUID, status MakerCheckerStatus, checkerID, notes string) error
	ListPending(ctx context.Context, actor Actor) ([]MakerCheckerRequest, error)
}

type MakerCheckerService interface {
	CreateRequest(ctx context.Context, input CreateMakerCheckerInput, actor Actor) (*MakerCheckerRequest, error)
	Approve(ctx context.Context, id uuid.UUID, actor Actor, notes string) error
	Reject(ctx context.Context, id uuid.UUID, actor Actor, notes string) error
	ListPending(ctx context.Context, actor Actor) ([]MakerCheckerRequest, error)
	// Threshold membaca ambang nominal yang mewajibkan persetujuan pejabat untuk satu
	// jenis aksi. Nilai 0 berarti setiap nominal wajib disetujui.
	Threshold(ctx context.Context, actionType string) decimal.Decimal
}
