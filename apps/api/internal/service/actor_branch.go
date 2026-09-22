package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// resolveActorBranch memetakan kode cabang aktor ke baris cabang yang sah. Dipakai
// jalur tulis (pembukaan rekening, penempatan deposito) yang butuh cabang operasional
// nyata untuk atribusi dan penomoran.
//
// Aktor lintas cabang (SUPERADMIN/AUDITOR/SYSTEM) bertindak atas nama kantor pusat:
// kode cabangnya tidak wajib terdaftar, karena kode 'HO' pada akun kantor pusat bukan
// baris di tabel branches. Bila kodenya kosong atau tidak terdaftar, cabangnya jatuh
// ke kantor pusat (is_head_office) agar tidak diatribusikan ke cabang yang salah.
// Peran bercabang biasa tetap wajib punya cabang terdaftar (lihat
// Actor.RequiresRegisteredBranch): cabang tidak boleh diciptakan dari kode yang tidak ada.
func resolveActorBranch(ctx context.Context, repo domain.BranchRepository, actor domain.Actor) (*domain.Branch, error) {
	code := strings.TrimSpace(actor.BranchCode)
	if code != "" {
		branch, err := repo.GetByCode(ctx, code)
		if err == nil {
			return branch, nil
		}
		if !errors.Is(err, domain.ErrBranchNotFound) {
			return nil, err
		}
		if actor.RequiresRegisteredBranch() {
			return nil, domain.ErrBranchNotFound
		}
	} else if actor.RequiresRegisteredBranch() {
		return nil, fmt.Errorf("kode cabang aktor wajib diisi")
	}
	return headOfficeBranch(ctx, repo)
}

// headOfficeBranch mengembalikan cabang kantor pusat (is_head_office) dengan kode
// terkecil, atau nil bila belum ada. List sudah terurut kode, sehingga pemilihan ini
// konsisten dengan lookup langsung di customer_service.
func headOfficeBranch(ctx context.Context, repo domain.BranchRepository) (*domain.Branch, error) {
	branches, err := repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range branches {
		if branches[i].IsHeadOffice {
			return &branches[i], nil
		}
	}
	return nil, nil
}

// resolveCustomerBranch menegakkan cakupan unit aktor atas nasabah dan menentukan
// cabang yang dipakai operasi lanjutan (rekening/deposito mengikuti cabang
// nasabah). Aktor lintas cabang selalu boleh; nasabah tanpa cabang (data
// pra-migrasi) dibiarkan agar operasional tidak terblokir.
//
// Pemeriksaan memakai Actor.CanAccessBranch sehingga aktor ber-cakupan area/wilayah
// boleh melayani nasabah di cabang bawahannya (W14), bukan hanya cabangnya sendiri.
// Inilah jalur tulis yang sebelumnya bocor bila cakupan hanya dicek di UI.
func resolveCustomerBranch(ctx context.Context, repo domain.BranchRepository, actor domain.Actor, customerBranchID *uuid.UUID, actorBranch *domain.Branch) (*domain.Branch, error) {
	if customerBranchID == nil {
		return actorBranch, nil
	}
	customerBranch, err := repo.GetByID(ctx, *customerBranchID)
	if err != nil {
		return nil, fmt.Errorf("cabang nasabah tidak valid: %w", err)
	}
	if !actor.IsCrossBranch() && !actor.CanAccessBranch(customerBranch.Code) {
		return nil, domain.ErrCrossBranchAccess
	}
	return customerBranch, nil
}
