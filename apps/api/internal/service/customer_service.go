package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// customerService menangani data pribadi nasabah. Nilai pribadi dienkripsi sebelum
// disimpan dan baru didekripsi saat dibaca; repository tidak pernah melihat plaintext.
type customerService struct {
	db        *sql.DB
	repo      domain.CustomerRepository
	cipher    *crypto.Cipher
	cifSource domain.CIFGenerator
	auditRepo domain.AuditRepository
}

func NewCustomerService(db *sql.DB, repo domain.CustomerRepository, cipher *crypto.Cipher, cifSource domain.CIFGenerator, auditSinks ...domain.AuditRepository) domain.CustomerService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &customerService{db: db, repo: repo, cipher: cipher, cifSource: cifSource, auditRepo: auditRepo}
}

// cipherOrError memastikan kunci enkripsi tersedia sebelum menyentuh data nasabah.
func (s *customerService) cipherOrError() error {
	if s.cipher == nil {
		return domain.ErrCipherNotConfigured
	}
	return nil
}

// nextCIF menerbitkan nomor CIF. Kegagalan sequence diteruskan sebagai error,
// tidak ada nomor cadangan: CIF ganda menyatukan dua nasabah berbeda.
func (s *customerService) nextCIF() (string, error) {
	if s.cifSource == nil {
		return "", errors.New("generator nomor CIF belum dikonfigurasi")
	}
	return s.cifSource.NextCIF()
}

// resolveBranchID memetakan cabang efektif aktor ke id cabang. Database yang
// tidak tersedia (mis. unit test tanpa DB) menghasilkan nil.
//
// Aktor lintas cabang (SUPERADMIN/AUDITOR/SYSTEM) bertindak atas nama kantor
// pusat: kode cabangnya tidak wajib terdaftar, karena kode 'HO' pada akun kantor
// pusat bukan cabang operasional. Bila kodenya kosong atau tidak terdaftar,
// nasabah diatribusikan ke cabang kantor pusat (is_head_office) bila ada, agar
// tidak tersimpan tanpa cabang; bila kantor pusat tidak ada, branch_id NULL tetap
// sah karena kolomnya nullable. Peran bercabang biasa tetap ditolak bila kode
// cabangnya tidak terdaftar (lihat Actor.RequiresRegisteredBranch): nasabah tidak
// boleh tersimpan di cabang yang tidak sah.
func (s *customerService) resolveBranchID(ctx context.Context, actor domain.Actor) (*uuid.UUID, error) {
	if s.db == nil {
		return nil, nil
	}
	code := strings.TrimSpace(actor.BranchCode)
	if code != "" {
		var id uuid.UUID
		err := s.db.QueryRowContext(ctx, `SELECT id FROM branches WHERE code = $1`, code).Scan(&id)
		if err == nil {
			return &id, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if actor.RequiresRegisteredBranch() {
			return nil, domain.ErrBranchNotFound
		}
	}
	if !actor.IsCrossBranch() {
		return nil, nil
	}
	return s.headOfficeBranchID(ctx)
}

// headOfficeBranchID mengembalikan id cabang kantor pusat (is_head_office), atau
// nil bila belum ada. Dipakai untuk diatribusikan ke aktor lintas cabang yang
// tidak punya cabang operasional terdaftar.
func (s *customerService) headOfficeBranchID(ctx context.Context) (*uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRowContext(ctx, `SELECT id FROM branches WHERE is_head_office = TRUE ORDER BY code LIMIT 1`).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

func (s *customerService) RegisterCustomer(ctx context.Context, input domain.CreateCustomerInput, actor domain.Actor) (*domain.Customer, error) {
	if err := s.cipherOrError(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.FullName) == "" {
		return nil, fmt.Errorf("nama lengkap wajib diisi")
	}
	// NIK dinormalisasi SEBELUM dienkripsi dan diindeks, agar blind index-nya
	// konsisten dengan yang dipakai saat pencarian.
	normalizedIDCard, err := domain.NormalizeIDCardNumber(input.IDCardNumber)
	if err != nil {
		return nil, err
	}
	input.IDCardNumber = normalizedIDCard

	record, err := s.encryptInput(input)
	if err != nil {
		return nil, err
	}
	// Cabang nasabah HANYA dari JWT; branch_code pada body diabaikan. Service ini
	// tidak memegang BranchRepository (konstruktornya dipakai main.go yang tidak
	// boleh diubah), jadi pemetaan kode->id dibaca langsung dari tabel branches.
	branchID, err := s.resolveBranchID(ctx, actor)
	if err != nil {
		return nil, err
	}
	cifNumber, err := s.nextCIF()
	if err != nil {
		return nil, err
	}
	record.ID = uuid.New()
	record.CIFNumber = cifNumber
	record.BranchID = branchID
	record.Status = domain.CustomerStatusActive
	record.Metadata = input.Metadata
	record.CreatedAt = time.Now().UTC()
	record.UpdatedAt = record.CreatedAt

	// Deteksi duplikat lewat blind index sebelum insert, agar pesannya jelas
	// dan tidak bergantung pada pesan unique violation dari database. Kandidat
	// lintas versi dipakai agar NIK yang terdaftar dengan kunci indeks lama tetap
	// terdeteksi setelah kunci diganti.
	if existing, err := s.repo.FindByIDCard(ctx, s.cipher.BlindIndexCandidates(input.IDCardNumber)); err == nil && existing != nil {
		return nil, domain.ErrDuplicateIDCard
	} else if err != nil && !errors.Is(err, domain.ErrCustomerNotFound) {
		return nil, err
	}

	if err := s.persistCustomer(ctx, record, actor); err != nil {
		return nil, err
	}

	return s.decryptRecord(ctx, record)
}

// persistCustomer menyimpan nasabah dan audit log dalam satu transaksi bila database
// tersedia. Tanpa database (mis. test unit), penyimpanan dilakukan langsung.
func (s *customerService) persistCustomer(ctx context.Context, record *domain.CustomerRecord, actor domain.Actor) error {
	changes := map[string]any{
		"cif_number": record.CIFNumber,
		"status":     record.Status,
	}

	if s.db == nil {
		if err := s.repo.Create(ctx, record); err != nil {
			if isUniqueViolation(err) {
				return domain.ErrDuplicateIDCard
			}
			return err
		}
		return writeAudit(ctx, s.auditRepo, nil, actor, "REGISTER_CUSTOMER", "customer", record.ID.String(), changes)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.CreateTx(ctx, tx, record); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateIDCard
		}
		return err
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "REGISTER_CUSTOMER", "customer", record.ID.String(), changes); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateCustomer mengubah data nasabah yang sudah ada. NIK dinormalisasi ulang lalu
// dideteksi duplikatnya lewat blind index lintas versi kunci, sehingga NIK yang
// diubah tidak menabrak nasabah lain tanpa pesan yang jelas. CIF, status, dan cabang
// dipertahankan; cabang nasabah di luar cakupan unit aktor ditolak.
func (s *customerService) UpdateCustomer(ctx context.Context, id uuid.UUID, input domain.UpdateCustomerInput, actor domain.Actor) (*domain.Customer, error) {
	if err := s.cipherOrError(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.FullName) == "" {
		return nil, fmt.Errorf("nama lengkap wajib diisi")
	}
	normalizedIDCard, err := domain.NormalizeIDCardNumber(input.IDCardNumber)
	if err != nil {
		return nil, err
	}
	input.IDCardNumber = normalizedIDCard

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.db != nil {
		allowed, err := s.canReadRecord(ctx, actor, existing)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, domain.ErrCrossBranchAccess
		}
	}

	// Duplikat NIK dicek lewat blind index. Baris yang ditemukan adalah dirinya
	// sendiri saat NIK tidak berubah, sehingga aman diperiksa selalu.
	if found, err := s.repo.FindByIDCard(ctx, s.cipher.BlindIndexCandidates(input.IDCardNumber)); err == nil && found != nil && found.ID != id {
		return nil, domain.ErrDuplicateIDCard
	} else if err != nil && !errors.Is(err, domain.ErrCustomerNotFound) {
		return nil, err
	}

	record, err := s.encryptInput(domain.CreateCustomerInput{
		FullName:     input.FullName,
		IDCardNumber: input.IDCardNumber,
		Email:        input.Email,
		PhoneNumber:  input.PhoneNumber,
		Address:      input.Address,
	})
	if err != nil {
		return nil, err
	}
	record.ID = id
	record.CIFNumber = existing.CIFNumber
	record.Status = existing.Status
	record.BranchID = existing.BranchID
	record.Metadata = input.Metadata
	record.CreatedAt = existing.CreatedAt
	record.UpdatedAt = time.Now().UTC()

	if err := s.persistUpdate(ctx, record, actor); err != nil {
		return nil, err
	}
	return s.decryptRecord(ctx, record)
}

// persistUpdate menyimpan perubahan nasabah dan audit log dalam satu transaksi.
// Perubahan data, blind index, dan token nama harus commit bersama: tanpa itu,
// pencarian nama/NIK bisa menunjuk data yang sudah berubah.
func (s *customerService) persistUpdate(ctx context.Context, record *domain.CustomerRecord, actor domain.Actor) error {
	if s.db == nil {
		return errors.New("pembaruan nasabah memerlukan database")
	}
	changes := map[string]any{
		"cif_number": record.CIFNumber,
		"status":     record.Status,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.UpdateTx(ctx, tx, record); err != nil {
		// Unique violation pada index NIK/email bermakna nasabah lain sudah memakai
		// nilai itu; pesannya sama dengan jalur pendaftaran.
		if isUniqueViolation(err) {
			return domain.ErrDuplicateIDCard
		}
		return err
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "UPDATE_CUSTOMER", "customer", record.ID.String(), changes); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *customerService) GetCustomer(ctx context.Context, id uuid.UUID, actor domain.Actor) (*domain.Customer, error) {
	record, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	allowed, err := s.canReadRecord(ctx, actor, record)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, domain.ErrCrossBranchAccess
	}
	return s.decryptRecord(ctx, record)
}

// canReadRecord menegakkan kepemilikan cabang untuk pembacaan satu nasabah.
// CustomerRecord hanya menyimpan branch_id, bukan kode cabang. Bila cakupan unit
// aktor sudah diresolusi (W14), kode cabang baris dipetakan lewat branches lalu
// diperiksa Actor.CanAccessBranch sehingga area/wilayah mencakup cabang di
// bawahannya; tanpa resolusi, perilaku lama dipertahankan (kesamaan id cabang).
// Nasabah tanpa cabang (branch_id NULL, data pra-migrasi) tetap boleh dibaca agar
// operasional atas data lama tidak terblokir.
func (s *customerService) canReadRecord(ctx context.Context, actor domain.Actor, record *domain.CustomerRecord) (bool, error) {
	if actor.IsCrossBranch() || record.BranchID == nil {
		return true, nil
	}
	if actor.BranchCode == "" {
		return false, nil
	}
	// Cakupan unit sudah diresolusi (W14): area/wilayah mencakup cabang di
	// bawahannya, jadi akses ditentukan Actor.CanAccessBranch atas kode cabang
	// baris, bukan kesamaan id cabang aktor. Satu baris tetap dipetakan ke kode
	// lewat branches agar sumbu cakupan hanya satu.
	if actor.HasResolvedBranchScope() {
		var code string
		if err := s.db.QueryRowContext(ctx, `SELECT code FROM branches WHERE id = $1`, *record.BranchID).Scan(&code); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		return actor.CanAccessBranch(code), nil
	}
	branchID, err := s.resolveBranchID(ctx, actor)
	if err != nil {
		return false, err
	}
	return branchID != nil && *branchID == *record.BranchID, nil
}

func (s *customerService) ListCustomers(ctx context.Context, page, pageSize int, search string, actor domain.Actor) ([]domain.Customer, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	records, total, err := s.repo.List(ctx, pageSize, offset, s.searchQuery(search), actor)
	if err != nil {
		return nil, 0, err
	}

	customers := make([]domain.Customer, 0, len(records))
	for i := range records {
		c, err := s.decryptRecord(ctx, &records[i])
		if err != nil {
			// Satu record rusak tidak boleh menggagalkan seluruh daftar; catat dan lewati.
			slog.ErrorContext(ctx, "gagal mendekripsi data nasabah", "customer_id", records[i].ID, "error", err)
			continue
		}
		customers = append(customers, *c)
	}
	return customers, total, nil
}

// searchQuery menerjemahkan satu kata kunci menjadi filter repo. Bentuknya
// ditentukan agar setiap masukan jatuh ke satu mode pencarian saja:
//   - NIK (tepat 16 digit) dicari lewat blind index NIK;
//   - bentuk CIF ("CIF..." atau angka) dicari sebagai awalan nomor CIF;
//   - sisanya dianggap potongan nama dan dipecah menjadi token (AND antar kata).
//
// Menggabungkan mode dengan AND akan menyaring habis hasilnya (mis. nama yang
// jarang diawali "CIF"), jadi tiap kata kunci hanya memakai satu mode.
func (s *customerService) searchQuery(term string) domain.CustomerQuery {
	term = strings.TrimSpace(term)
	if term == "" {
		return domain.CustomerQuery{}
	}
	if isNIK(term) {
		if s.cipher != nil {
			// Kandidat semua versi kunci indeks: baris dengan kunci lama tetap
			// ditemukan meski kunci indeks aktif sudah diganti.
			return domain.CustomerQuery{IDCardIndexes: s.cipher.BlindIndexCandidates(term)}
		}
		// Tanpa cipher NIK tidak bisa diindeks; jatuh ke awalan CIF apa adanya agar
		// tidak memaksa hash yang tidak akan pernah cocok.
		return domain.CustomerQuery{CIF: term}
	}
	if looksLikeCIF(term) {
		return domain.CustomerQuery{CIF: term}
	}
	// Tanpa cipher token nama tidak dapat dihitung; perlakukan sebagai CIF seperti
	// perilaku lama alih-alih mengembalikan hasil kosong yang menyesatkan.
	if s.cipher == nil {
		return domain.CustomerQuery{CIF: term}
	}
	candidates := s.nameTokenCandidates(term)
	if len(candidates) == 0 {
		return domain.CustomerQuery{}
	}
	return domain.CustomerQuery{NameTokenIndexes: candidates}
}

// nameTokenIndexes menormalkan nama menjadi kata lalu menghitung blind index tiap
// kata dengan kunci indeks AKTIF, untuk ditulis ke tabel token. Nil mengembalikan
// nil bila cipher tidak ada atau nama tidak punya kata bermakna, sehingga tidak ada
// token sampah yang ditulis.
func (s *customerService) nameTokenIndexes(fullName string) []string {
	if s.cipher == nil {
		return nil
	}
	tokens := domain.NormalizeNameTokens(fullName)
	if len(tokens) == 0 {
		return nil
	}
	indexes := make([]string, 0, len(tokens))
	for _, token := range tokens {
		indexes = append(indexes, s.cipher.NameTokenIndex(token))
	}
	return indexes
}

// nameTokenCandidates menghitung kandidat indeks tiap kata lintas versi kunci,
// dipakai saat pencarian agar baris lama dan baru menyatu.
func (s *customerService) nameTokenCandidates(fullName string) [][]string {
	if s.cipher == nil {
		return nil
	}
	tokens := domain.NormalizeNameTokens(fullName)
	if len(tokens) == 0 {
		return nil
	}
	candidates := make([][]string, 0, len(tokens))
	for _, token := range tokens {
		candidates = append(candidates, s.cipher.NameTokenIndexCandidates(token))
	}
	return candidates
}

// looksLikeCIF melaporkan apakah kata kunci berbentuk nomor CIF. CIF selalu
// diawali "CIF" (lihat ReferenceGenerator.NextCIF), tetapi pencarian CIF parsial
// berupa angka juga tetap didukung seperti perilaku lama.
func looksLikeCIF(term string) bool {
	return strings.HasPrefix(strings.ToUpper(term), "CIF") || isDigits(term)
}

// isDigits melaporkan apakah seluruh karakter adalah angka (dan tidak kosong).
func isDigits(term string) bool {
	if term == "" {
		return false
	}
	for i := 0; i < len(term); i++ {
		if term[i] < '0' || term[i] > '9' {
			return false
		}
	}
	return true
}

// isNIK melaporkan apakah kata kunci berbentuk NIK: tepat 16 digit angka.
func isNIK(term string) bool {
	return len(term) == domain.IDCardDigits && isDigits(term)
}

// NamesByIDs mengembalikan nama nasabah yang sudah didekripsi. Hanya field nama yang
// dibuka; NIK/email/telepon/alamat tidak perlu untuk pelengkapan tampilan. Record yang
// gagal didekripsi dilewati agar satu data rusak tidak menggagalkan seluruh daftar.
func (s *customerService) NamesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	names := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	if err := s.cipherOrError(); err != nil {
		return names, err
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	records, err := s.repo.GetByIDs(ctx, unique)
	if err != nil {
		return names, err
	}
	for id, rec := range records {
		if rec.FullNameEnc == "" {
			continue
		}
		name, err := s.cipher.Decrypt(rec.FullNameEnc)
		if err != nil {
			slog.ErrorContext(ctx, "gagal mendekripsi nama nasabah", "customer_id", id, "error", err)
			continue
		}
		names[id] = name
	}
	return names, nil
}

func (s *customerService) encryptInput(input domain.CreateCustomerInput) (*domain.CustomerRecord, error) {
	fullName, err := s.cipher.Encrypt(input.FullName)
	if err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi nama: %w", err)
	}
	idCard, err := s.cipher.Encrypt(input.IDCardNumber)
	if err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi NIK: %w", err)
	}
	email, err := s.cipher.Encrypt(input.Email)
	if err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi email: %w", err)
	}
	phone, err := s.cipher.Encrypt(input.PhoneNumber)
	if err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi telepon: %w", err)
	}
	address, err := s.cipher.Encrypt(input.Address)
	if err != nil {
		return nil, fmt.Errorf("gagal mengenkripsi alamat: %w", err)
	}

	record := &domain.CustomerRecord{
		FullNameEnc:     fullName,
		IDCardNumberEnc: idCard,
		EmailEnc:        email,
		PhoneNumberEnc:  phone,
		AddressEnc:      address,
		// Versi kunci indeks yang membentuk IDCardIndex/EmailIndex/NameTokenIndexes.
		// Dicatat per baris supaya rotasi kunci berikutnya tahu baris mana yang perlu
		// diindeks ulang.
		IndexKeyVersion: s.cipher.IndexKeyVersion(),
	}
	// Token nama ikut dihitung di sini agar tersedia sebelum insert; repository
	// menuliskannya dalam transaksi yang sama dengan nasabahnya.
	record.NameTokenIndexes = s.nameTokenIndexes(input.FullName)
	if strings.TrimSpace(input.IDCardNumber) != "" {
		record.IDCardIndex = s.cipher.BlindIndex(input.IDCardNumber)
	}
	if strings.TrimSpace(input.Email) != "" {
		record.EmailIndex = s.cipher.BlindIndex(input.Email)
	}
	return record, nil
}

func (s *customerService) decryptRecord(ctx context.Context, record *domain.CustomerRecord) (*domain.Customer, error) {
	fullName, err := s.cipher.Decrypt(record.FullNameEnc)
	if err != nil {
		return nil, fmt.Errorf("gagal mendekripsi nama nasabah %s: %w", record.ID, err)
	}
	idCard, err := s.cipher.Decrypt(record.IDCardNumberEnc)
	if err != nil {
		return nil, fmt.Errorf("gagal mendekripsi NIK nasabah %s: %w", record.ID, err)
	}
	email, err := s.cipher.Decrypt(record.EmailEnc)
	if err != nil {
		return nil, fmt.Errorf("gagal mendekripsi email nasabah %s: %w", record.ID, err)
	}
	phone, err := s.cipher.Decrypt(record.PhoneNumberEnc)
	if err != nil {
		return nil, fmt.Errorf("gagal mendekripsi telepon nasabah %s: %w", record.ID, err)
	}
	address, err := s.cipher.Decrypt(record.AddressEnc)
	if err != nil {
		return nil, fmt.Errorf("gagal mendekripsi alamat nasabah %s: %w", record.ID, err)
	}

	return &domain.Customer{
		ID:           record.ID,
		CIFNumber:    record.CIFNumber,
		FullName:     fullName,
		IDCardNumber: idCard,
		Email:        email,
		PhoneNumber:  phone,
		Address:      address,
		Status:       record.Status,
		BranchID:     record.BranchID,
		Metadata:     record.Metadata,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}, nil
}
