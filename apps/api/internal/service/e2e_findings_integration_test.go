package service_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
)

// Uji integrasi temuan E2E putaran ini terhadap PostgreSQL sungguhan: tutup rekening,
// ubah nasabah, kelola keanggotaan grup, dan normal_balance saat membuka rekening.
// Di-skip kecuali CBS_TEST_DB_DSN diisi.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiTemuanE2E -v

var findingsNikCounter int64

// findingsNik menghasilkan NIK 16 digit unik per pemanggilan tanpa bergantung pada
// helper berkas uji lain.
func findingsNik() string {
	n := atomic.AddInt64(&findingsNikCounter, 1)
	return fmt.Sprintf("99%012d%02d", time.Now().UnixNano()%1_000_000_000_000, n%100)
}

func findingsSuperadmin(e *actorBranchEnv) domain.Actor {
	return domain.Actor{
		UserID:     e.money.actor.UserID,
		Username:   "superadmin.temuan",
		Role:       domain.RoleSuperAdmin,
		BranchCode: "HO",
	}
}

func seedFindingsCustomer(t *testing.T, e *actorBranchEnv, name string) *domain.Customer {
	t.Helper()
	cust, err := e.money.customerSvc.RegisterCustomer(e.money.ctx, domain.CreateCustomerInput{
		FullName:     name,
		IDCardNumber: findingsNik(),
		PhoneNumber:  "0812000000",
	}, findingsSuperadmin(e))
	if err != nil {
		t.Fatalf("mendaftarkan nasabah %q: %v", name, err)
	}
	return cust
}

// TestIntegrasiTemuanE2ETutupRekening membuktikan rute tutup rekening punya jalur
// service nyata: rekening bersih menjadi CLOSED + teraudit, rekening bersaldo ditolak,
// dan pembukaan rekening mengisi normal_balance dari bagan akun (bukan kosong).
func TestIntegrasiTemuanE2ETutupRekening(t *testing.T) {
	e := newActorBranchEnv(t)
	actor := findingsSuperadmin(e)
	cust := seedFindingsCustomer(t, e, "Nasabah Tutup Rekening")

	product, err := e.money.productRepo.GetByCode(e.money.ctx, "TAB-CONV")
	if err != nil {
		t.Fatalf("membaca produk tabungan: %v", err)
	}
	acc, err := e.accountSvc.OpenAccount(e.money.ctx, domain.OpenAccountInput{
		CustomerID: cust.ID, ProductID: product.ID, Currency: "IDR",
	}, actor)
	if err != nil {
		t.Fatalf("membuka rekening: %v", err)
	}
	// R3: normal balance/buku diisi dari bagan akun pada respons buka rekening.
	if acc.NormalBalance != domain.BalanceTypeCredit {
		t.Fatalf("normal_balance = %q, ingin CREDIT", acc.NormalBalance)
	}
	if acc.COABook != domain.BookConventional {
		t.Fatalf("coa_book = %q, ingin CONVENTIONAL", acc.COABook)
	}

	closed, err := e.accountSvc.CloseAccount(e.money.ctx, acc.AccountNumber, "uji tutup", actor)
	if err != nil {
		t.Fatalf("menutup rekening bersih: %v", err)
	}
	if closed.Status != domain.AccountStatusClosed {
		t.Fatalf("status = %q, ingin CLOSED", closed.Status)
	}
	var action string
	if err := e.money.db.QueryRowContext(e.money.ctx, `
		SELECT action FROM audit_logs
		WHERE resource_type='account' AND resource_id=$1 AND action='CLOSE_ACCOUNT'
		ORDER BY created_at DESC LIMIT 1`, acc.ID).Scan(&action); err != nil {
		t.Fatalf("audit CLOSE_ACCOUNT tidak ditemukan: %v", err)
	}

	// Rekening yang sudah CLOSED tidak dapat ditutup lagi.
	if _, err := e.accountSvc.CloseAccount(e.money.ctx, acc.AccountNumber, "", actor); !errors.Is(err, domain.ErrAccountNotClosable) {
		t.Fatalf("tutup ulang: err = %v, ingin ErrAccountNotClosable", err)
	}

	// Rekening bersaldo tidak boleh ditutup: uang nasabah tidak boleh hilang.
	acc2, err := e.accountSvc.OpenAccount(e.money.ctx, domain.OpenAccountInput{
		CustomerID: cust.ID, ProductID: product.ID, Currency: "IDR",
	}, actor)
	if err != nil {
		t.Fatalf("membuka rekening kedua: %v", err)
	}
	if _, err := e.money.db.ExecContext(e.money.ctx,
		`UPDATE accounts SET balance=100000, available_balance=100000 WHERE id=$1`, acc2.ID); err != nil {
		t.Fatalf("mengisi saldo rekening uji: %v", err)
	}
	if _, err := e.accountSvc.CloseAccount(e.money.ctx, acc2.AccountNumber, "", actor); !errors.Is(err, domain.ErrAccountCloseBalance) {
		t.Fatalf("tutup rekening bersaldo: err = %v, ingin ErrAccountCloseBalance", err)
	}
}

// TestIntegrasiTemuanE2EUbahNasabah membuktikan perubahan data nasabah berlaku,
// token nama diganti (nama lama tidak lagi menemukan), dan NIK milik nasabah lain
// ditolak sebagai duplikat.
func TestIntegrasiTemuanE2EUbahNasabah(t *testing.T) {
	e := newActorBranchEnv(t)
	actor := findingsSuperadmin(e)
	cust := seedFindingsCustomer(t, e, "Budi Zzzz")

	updated, err := e.money.customerSvc.UpdateCustomer(e.money.ctx, cust.ID, domain.UpdateCustomerInput{
		FullName:     "Budi Yyyy",
		IDCardNumber: cust.IDCardNumber,
		PhoneNumber:  "0812999999",
		// Email harus unik per run: indeks email parsial unik menolak alamat yang
		// sudah dipakai baris lama dari run sebelumnya.
		Email: "budi." + findingsNik() + "@uji.local",
	}, actor)
	if err != nil {
		t.Fatalf("mengubah nasabah: %v", err)
	}
	if updated.FullName != "Budi Yyyy" || updated.PhoneNumber != "0812999999" {
		t.Fatalf("perubahan tidak tersimpan: %+v", updated)
	}

	found, _, err := e.money.customerSvc.ListCustomers(e.money.ctx, 1, 20, "Yyyy", actor)
	if err != nil {
		t.Fatalf("mencari nama baru: %v", err)
	}
	if !containsCustomer(found, cust.ID) {
		t.Fatal("nama baru tidak ditemukan lewat token; token nama belum diperbarui")
	}
	// Token lama harus hilang; bila ReplaceNameTokens tidak dipanggil, "Zzzz" masih
	// menemukan nasabah ini dan uji gagal.
	stale, _, err := e.money.customerSvc.ListCustomers(e.money.ctx, 1, 20, "Zzzz", actor)
	if err != nil {
		t.Fatalf("mencari nama lama: %v", err)
	}
	if containsCustomer(stale, cust.ID) {
		t.Fatal("nama lama masih menemukan nasabah; token lama tidak dihapus")
	}

	other := seedFindingsCustomer(t, e, "Nasabah Lain")
	if _, err := e.money.customerSvc.UpdateCustomer(e.money.ctx, other.ID, domain.UpdateCustomerInput{
		FullName:     "Nasabah Lain",
		IDCardNumber: cust.IDCardNumber,
	}, actor); !errors.Is(err, domain.ErrDuplicateIDCard) {
		t.Fatalf("NIK milik nasabah lain: err = %v, ingin ErrDuplicateIDCard", err)
	}
}

// TestIntegrasiTemuanE2EKeanggotaanGrup membuktikan keanggotaan grup dapat dikelola
// lewat maker-checker: ADD_MEMBER/REMOVE_MEMBER berlaku, idempoten, dan teraudit.
func TestIntegrasiTemuanE2EKeanggotaanGrup(t *testing.T) {
	db, ctx := newPermissionDB(t)
	rig := newPermissionRig(t, db, ctx)

	makerID, _ := seedTempUser(t, db, ctx, domain.RoleAdmin)
	checkerID, _ := seedTempUser(t, db, ctx, domain.RoleSuperAdmin)
	targetID, _ := seedTempUser(t, db, ctx, domain.RoleCS)
	maker := actorFor(makerID, "pembuat-anggota", domain.RoleAdmin)
	checker := actorFor(checkerID, "pemeriksa-anggota", domain.RoleSuperAdmin)

	const group = "ROLE_TELLER"
	repo := postgres.NewPermissionRepository(db)

	// Tambah anggota.
	addReq, err := rig.perm.RequestChange(ctx, domain.PermissionChangeInput{
		GroupCode: group,
		Operation: domain.PermissionChangeAddMember,
		UserID:    targetID.String(),
	}, maker)
	if err != nil {
		t.Fatalf("mengajukan tambah anggota: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM maker_checker_requests WHERE id=$1`, addReq.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_group_members WHERE user_id=$1`, targetID)
	})
	if err := rig.mc.Approve(ctx, addReq.ID, checker, "uji"); err != nil {
		t.Fatalf("menyetujui tambah anggota: %v", err)
	}
	inGroup, err := repo.UserInGroup(ctx, group, targetID)
	if err != nil {
		t.Fatalf("memeriksa keanggotaan: %v", err)
	}
	if !inGroup {
		t.Fatal("anggota belum masuk grup setelah persetujuan")
	}

	var changesRaw []byte
	if err := db.QueryRowContext(ctx, `
		SELECT changes FROM audit_logs
		WHERE resource_type='user_group' AND resource_id=$1 AND action='GROUP_MEMBERSHIP_CHANGE'
		ORDER BY created_at DESC LIMIT 1`, group).Scan(&changesRaw); err != nil {
		t.Fatalf("audit keanggotaan tidak ditemukan: %v", err)
	}
	var changes map[string]any
	if err := json.Unmarshal(changesRaw, &changes); err != nil {
		t.Fatalf("audit changes bukan JSON: %v", err)
	}
	if changes["before"] != false || changes["after"] != true {
		t.Fatalf("jejak audit salah: %+v", changes)
	}

	// Keluarkan anggota. Target hanya memperoleh izin TELLER lewat keanggotaan ini,
	// sehingga penghapusan mencabut akses efektifnya: tanpa konfirmasi harus ditolak.
	if _, err := rig.perm.RequestChange(ctx, domain.PermissionChangeInput{
		GroupCode: group,
		Operation: domain.PermissionChangeRemoveMember,
		UserID:    targetID.String(),
	}, maker); err == nil {
		t.Fatal("penghapusan anggota berdampak diterima tanpa konfirmasi; seharusnya ditolak")
	} else {
		var loss *domain.AccessLossError
		if !errors.As(err, &loss) {
			t.Fatalf("err = %v, mau AccessLossError", err)
		}
	}
	removeReq, err := rig.perm.RequestChange(ctx, domain.PermissionChangeInput{
		GroupCode:         group,
		Operation:         domain.PermissionChangeRemoveMember,
		UserID:            targetID.String(),
		ConfirmAccessLoss: true,
	}, maker)
	if err != nil {
		t.Fatalf("mengajukan keluar anggota: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM maker_checker_requests WHERE id=$1`, removeReq.ID)
	})
	if err := rig.mc.Approve(ctx, removeReq.ID, checker, "uji"); err != nil {
		t.Fatalf("menyetujui keluar anggota: %v", err)
	}
	inGroup, err = repo.UserInGroup(ctx, group, targetID)
	if err != nil {
		t.Fatalf("memeriksa keanggotaan setelah keluar: %v", err)
	}
	if inGroup {
		t.Fatal("anggota masih di grup setelah persetujuan keluar")
	}

	// Pengguna tak dikenal ditolak sebelum masuk antrean.
	if _, err := rig.perm.RequestChange(ctx, domain.PermissionChangeInput{
		GroupCode: group,
		Operation: domain.PermissionChangeAddMember,
		UserID:    "00000000-0000-0000-0000-000000000000",
	}, maker); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("pengguna tak dikenal: err = %v, ingin ErrUserNotFound", err)
	}
}

// TestIntegrasiRemoveMemberTanpaDampakTidakPerluKonfirmasi membuktikan penjaga
// kehilangan akses TIDAK memblokir penghapusan yang tidak berdampak: SUPERADMIN
// sudah memiliki seluruh izin TELLER lewat perannya, sehingga dikeluarkan dari grup
// TELLER tidak mencabut apa pun dan tidak menuntut konfirmasi.
func TestIntegrasiRemoveMemberTanpaDampakTidakPerluKonfirmasi(t *testing.T) {
	db, ctx := newPermissionDB(t)
	rig := newPermissionRig(t, db, ctx)

	makerID, _ := seedTempUser(t, db, ctx, domain.RoleAdmin)
	checkerID, _ := seedTempUser(t, db, ctx, domain.RoleSuperAdmin)
	targetID, _ := seedTempUser(t, db, ctx, domain.RoleSuperAdmin)
	maker := actorFor(makerID, "pembuat-tanpa-dampak", domain.RoleAdmin)
	checker := actorFor(checkerID, "pemeriksa-tanpa-dampak", domain.RoleSuperAdmin)

	const group = "ROLE_TELLER"
	// Jadikan target anggota grup secara langsung: yang diuji adalah jalur
	// penghapusan, bukan penambahannya.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO user_group_members (user_id, group_id)
		SELECT $1, id FROM user_groups WHERE code = $2
		ON CONFLICT DO NOTHING`, targetID, group); err != nil {
		t.Fatalf("menyiapkan keanggotaan: %v", err)
	}

	// Tanpa konfirmasi pun diterima karena peran SUPERADMIN sudah menutup semua
	// izin grup TELLER.
	req, err := rig.perm.RequestChange(ctx, domain.PermissionChangeInput{
		GroupCode: group,
		Operation: domain.PermissionChangeRemoveMember,
		UserID:    targetID.String(),
	}, maker)
	if err != nil {
		t.Fatalf("penghapusan tanpa dampak tidak boleh menuntut konfirmasi: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM maker_checker_requests WHERE id=$1`, req.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_group_members WHERE user_id=$1`, targetID)
	})
	if err := rig.mc.Approve(ctx, req.ID, checker, "uji"); err != nil {
		t.Fatalf("menyetujui penghapusan tanpa dampak: %v", err)
	}
	if inGroup, err := postgres.NewPermissionRepository(db).UserInGroup(ctx, group, targetID); err != nil {
		t.Fatalf("memeriksa keanggotaan: %v", err)
	} else if inGroup {
		t.Fatal("anggota masih di grup setelah penghapusan disetujui")
	}
}

// TestIntegrasiTemuanE2EUnfreezeRekening membuktikan jalur keluar dari FROZEN ada:
// rekening dibekukan menjadi ACTIVE lagi lewat service nyata, teraudit, dan
// rekening berstatus lain ditolak, bukan diam-diam diaktifkan.
func TestIntegrasiTemuanE2EUnfreezeRekening(t *testing.T) {
	e := newActorBranchEnv(t)
	actor := findingsSuperadmin(e)
	cust := seedFindingsCustomer(t, e, "Nasabah Unfreeze")

	product, err := e.money.productRepo.GetByCode(e.money.ctx, "TAB-CONV")
	if err != nil {
		t.Fatalf("membaca produk tabungan: %v", err)
	}
	acc, err := e.accountSvc.OpenAccount(e.money.ctx, domain.OpenAccountInput{
		CustomerID: cust.ID, ProductID: product.ID, Currency: "IDR",
	}, actor)
	if err != nil {
		t.Fatalf("membuka rekening: %v", err)
	}
	// Tidak ada rute pembekuan di API; FROZEN disiapkan langsung di database untuk
	// menguji jalur keluarnya, lalu dibersihkan otomatis.
	if _, err := e.money.db.ExecContext(e.money.ctx,
		`UPDATE accounts SET status='FROZEN' WHERE id=$1`, acc.ID); err != nil {
		t.Fatalf("membekukan rekening uji: %v", err)
	}

	unfrozen, err := e.accountSvc.UnfreezeAccount(e.money.ctx, acc.AccountNumber, "uji unfreeze", actor)
	if err != nil {
		t.Fatalf("membatalkan pembekuan: %v", err)
	}
	if unfrozen.Status != domain.AccountStatusActive {
		t.Fatalf("status = %q, ingin ACTIVE", unfrozen.Status)
	}
	var statusAfter string
	if err := e.money.db.QueryRowContext(e.money.ctx,
		`SELECT status FROM accounts WHERE id=$1`, acc.ID).Scan(&statusAfter); err != nil {
		t.Fatalf("membaca status tersimpan: %v", err)
	}
	if statusAfter != string(domain.AccountStatusActive) {
		t.Fatalf("status tersimpan = %q, ingin ACTIVE", statusAfter)
	}
	var action string
	if err := e.money.db.QueryRowContext(e.money.ctx, `
		SELECT action FROM audit_logs
		WHERE resource_type='account' AND resource_id=$1 AND action='UNFREEZE_ACCOUNT'
		ORDER BY created_at DESC LIMIT 1`, acc.ID).Scan(&action); err != nil {
		t.Fatalf("audit UNFREEZE_ACCOUNT tidak ditemukan: %v", err)
	}

	// Sudah ACTIVE: unfreeze ulang ditolak (bukan sukses diam-diam).
	if _, err := e.accountSvc.UnfreezeAccount(e.money.ctx, acc.AccountNumber, "", actor); !errors.Is(err, domain.ErrAccountNotFrozen) {
		t.Fatalf("unfreeze rekening ACTIVE: err = %v, ingin ErrAccountNotFrozen", err)
	}
}
