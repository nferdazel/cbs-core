-- 000095_ckpn_ratification_guard.up.sql
-- Run after: 000094_panel_tazir_collateral_ckpn_t0.up.sql
--
-- KEPUTUSAN PANEL RISIKO CKPN (butir 1) — PENGAMAN RATIFIKASI PARAMETER CKPN.
--
-- Panel TIDAK meratifikasi atas nama bank: ratifikasi adalah tindakan manusia yang
-- bertanda tangan (Direksi + akuntan; DPS untuk BPRS). Yang dapat dan HARUS dilakukan
-- mesin adalah MENOLAK perubahan status ke FINAL selama bukti ratifikasi belum lengkap,
-- dengan pesan yang menyebut apa yang kurang — bukan diam-diam mengizinkan.
--
-- Sebelumnya (000093) status hanya bertumpu pada niat operator: siapa pun dengan akses
-- SQL dapat menyetel FINAL dan angka PD/LGD sementara berubah status menjadi "final"
-- tanpa dasar. Migrasi ini menambahkan KUNCI BUKTI dan TRIGGER penegak. Jalur SQL
-- langsung pun ditolak; tidak hanya API.
--
-- Bukti (format paling sederhana yang tetap sah — kunci konfigurasi, tanpa tabel baru):
--   * ckpn.ratification.ba_number       nomor berita acara ratifikasi Direksi + akuntan
--   * ckpn.ratification.ba_date         tanggal berita acara (YYYY-MM-DD, tidak masa depan)
--   * ckpn.ratification.approved_by     nama dan jabatan pengesah
--   * ckpn.ratification.pd_lgd_basis    dasar perhitungan PD (PA BPR 12.6) dan LGD (12.7)
--   * ckpn.ratification.pd_lgd_from_bank = true  (PD/LGD dari data historis bank, bukan
--                                       angka turunan tarif PPKA/LGD 0,45)
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. Tidak ada angka ckpn.pd_frac.*/ckpn.lgd_frac yang
-- diubah dan ckpn.enabled TIDAK dinyalakan. Nilai bukti yang sudah diisi bank tidak
-- ditimpa (ON CONFLICT DO UPDATE hanya mengisi deskripsi bila nilai masih kosong).
--
-- Yang berwenang mengubah bukti/status: izin system:config (peran yang sama dengan
-- setelan instalasi lain), dan setiap perubahan tercatat di system_config.updated_by/
-- updated_at. Trigger menegakkan kelengkapan; otorisasi tetap di lapisan izin.

INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.ratification.ba_number', '',
     'Nomor berita acara ratifikasi parameter CKPN oleh Direksi + akuntan (dan DPS untuk BPRS). Bukti ratifikasi; wajib diisi sebelum ckpn.parameters.status dapat diubah ke FINAL (keputusan panel butir 1). Trigger 000095 menolak perubahan ke FINAL bila kunci ini kosong.'),
    ('ckpn.ratification.ba_date', '',
     'Tanggal berita acara ratifikasi parameter CKPN dalam format YYYY-MM-DD (tidak boleh di masa depan). Wajib sebelum status FINAL; diisi manusia, bukan sistem.'),
    ('ckpn.ratification.approved_by', '',
     'Nama dan jabatan pengesah ratifikasi parameter CKPN (Direksi + akuntan; DPS untuk BPRS). Data bank; sistem tidak boleh mengarang identitas. Wajib sebelum status FINAL.'),
    ('ckpn.ratification.pd_lgd_basis', '',
     'Dasar perhitungan PD (PA BPR butir 12.6) dan LGD (butir 12.7) dari data historis bank, mis. metode/periode observasi dan nomor lampiran berita acara. Wajib sebelum status FINAL; menutup pemakaian angka turunan tarif PPKA yang bukan metode SAK EP.'),
    ('ckpn.ratification.pd_lgd_from_bank', 'false',
     'Penanda bahwa PD/LGD dihitung dari data historis bank (PA BPR 12.6/12.7) atau excel parameter resmi OJK, BUKAN angka turunan tarif PPKA. Harus true sebelum ckpn.parameters.status boleh FINAL; selain itu perubahan ke FINAL ditolak trigger 000095.')
ON CONFLICT (key) DO UPDATE
    SET description = EXCLUDED.description
    WHERE system_config.value = '';

-- Trigger penegak: perubahan ckpn.parameters.status menuju FINAL WAJIB disertai bukti.
-- Ia berlaku untuk INSERT maupun UPDATE dan untuk pemilik tabel (bukan hanya lewat
-- REVOKE): akses SQL langsung pun tidak dapat melewatinya tanpa melengkapi bukti.
CREATE OR REPLACE FUNCTION ckpn_ratification_guard() RETURNS trigger AS $$
DECLARE
    v_ba_number   text;
    v_ba_date     text;
    v_approved_by text;
    v_basis       text;
    v_from_bank   text;
    missing       text[] := ARRAY[]::text[];
BEGIN
    -- Hanya transisi menuju FINAL pada kunci status yang dijaga. Menyetel SEMENTARA,
    -- mengubah kunci lain, dan FINAL->FINAL (idempotent) tidak diperiksa ulang.
    IF NEW.key <> 'ckpn.parameters.status'
       OR upper(btrim(coalesce(NEW.value, ''))) <> 'FINAL'
       OR (TG_OP = 'UPDATE' AND upper(btrim(coalesce(OLD.value, ''))) = 'FINAL') THEN
        RETURN NEW;
    END IF;

    SELECT btrim(value) INTO v_ba_number   FROM system_config WHERE key = 'ckpn.ratification.ba_number';
    SELECT btrim(value) INTO v_ba_date     FROM system_config WHERE key = 'ckpn.ratification.ba_date';
    SELECT btrim(value) INTO v_approved_by FROM system_config WHERE key = 'ckpn.ratification.approved_by';
    SELECT btrim(value) INTO v_basis       FROM system_config WHERE key = 'ckpn.ratification.pd_lgd_basis';
    SELECT btrim(value) INTO v_from_bank   FROM system_config WHERE key = 'ckpn.ratification.pd_lgd_from_bank';

    IF coalesce(v_ba_number, '') = '' THEN
        missing := array_append(missing, 'ckpn.ratification.ba_number (nomor berita acara)');
    END IF;
    IF coalesce(v_approved_by, '') = '' THEN
        missing := array_append(missing, 'ckpn.ratification.approved_by (nama/jabatan pengesah)');
    END IF;
    IF coalesce(v_basis, '') = '' THEN
        missing := array_append(missing, 'ckpn.ratification.pd_lgd_basis (dasar PD/LGD dari data bank)');
    END IF;
    IF lower(coalesce(v_from_bank, '')) NOT IN ('true', 't', '1', 'yes', 'on') THEN
        missing := array_append(missing, 'ckpn.ratification.pd_lgd_from_bank=true (PD/LGD dari data historis bank)');
    END IF;
    IF coalesce(v_ba_date, '') = '' THEN
        missing := array_append(missing, 'ckpn.ratification.ba_date (tanggal YYYY-MM-DD)');
    ELSIF v_ba_date !~ '^\d{4}-\d{2}-\d{2}$' THEN
        missing := array_append(missing, 'ckpn.ratification.ba_date bukan format YYYY-MM-DD: ' || v_ba_date);
    ELSE
        BEGIN
            IF v_ba_date::date > CURRENT_DATE THEN
                missing := array_append(missing, 'ckpn.ratification.ba_date tidak boleh di masa depan: ' || v_ba_date);
            END IF;
        EXCEPTION WHEN others THEN
            missing := array_append(missing, 'ckpn.ratification.ba_date bukan tanggal sah: ' || v_ba_date);
        END;
    END IF;

    IF array_length(missing, 1) > 0 THEN
        RAISE EXCEPTION 'ratifikasi parameter CKPN ditolak: bukti belum lengkap -> %',
            array_to_string(missing, '; ')
            USING ERRCODE = 'check_violation',
                  HINT = 'Lengkapi kunci bukti ratifikasi lebih dulu (Direksi + akuntan; DPS untuk BPRS), lalu setel ckpn.parameters.status=FINAL. Sistem tidak meratifikasi atas nama bank.';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ckpn_ratification_guard_row ON system_config;
CREATE TRIGGER ckpn_ratification_guard_row
    BEFORE INSERT OR UPDATE ON system_config
    FOR EACH ROW EXECUTE FUNCTION ckpn_ratification_guard();
