package service

import (
	"context"
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
	repo      domain.CustomerRepository
	cipher    *crypto.Cipher
	cifSource domain.CIFGenerator
}

func NewCustomerService(repo domain.CustomerRepository, cipher *crypto.Cipher, cifSource domain.CIFGenerator) domain.CustomerService {
	return &customerService{repo: repo, cipher: cipher, cifSource: cifSource}
}

// cipherOrError memastikan kunci enkripsi tersedia sebelum menyentuh data nasabah.
func (s *customerService) cipherOrError() error {
	if s.cipher == nil {
		return domain.ErrCipherNotConfigured
	}
	return nil
}

func (s *customerService) nextCIF() string {
	if s.cifSource != nil {
		return s.cifSource.NextCIF()
	}
	return fmt.Sprintf("CIF-TMP-%s", uuid.New().String()[:8])
}

func (s *customerService) RegisterCustomer(ctx context.Context, input domain.CreateCustomerInput, actor domain.Actor) (*domain.Customer, error) {
	if err := s.cipherOrError(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.FullName) == "" {
		return nil, fmt.Errorf("nama lengkap wajib diisi")
	}
	if strings.TrimSpace(input.IDCardNumber) == "" {
		return nil, fmt.Errorf("NIK wajib diisi")
	}

	record, err := s.encryptInput(input)
	if err != nil {
		return nil, err
	}
	record.ID = uuid.New()
	record.CIFNumber = s.nextCIF()
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

	if err := s.repo.Create(ctx, record); err != nil {
		return nil, err
	}

	return s.decryptRecord(ctx, record)
}

func (s *customerService) GetCustomer(ctx context.Context, id uuid.UUID, actor domain.Actor) (*domain.Customer, error) {
	record, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.decryptRecord(ctx, record)
}

func (s *customerService) ListCustomers(ctx context.Context, page, pageSize int, actor domain.Actor) ([]domain.Customer, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	records, total, err := s.repo.List(ctx, pageSize, offset)
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
