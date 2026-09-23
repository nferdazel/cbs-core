package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// CustomerRepository menyimpan data nasabah apa adanya. Nilai pribadi sudah
// terenkripsi saat mencapai lapisan ini; repository tidak memegang kunci maupun
// mengetahui plaintext. Pencarian NIK/email memakai blind index.
type CustomerRepository struct {
	db *sql.DB
}

func NewCustomerRepository(db *sql.DB) *CustomerRepository {
	return &CustomerRepository{db: db}
}

const customerColumns = `id, cif_number, full_name_enc, id_card_number_enc, email_enc,
	phone_number_enc, address_enc, id_card_index, email_index, index_key_version,
	status, branch_id, metadata, created_at, updated_at`

func (r *CustomerRepository) Create(ctx context.Context, c *domain.CustomerRecord) error {
	return r.executeCreate(ctx, r.db, c)
}

// CreateTx menyimpan nasabah di dalam transaksi pemanggil, sehingga pendaftaran
// nasabah dan audit log-nya commit bersama.
func (r *CustomerRepository) CreateTx(ctx context.Context, tx *sql.Tx, c *domain.CustomerRecord) error {
	return r.executeCreate(ctx, tx, c)
}

func (r *CustomerRepository) executeCreate(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, c *domain.CustomerRecord) error {
	metaJSON, err := json.Marshal(c.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	query := `
		INSERT INTO customers (
			id, cif_number, full_name_enc, id_card_number_enc, email_enc,
			phone_number_enc, address_enc, id_card_index, email_index, index_key_version,
			status, branch_id, metadata, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	// Blind index kosong disimpan sebagai NULL, bukan string kosong. Indeks unik
	// email_index bersifat parsial (WHERE email_index IS NOT NULL); menyimpan ""
	// akan membuat nasabah kedua tanpa email bertabrakan. Email opsional, jadi kolom
	// ini harus benar-benar kosong saat tidak ada.
	idCardIndex := sql.NullString{String: c.IDCardIndex, Valid: c.IDCardIndex != ""}
	emailIndex := sql.NullString{String: c.EmailIndex, Valid: c.EmailIndex != ""}
	// Baris yang belum mencatat versi (pemanggil lama) dianggap memakai kunci
	// enkripsi bawaan "k1", sama dengan nilai DEFAULT kolom di migrasi 000048.
	indexKeyVersion := c.IndexKeyVersion
	if indexKeyVersion == "" {
		indexKeyVersion = "k1"
	}
	_, err = exec.ExecContext(ctx, query,
		c.ID, c.CIFNumber, c.FullNameEnc, c.IDCardNumberEnc, c.EmailEnc,
		c.PhoneNumberEnc, c.AddressEnc, idCardIndex, emailIndex, indexKeyVersion,
		c.Status, c.BranchID, metaJSON, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert customer: %w", err)
	}
	// Token nama ditulis lewat handler exec yang sama, sehingga pada CreateTx token
	// ikut dalam transaksi yang sama dengan nasabahnya (atomik).
	if err := insertNameTokens(ctx, exec, c.ID, c.NameTokenIndexes, indexKeyVersion); err != nil {
		return err
	}
	return nil
}

// insertNameTokens menyimpan indeks tiap kata nama. ON CONFLICT DO NOTHING membuat
// operasi ini aman terhadap pengulangan (mis. backfill) dan terhadap kata kembar.
func insertNameTokens(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, customerID uuid.UUID, tokenIndexes []string, indexKeyVersion string) error {
	for _, index := range tokenIndexes {
		if index == "" {
			continue
		}
		_, err := exec.ExecContext(ctx,
			`INSERT INTO customer_name_tokens (customer_id, token_index, index_key_version) VALUES ($1, $2, $3)
			 ON CONFLICT (customer_id, token_index) DO NOTHING`,
			customerID, index, indexKeyVersion,
		)
		if err != nil {
			return fmt.Errorf("failed to insert customer name token: %w", err)
		}
	}
	return nil
}

// ReplaceNameTokens menghapus token lama nasabah lalu menulis token baru di dalam
// transaksi pemanggil. Hapus-lalu-isi diperlukan karena nama yang diubah bisa
// membuang kata lama; tanpa ini token lama tetap mencocokkan nasabah pada kata yang
// sudah tidak ada di namanya.
func (r *CustomerRepository) ReplaceNameTokens(ctx context.Context, tx *sql.Tx, customerID uuid.UUID, tokenIndexes []string, indexKeyVersion string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM customer_name_tokens WHERE customer_id = $1`, customerID); err != nil {
		return fmt.Errorf("failed to delete old customer name tokens: %w", err)
	}
	return insertNameTokens(ctx, tx, customerID, tokenIndexes, indexKeyVersion)
}

// UpdateTx memperbarui data terenkripsi nasabah dan blind index-nya, lalu mengganti
// token nama, semuanya di dalam transaksi pemanggil. Blind index kosong disimpan
// sebagai NULL mengikuti alasan yang sama seperti Create (indeks email parsial unik).
func (r *CustomerRepository) UpdateTx(ctx context.Context, tx *sql.Tx, c *domain.CustomerRecord) error {
	metaJSON, err := json.Marshal(c.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	idCardIndex := sql.NullString{String: c.IDCardIndex, Valid: c.IDCardIndex != ""}
	emailIndex := sql.NullString{String: c.EmailIndex, Valid: c.EmailIndex != ""}
	indexKeyVersion := c.IndexKeyVersion
	if indexKeyVersion == "" {
		indexKeyVersion = "k1"
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE customers
		SET full_name_enc = $2, id_card_number_enc = $3, email_enc = $4,
		    phone_number_enc = $5, address_enc = $6, id_card_index = $7,
		    email_index = $8, index_key_version = $9, metadata = $10, updated_at = $11
		WHERE id = $1`,
		c.ID, c.FullNameEnc, c.IDCardNumberEnc, c.EmailEnc,
		c.PhoneNumberEnc, c.AddressEnc, idCardIndex, emailIndex, indexKeyVersion,
		metaJSON, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update customer: %w", err)
	}
	return r.ReplaceNameTokens(ctx, tx, c.ID, c.NameTokenIndexes, indexKeyVersion)
}

func (r *CustomerRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.CustomerRecord, error) {
	query := `SELECT ` + customerColumns + ` FROM customers WHERE id = $1`
	return r.scanOne(ctx, query, id)
}

func (r *CustomerRepository) GetByCIF(ctx context.Context, cif string) (*domain.CustomerRecord, error) {
	query := `SELECT ` + customerColumns + ` FROM customers WHERE cif_number = $1`
	return r.scanOne(ctx, query, cif)
}

// FindByIDCard mencari NIK lewat kandidat blind index lintas versi kunci. ANY
// memastikan NIK yang terdaftar dengan kunci indeks lama tetap terdeteksi sebagai
// duplikat saat kunci indeks sudah diganti.
func (r *CustomerRepository) FindByIDCard(ctx context.Context, idCardIndexes []string) (*domain.CustomerRecord, error) {
	if len(idCardIndexes) == 0 {
		return nil, domain.ErrCustomerNotFound
	}
	query := `SELECT ` + customerColumns + ` FROM customers WHERE id_card_index = ANY($1)`
	return r.scanOne(ctx, query, idCardIndexes)
}

func (r *CustomerRepository) List(ctx context.Context, limit, offset int, q domain.CustomerQuery, actor domain.Actor) ([]domain.CustomerRecord, int, error) {
	where, whereArgs := branchReadClause("branch_id", actor)

	// Pencarian digabung dengan filter cabang, dan COUNT memakai klausa yang sama
	// sehingga total pagination tetap benar saat pencarian aktif.
	if pattern := likePrefixPattern(q.CIF); pattern != "" {
		whereArgs = append(whereArgs, pattern)
		where = andCondition(where, fmt.Sprintf("cif_number ILIKE $%d ESCAPE '\\'", len(whereArgs)))
	}
	// Kandidat lintas versi kunci indeks dikirim sebagai satu array; ANY mencocokkan
	// baris yang diindeks dengan kunci lama maupun baru sekaligus.
	if len(q.IDCardIndexes) > 0 {
		whereArgs = append(whereArgs, q.IDCardIndexes)
		where = andCondition(where, fmt.Sprintf("id_card_index = ANY($%d)", len(whereArgs)))
	}
	// Pencarian nama: satu kondisi EXISTS per kata, digabung AND, sehingga semua
	// kata yang diketik harus ada pada nama nasabah yang sama.
	where, whereArgs = addNameTokenFilters(where, whereArgs, q.NameTokenIndexes)

	countQuery := "SELECT COUNT(*) FROM customers"
	if where != "" {
		countQuery += " WHERE " + where
	}
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + customerColumns + ` FROM customers`
	if where != "" {
		query += " WHERE " + where
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(whereArgs)+1, len(whereArgs)+2)
	args := append(append([]any{}, whereArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var customers []domain.CustomerRecord
	for rows.Next() {
		rec, err := scanCustomer(rows)
		if err != nil {
			return nil, 0, err
		}
		customers = append(customers, *rec)
	}
	return customers, total, rows.Err()
}

func (r *CustomerRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.CustomerStatus) error {
	_, err := r.db.ExecContext(ctx, "UPDATE customers SET status = $1, updated_at = NOW() WHERE id = $2", status, id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *CustomerRepository) scanOne(ctx context.Context, query string, arg any) (*domain.CustomerRecord, error) {
	rec, err := scanCustomer(r.db.QueryRowContext(ctx, query, arg))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}
	return rec, nil
}

func scanCustomer(row rowScanner) (*domain.CustomerRecord, error) {
	var c domain.CustomerRecord
	var metaBytes []byte
	var fullName, idCard, email, phone, address, idCardIdx, emailIdx sql.NullString

	if err := row.Scan(
		&c.ID, &c.CIFNumber, &fullName, &idCard, &email, &phone, &address,
		&idCardIdx, &emailIdx, &c.IndexKeyVersion, &c.Status, &c.BranchID, &metaBytes, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}

	c.FullNameEnc = fullName.String
	c.IDCardNumberEnc = idCard.String
	c.EmailEnc = email.String
	c.PhoneNumberEnc = phone.String
	c.AddressEnc = address.String
	c.IDCardIndex = idCardIdx.String
	c.EmailIndex = emailIdx.String

	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &c.Metadata)
	}
	return &c, nil
}
