package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// PermissionRepository membaca & mengubah pemetaan grup/izin dan katalog menu.
// Semua perubahan nyata terjadi lewat transaksi maker-checker (lihat service),
// sehingga repository menyediakan varian *Tx.
type PermissionRepository struct {
	db *sql.DB
}

func NewPermissionRepository(db *sql.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

var _ domain.PermissionRepository = (*PermissionRepository)(nil)
var _ domain.ApprovalLimitRoleResolver = (*PermissionRepository)(nil)

// ListGroups mengembalikan seluruh grup beserta izinnya. Dua kueri (grup dan izin)
// lalu digabung di Go agar tidak ada kueri per grup.
func (r *PermissionRepository) ListGroups(ctx context.Context) ([]domain.UserGroup, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, code, name, COALESCE(description, ''), is_system, approval_limit_role
		FROM user_groups
		ORDER BY is_system DESC, code ASC`)
	if err != nil {
		return nil, fmt.Errorf("membaca grup: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groups := []domain.UserGroup{}
	index := map[string]int{}
	for rows.Next() {
		var g domain.UserGroup
		var limitRole sql.NullString
		if err := rows.Scan(&g.ID, &g.Code, &g.Name, &g.Description, &g.IsSystem, &limitRole); err != nil {
			return nil, err
		}
		if limitRole.Valid {
			g.ApprovalLimitRole = domain.StaffRole(limitRole.String)
		}
		index[g.Code] = len(groups)
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	permRows, err := r.db.QueryContext(ctx, `
		SELECT g.code, gp.permission
		FROM group_permissions gp
		JOIN user_groups g ON g.id = gp.group_id
		ORDER BY g.code, gp.permission`)
	if err != nil {
		return nil, fmt.Errorf("membaca izin grup: %w", err)
	}
	defer func() { _ = permRows.Close() }()
	for permRows.Next() {
		var code, permission string
		if err := permRows.Scan(&code, &permission); err != nil {
			return nil, err
		}
		if i, ok := index[code]; ok {
			groups[i].Permissions = append(groups[i].Permissions, domain.Permission(permission))
		}
	}
	if err := permRows.Err(); err != nil {
		return nil, err
	}

	// Anggota dibaca sekali untuk seluruh grup, lalu ditempelkan di Go. Halaman
	// pengelolaan izin memakainya untuk melihat siapa yang terdampak sebelum
	// pemetaan diubah; tanpa gambar anggota, perubahan berisiko tak terlihat.
	memberRows, err := r.db.QueryContext(ctx, `
		SELECT g.code, u.id, u.username, u.full_name, u.role::text, u.is_active
		FROM user_group_members m
		JOIN user_groups g ON g.id = m.group_id
		JOIN staff_users u ON u.id = m.user_id
		ORDER BY g.code, u.username`)
	if err != nil {
		return nil, fmt.Errorf("membaca anggota grup: %w", err)
	}
	defer func() { _ = memberRows.Close() }()
	for memberRows.Next() {
		var code string
		var m domain.GroupMember
		if err := memberRows.Scan(&code, &m.UserID, &m.Username, &m.FullName, &m.Role, &m.IsActive); err != nil {
			return nil, err
		}
		if i, ok := index[code]; ok {
			groups[i].Members = append(groups[i].Members, m)
		}
	}
	if err := memberRows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

// ListMenus mengembalikan katalog menu beserta izin yang membukanya. Menu tanpa
// baris izin tetap dikembalikan dengan daftar kosong (tampil untuk semua pengguna).
func (r *PermissionRepository) ListMenus(ctx context.Context) ([]domain.MenuDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT menu_key
		FROM menu_catalog
		ORDER BY menu_key ASC`)
	if err != nil {
		return nil, fmt.Errorf("membaca katalog menu: %w", err)
	}
	defer func() { _ = rows.Close() }()

	menus := []domain.MenuDefinition{}
	index := map[string]int{}
	for rows.Next() {
		var m domain.MenuDefinition
		if err := rows.Scan(&m.MenuKey); err != nil {
			return nil, err
		}
		index[m.MenuKey] = len(menus)
		menus = append(menus, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	permRows, err := r.db.QueryContext(ctx, `
		SELECT menu_key, permission FROM menu_permissions ORDER BY menu_key, permission`)
	if err != nil {
		return nil, fmt.Errorf("membaca izin menu: %w", err)
	}
	defer func() { _ = permRows.Close() }()
	for permRows.Next() {
		var menuKey, permission string
		if err := permRows.Scan(&menuKey, &permission); err != nil {
			return nil, err
		}
		if i, ok := index[menuKey]; ok {
			menus[i].Permissions = append(menus[i].Permissions, domain.Permission(permission))
		}
	}
	return menus, permRows.Err()
}

// ResolveApprovalLimitRole mengembalikan peran pada matriks limit 000051 yang
// menjadi jenjang kewenangan persetujuan pengguna. Sumbernya: grup peran bawaan
// (ROLE_<peran>, selalu berlaku) digabung keanggotaan grup lain (aditif). Bila
// beberapa grup menunjuk peran berbeda, peran dengan kewenangan tertinggi yang
// menang — sejalan dengan semantik izin efektif yang juga aditif.
//
// userID uuid.Nil (mis. pemanggilan List yang hanya punya peran) tetap membaca
// grup ROLE_<peran>; anggota grup lain tidak ada sehingga hasilnya peran itu.
// Tidak ada grup yang menunjuk peran valid -> peran pelaku dikembalikan apa adanya.
func (r *PermissionRepository) ResolveApprovalLimitRole(ctx context.Context, userID uuid.UUID, role domain.StaffRole) (domain.StaffRole, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.approval_limit_role::text
		FROM user_groups g
		WHERE g.approval_limit_role IS NOT NULL
		  AND (g.code = 'ROLE_' || $2
		       OR EXISTS (
		           SELECT 1 FROM user_group_members m
		           WHERE m.user_id = $1 AND m.group_id = g.id))`, userID, string(role))
	if err != nil {
		return "", fmt.Errorf("membaca peran limit grup: %w", err)
	}
	defer func() { _ = rows.Close() }()

	best := domain.StaffRole("")
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return "", err
		}
		candidate := domain.StaffRole(raw)
		if candidate.PrivilegeRank() > best.PrivilegeRank() {
			best = candidate
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if best == "" {
		return role, nil
	}
	return best, nil
}

// UserExists melaporkan apakah pengguna staf ada (aktif maupun tidak). Keanggotaan
// grup tidak mengubah status pengguna, jadi pengguna nonaktif tetap boleh ditata.
func (r *PermissionRepository) UserExists(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM staff_users WHERE id = $1)`, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// UserInGroup melaporkan apakah pengguna sudah menjadi anggota grup groupCode.
func (r *PermissionRepository) UserInGroup(ctx context.Context, groupCode string, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_group_members m
			JOIN user_groups g ON g.id = m.group_id
			WHERE g.code = $1 AND m.user_id = $2)`, groupCode, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// AddMemberTx menambahkan pengguna ke grup. Grup tak dikenal menjadi 0 baris, yang
// oleh service diperlakukan sebagai error; idempoten lewat ON CONFLICT.
func (r *PermissionRepository) AddMemberTx(ctx context.Context, tx any, groupCode string, userID uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("permission: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO user_group_members (user_id, group_id)
		SELECT $2, id FROM user_groups WHERE code = $1
		ON CONFLICT (user_id, group_id) DO NOTHING`, groupCode, userID)
	return err
}

// RemoveMemberTx mengeluarkan pengguna dari grup. Grup tak dikenal cukup 0 baris.
func (r *PermissionRepository) RemoveMemberTx(ctx context.Context, tx any, groupCode string, userID uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("permission: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		DELETE FROM user_group_members m
		USING user_groups g
		WHERE m.group_id = g.id AND g.code = $1 AND m.user_id = $2`, groupCode, userID)
	return err
}

func (r *PermissionRepository) GroupExists(ctx context.Context, groupCode string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM user_groups WHERE code = $1)`, groupCode).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *PermissionRepository) GroupHasPermission(ctx context.Context, groupCode string, p domain.Permission) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM group_permissions gp
			JOIN user_groups g ON g.id = gp.group_id
			WHERE g.code = $1 AND gp.permission = $2)`, groupCode, p).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// GrantPermissionTx menambah izin ke grup. Grup tak dikenal menjadi 0 baris, yang
// pemanggil perlakukan sebagai error; idempoten lewat ON CONFLICT.
func (r *PermissionRepository) GrantPermissionTx(ctx context.Context, tx any, groupCode string, p domain.Permission) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("permission: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO group_permissions (group_id, permission, granted_at)
		SELECT id, $2, NOW() FROM user_groups WHERE code = $1
		ON CONFLICT (group_id, permission) DO NOTHING`, groupCode, p)
	return err
}

func (r *PermissionRepository) RevokePermissionTx(ctx context.Context, tx any, groupCode string, p domain.Permission) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("permission: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		DELETE FROM group_permissions gp
		USING user_groups g
		WHERE gp.group_id = g.id AND g.code = $1 AND gp.permission = $2`, groupCode, p)
	return err
}

// CountUsersLosingPermission menghitung pengguna aktif yang benar-benar kehilangan
// izin p bila p dicabut dari grup groupCode. Sumber izin efektif: grup ROLE_<peran>
// (selalu berlaku) digabung keanggotaan grup lain (aditif). Karena itu seorang
// pengguna hanya "kehilangan" bila tidak ada sumber lain yang masih memberikan p.
func (r *PermissionRepository) CountUsersLosingPermission(ctx context.Context, groupCode string, p domain.Permission) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM staff_users u
		WHERE u.is_active
		  -- sumber saat ini memang grup tujuan
		  AND EXISTS (
		      SELECT 1 FROM group_permissions gp
		      JOIN user_groups g ON g.id = gp.group_id
		      WHERE g.code = $1 AND gp.permission = $2
		        AND (g.code = 'ROLE_' || u.role::text OR EXISTS (
		              SELECT 1 FROM user_group_members m WHERE m.user_id = u.id AND m.group_id = g.id)))
		  -- dan group tujuan BUKAN satu-satunya sumber yang tersisa
		  AND NOT (
		      ($1 <> 'ROLE_' || u.role::text AND EXISTS (
		          SELECT 1 FROM group_permissions gp
		          JOIN user_groups rg ON rg.id = gp.group_id
		          WHERE rg.code = 'ROLE_' || u.role::text AND gp.permission = $2))
		      OR EXISTS (
		          SELECT 1 FROM user_group_members m
		          JOIN group_permissions gp ON gp.group_id = m.group_id
		          JOIN user_groups og ON og.id = m.group_id
		          WHERE m.user_id = u.id AND gp.permission = $2 AND og.code <> $1))`,
		groupCode, p).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountAccessLossOnMemberRemoval menghitung (0 atau 1) apakah mengeluarkan userID
// dari grup groupCode membuat pengguna itu kehilangan setidaknya satu izin efektif.
// Izin efektif = grup ROLE_<peran> (selalu berlaku) digabung seluruh keanggotaan.
// Penghapusan hanya berdampak bila ada izin grup tujuan yang tidak diberikan peran
// maupun keanggotaan lain; bila tidak ada, hasilnya 0 dan penghapusan tidak perlu
// konfirmasi. Semantik ini sejalan dengan CountUsersLosingPermission untuk pencabutan.
func (r *PermissionRepository) CountAccessLossOnMemberRemoval(ctx context.Context, groupCode string, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT CASE WHEN EXISTS (
		    SELECT 1
		    FROM staff_users u
		    JOIN user_group_members m ON m.user_id = u.id
		    JOIN user_groups g ON g.id = m.group_id AND g.code = $1
		    JOIN group_permissions gp ON gp.group_id = g.id
		    WHERE u.id = $2 AND u.is_active
		      AND NOT EXISTS (
		          SELECT 1 FROM group_permissions rp
		          JOIN user_groups rg ON rg.id = rp.group_id
		          WHERE rg.code = 'ROLE_' || u.role::text AND rp.permission = gp.permission)
		      AND NOT EXISTS (
		          SELECT 1 FROM user_group_members om
		          JOIN group_permissions op ON op.group_id = om.group_id
		          WHERE om.user_id = u.id AND op.permission = gp.permission
		            AND om.group_id <> g.id)
		) THEN 1 ELSE 0 END`, groupCode, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
