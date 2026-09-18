package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"cbs-core/apps/core-api/internal/domain"
)

// ExecutorRegistry memetakan jenis aksi maker-checker ke eksekutornya.
//
// Registry memutus siklus dependensi: ledger service membutuhkan maker-checker untuk
// mengajukan persetujuan, dan maker-checker membutuhkan ledger service untuk
// mengeksekusi transaksi yang disetujui. Keduanya bertemu di registry ini, bukan
// saling memegang secara langsung.
type ExecutorRegistry struct {
	mu        sync.RWMutex
	executors map[string]domain.MakerCheckerExecutor
}

func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{executors: make(map[string]domain.MakerCheckerExecutor)}
}

// Register mendaftarkan eksekutor untuk satu jenis aksi. Jenis aksi dinormalkan
// (huruf besar, tanpa spasi) agar "Deposit" dan "deposit" tidak menjadi dua entri.
func (r *ExecutorRegistry) Register(actionType string, executor domain.MakerCheckerExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[normalizeAction(actionType)] = executor
}

// ExecuteApproved meneruskan eksekusi ke eksekutor yang terdaftar. Bila tidak ada,
// permintaan ditolak dengan jelas daripada diam-diam menyetujui tanpa efek.
func (r *ExecutorRegistry) ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor domain.Actor) error {
	r.mu.RLock()
	executor, ok := r.executors[normalizeAction(actionType)]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrNoExecutorForAction, actionType)
	}
	return executor.ExecuteApproved(ctx, tx, actionType, payload, actor)
}

func normalizeAction(actionType string) string {
	return strings.ToUpper(strings.TrimSpace(actionType))
}
