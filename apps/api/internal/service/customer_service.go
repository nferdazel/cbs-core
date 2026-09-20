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

// resolveBranchID memetakan kode cabang aktor ke id cabang. Kode kosong atau
// database yang tidak tersedia (mis. unit test tanpa DB) menghasilkan nil:
// kolom branch_id nullable dan migrasi backfill yang mengisi data lama. Kode
// cabang yang tidak dikenal ditolak agar nasabah tidak tersimpan tanpa cabang
// yang sah.
func (s *customerService) resolveBranchID(ctx context.Context, branchCode string) (*uuid.UUID, error) {
	if branchCode == "" || s.db == nil {
		return nil, nil
	}
	var id uuid.UUID
	err := s.db.QueryRowContext(ctx, `SELECT id FROM branches WHERE code = $1`, branchCode).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrBranchNotFound
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
	branchID, err := s.resolveBranchID(ctx, actor.BranchCode)
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
	// dan tidak bergantung pada pesan unique violation dari database.
	if existing, err := s.repo.FindByIDCard(ctx, record.IDCardIndex); err == nil && existing != nil {
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
	defer tx.Rollback()

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
// CustomerRecord hanya menyimpan branch_id, bukan kode cabang, jadi id cabang
// aktor dipetakan lewat resolveBranchID. Mengikuti semantik Actor.CanAccessBranch:
// aktor lintas cabang selalu boleh, dan nasabah tanpa cabang (branch_id NULL, data
// pra-migrasi) tetap boleh dibaca agar operasional atas data lama tidak terblokir.
func (s *customerService) canReadRecord(ctx context.Context, actor domain.Actor, record *domain.CustomerRecord) (bool, error) {
	if actor.IsCrossBranch() || record.BranchID == nil {
		return true, nil
	}
	if actor.BranchCode == "" {
		return false, nil
	}
	branchID, err := s.resolveBranchID(ctx, actor.BranchCode)
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

// searchQuery menerjemahkan satu kata kunci menjadi filter repo. Kata kunci yang
// berbentuk NIK (tepat 16 digit) dicari lewat blind index NIK SAJA: nomor CIF tidak
// pernah berbentuk 16 digit, sehingga menggabungkan keduanya dengan AND akan
// menyaring habis hasilnya. Bentuk lain dicari sebagai awalan nomor CIF.
func (s *customerService) searchQuery(term string) domain.CustomerQuery {
	term = strings.TrimSpace(term)
	if term == "" {
		return domain.CustomerQuery{}
	}
	if isNIK(term) && s.cipher != nil {
		return domain.CustomerQuery{IDCardIndex: s.cipher.BlindIndex(term)}
	}
	return domain.CustomerQuery{CIF: term}
}

// isNIK melaporkan apakah kata kunci berbentuk NIK: tepat 16 digit angka.
func isNIK(term string) bool {
	if len(term) != 16 {
		return false
	}
	for i := 0; i < len(term); i++ {
		if term[i] < '0' || term[i] > '9' {
			return false
		}
	}
	return true
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
	}
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
