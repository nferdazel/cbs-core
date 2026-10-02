"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  PihakLawanItem,
  PihakLawanItemsData,
} from "@/lib/operations-types";
import { formatDateISO } from "@/lib/format";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, Column } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { withCurrent, type Option } from "./register-options";

interface PihakLawanForm {
  id: string;
  pihak_lawan_id: string;
  jenis_identitas_code: string;
  jenis_kelamin_code: string;
  nama: string;
  kewarganegaraan_code: string;
  negara_code: string;
  jenis_usaha_code: string;
  hubungan_bank_code: string;
  golongan_code: string;
  lembaga_pemeringkat_code: string;
  peringkat_code: string;
  tanggal_pemeringkatan: string;
  tanggal_lahir: string;
  lokasi_code: string;
  grup_id: string;
  grup_nama: string;
  telepon: string;
  alamat: string;
  note: string;
}

/** Sandi baku Form 00.16 II/IV/IX/X (domain/form00_16_pihak_lawan.go). */
const IDENTITAS_CODES: string[] = ["1", "2", "3", "4"];
const KELAMIN_CODES: string[] = ["1", "2"];
const USAHA_CODES: string[] = ["1", "2"];
const HUBUNGAN_CODES: string[] = ["12", "20"];
const NAMA_MAX = 255;

function formFrom(row: PihakLawanItem): PihakLawanForm {
  return {
    id: row.id,
    pihak_lawan_id: row.pihak_lawan_id ?? "",
    jenis_identitas_code: row.jenis_identitas_code ?? "",
    jenis_kelamin_code: row.jenis_kelamin_code ?? "",
    nama: row.nama ?? "",
    kewarganegaraan_code: row.kewarganegaraan_code ?? "",
    negara_code: row.negara_code ?? "",
    jenis_usaha_code: row.jenis_usaha_code ?? "",
    hubungan_bank_code: row.hubungan_bank_code ?? "",
    golongan_code: row.golongan_code ?? "",
    lembaga_pemeringkat_code: row.lembaga_pemeringkat_code ?? "",
    peringkat_code: row.peringkat_code ?? "",
    tanggal_pemeringkatan: formatDateISO(row.tanggal_pemeringkatan),
    tanggal_lahir: formatDateISO(row.tanggal_lahir),
    lokasi_code: row.lokasi_code ?? "",
    grup_id: row.grup_id ?? "",
    grup_nama: row.grup_nama ?? "",
    telepon: row.telepon ?? "",
    alamat: row.alamat ?? "",
    note: row.note ?? "",
  };
}

function emptyForm(): PihakLawanForm {
  return {
    id: "",
    pihak_lawan_id: "",
    jenis_identitas_code: "",
    jenis_kelamin_code: "",
    nama: "",
    kewarganegaraan_code: "",
    negara_code: "",
    jenis_usaha_code: "",
    hubungan_bank_code: "",
    golongan_code: "",
    lembaga_pemeringkat_code: "",
    peringkat_code: "",
    tanggal_pemeringkatan: "",
    tanggal_lahir: "",
    lokasi_code: "",
    grup_id: "",
    grup_nama: "",
    telepon: "",
    alamat: "",
    note: "",
  };
}

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

/**
 * Kartu pengisian register Form 00.16 "Daftar Pihak Lawan". Nomor identitas (NIK/NPWP) dan
 * NPWP tidak diminta karena tidak disimpan. Penjagaan izin sebenarnya di API (system:config).
 */
export function OJKPihakLawanCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<PihakLawanItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<PihakLawanForm | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<PihakLawanItem | null>(null);
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<PihakLawanItemsData>(
        "/reports/ojk/pihak-lawan/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError
            ? err.message
            : t.ojkData.pihakLawan.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const identitasOptions: Option[] = [
    { value: "1", label: t.ojkData.pihakLawan.identitasKTP },
    { value: "2", label: t.ojkData.pihakLawan.identitasPaspor },
    { value: "3", label: t.ojkData.pihakLawan.identitasKITAS },
    { value: "4", label: t.ojkData.pihakLawan.identitasKK },
  ];
  const kelaminOptions: Option[] = [
    { value: "1", label: t.ojkData.pihakLawan.kelaminLakiLaki },
    { value: "2", label: t.ojkData.pihakLawan.kelaminPerempuan },
  ];
  const usahaOptions: Option[] = [
    { value: "1", label: t.ojkData.pihakLawan.usahaKonvensional },
    { value: "2", label: t.ojkData.pihakLawan.usahaSyariah },
  ];
  const hubunganOptions: Option[] = [
    { value: "12", label: t.ojkData.pihakLawan.hubunganTerkait },
    { value: "20", label: t.ojkData.pihakLawan.hubunganTidakTerkait },
  ];

  const updateForm = (patch: Partial<PihakLawanForm>) =>
    setForm((prev) => (prev ? { ...prev, ...patch } : prev));

  const resetMessages = () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const openCreate = () => {
    resetMessages();
    setForm(emptyForm());
  };

  const openEdit = (row: PihakLawanItem) => {
    resetMessages();
    setForm(formFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildPihakLawanItem, dicek sebelum menembak API. */
  const validate = (value: PihakLawanForm): string | null => {
    if (!value.pihak_lawan_id.trim()) {
      return t.ojkData.pihakLawan.validationPihakLawanId;
    }
    const nama = value.nama.trim();
    if (!nama) return t.ojkData.pihakLawan.validationNama;
    if (nama.length > NAMA_MAX)
      return t.ojkData.pihakLawan.validationNamaLength;
    if (
      value.jenis_identitas_code &&
      !IDENTITAS_CODES.includes(value.jenis_identitas_code)
    ) {
      return t.ojkData.pihakLawan.validationJenisIdentitas;
    }
    if (
      value.jenis_kelamin_code &&
      !KELAMIN_CODES.includes(value.jenis_kelamin_code)
    ) {
      return t.ojkData.pihakLawan.validationJenisKelamin;
    }
    if (
      value.jenis_usaha_code &&
      !USAHA_CODES.includes(value.jenis_usaha_code)
    ) {
      return t.ojkData.pihakLawan.validationJenisUsaha;
    }
    if (
      value.hubungan_bank_code &&
      !HUBUNGAN_CODES.includes(value.hubungan_bank_code)
    ) {
      return t.ojkData.pihakLawan.validationHubungan;
    }
    for (const raw of [value.tanggal_pemeringkatan, value.tanggal_lahir]) {
      if (raw && !ISO_DATE.test(raw)) {
        return t.ojkData.pihakLawan.validationTanggal;
      }
    }
    return null;
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!form) return;
    resetMessages();
    const invalid = validate(form);
    if (invalid) {
      setFieldError(invalid);
      return;
    }
    setSaving(true);
    try {
      await request("/reports/ojk/pihak-lawan/items", {
        method: "PUT",
        body: {
          id: form.id || undefined,
          pihak_lawan_id: form.pihak_lawan_id,
          jenis_identitas_code: form.jenis_identitas_code,
          jenis_kelamin_code: form.jenis_kelamin_code,
          nama: form.nama,
          kewarganegaraan_code: form.kewarganegaraan_code,
          negara_code: form.negara_code,
          jenis_usaha_code: form.jenis_usaha_code,
          hubungan_bank_code: form.hubungan_bank_code,
          golongan_code: form.golongan_code,
          lembaga_pemeringkat_code: form.lembaga_pemeringkat_code,
          peringkat_code: form.peringkat_code,
          tanggal_pemeringkatan: form.tanggal_pemeringkatan,
          tanggal_lahir: form.tanggal_lahir,
          lokasi_code: form.lokasi_code,
          grup_id: form.grup_id,
          grup_nama: form.grup_nama,
          telepon: form.telepon,
          alamat: form.alamat,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.pihakLawan.saved);
      setForm(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        setFieldError(err.message);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkData.saveError,
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    resetMessages();
    setDeleting(true);
    try {
      await request(`/reports/ojk/pihak-lawan/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.pihakLawan.deleted);
      setDeleteTarget(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.saveForbidden);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkData.saveError,
        );
      }
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  };

  const identitasSelectOptions = form
    ? withCurrent(identitasOptions, form.jenis_identitas_code)
    : identitasOptions;
  const kelaminSelectOptions = form
    ? withCurrent(kelaminOptions, form.jenis_kelamin_code)
    : kelaminOptions;
  const usahaSelectOptions = form
    ? withCurrent(usahaOptions, form.jenis_usaha_code)
    : usahaOptions;
  const hubunganSelectOptions = form
    ? withCurrent(hubunganOptions, form.hubungan_bank_code)
    : hubunganOptions;

  const columns: Column<PihakLawanItem>[] = [
    {
      header: t.ojkData.pihakLawan.colNama,
      cell: (row) => row.nama || "-",
    },
    {
      header: t.ojkData.pihakLawan.colGolongan,
      cell: (row) => row.golongan_code || "-",
      isMono: true,
    },
    {
      header: t.ojkData.pihakLawan.colKewarganegaraan,
      cell: (row) => row.kewarganegaraan_code || "-",
      isMono: true,
    },
    {
      header: t.ojkData.pihakLawan.colHubungan,
      cell: (row) => row.hubungan_bank_code || "-",
      isMono: true,
    },
  ];
  if (canEdit) {
    columns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <div className="flex justify-end gap-2">
          <Button size="sm" variant="secondary" onClick={() => openEdit(row)}>
            {t.ojkData.edit}
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => setDeleteTarget(row)}
          >
            {t.ojkData.delete}
          </Button>
        </div>
      ),
    });
  }

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkData.pihakLawan.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.pihakLawan.title}
            description={t.ojkData.pihakLawan.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.pihakLawan.title}
            description={loadError}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        ) : (
          <>
            <p className="text-body text-ink-600">
              {t.ojkData.pihakLawan.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.pihakLawan.itemsTitle}
              </h4>
              {canEdit && (
                <Button size="sm" onClick={openCreate}>
                  {t.ojkData.pihakLawan.addTitle}
                </Button>
              )}
            </div>

            {form && (
              <form
                onSubmit={onSubmit}
                className="space-y-4 rounded-md border border-border p-4"
              >
                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {form.id
                    ? t.ojkData.pihakLawan.editTitle
                    : t.ojkData.pihakLawan.addTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.pihakLawan.fieldPihakLawanId}
                    helperText={t.ojkData.pihakLawan.fieldPihakLawanIdHint}
                    value={form.pihak_lawan_id}
                    onChange={(event) =>
                      updateForm({ pihak_lawan_id: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakLawan.fieldNama}
                    helperText={t.ojkData.pihakLawan.fieldNamaHint}
                    value={form.nama}
                    onChange={(event) =>
                      updateForm({ nama: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.pihakLawan.fieldJenisIdentitas}
                    helperText={t.ojkData.pihakLawan.fieldJenisIdentitasHint}
                    placeholder={t.ojkData.pihakLawan.jenisIdentitasPlaceholder}
                    value={form.jenis_identitas_code}
                    options={identitasSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_identitas_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.pihakLawan.fieldJenisKelamin}
                    helperText={t.ojkData.pihakLawan.fieldJenisKelaminHint}
                    placeholder={t.ojkData.pihakLawan.jenisKelaminPlaceholder}
                    value={form.jenis_kelamin_code}
                    options={kelaminSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_kelamin_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakLawan.fieldKewarganegaraan}
                    helperText={t.ojkData.pihakLawan.fieldKewarganegaraanHint}
                    value={form.kewarganegaraan_code}
                    onChange={(event) =>
                      updateForm({ kewarganegaraan_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakLawan.fieldNegara}
                    helperText={t.ojkData.pihakLawan.fieldNegaraHint}
                    value={form.negara_code}
                    onChange={(event) =>
                      updateForm({ negara_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.pihakLawan.fieldJenisUsaha}
                    helperText={t.ojkData.pihakLawan.fieldJenisUsahaHint}
                    placeholder={t.ojkData.pihakLawan.jenisUsahaPlaceholder}
                    value={form.jenis_usaha_code}
                    options={usahaSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_usaha_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.pihakLawan.fieldHubungan}
                    helperText={t.ojkData.pihakLawan.fieldHubunganHint}
                    placeholder={t.ojkData.pihakLawan.hubunganPlaceholder}
                    value={form.hubungan_bank_code}
                    options={hubunganSelectOptions}
                    onChange={(event) =>
                      updateForm({ hubungan_bank_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakLawan.fieldGolongan}
                    helperText={t.ojkData.pihakLawan.fieldGolonganHint}
                    value={form.golongan_code}
                    onChange={(event) =>
                      updateForm({ golongan_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakLawan.fieldLokasi}
                    helperText={t.ojkData.pihakLawan.fieldLokasiHint}
                    value={form.lokasi_code}
                    onChange={(event) =>
                      updateForm({ lokasi_code: event.target.value })
                    }
                  />
                </div>

                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {t.ojkData.pihakLawan.sectionPemeringkat}
                </h5>
                <div className="grid gap-4 md:grid-cols-3">
                  <Input
                    label={t.ojkData.pihakLawan.fieldLembagaPemeringkat}
                    helperText={
                      t.ojkData.pihakLawan.fieldLembagaPemeringkatHint
                    }
                    value={form.lembaga_pemeringkat_code}
                    onChange={(event) =>
                      updateForm({
                        lembaga_pemeringkat_code: event.target.value,
                      })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakLawan.fieldPeringkat}
                    helperText={t.ojkData.pihakLawan.fieldPeringkatHint}
                    value={form.peringkat_code}
                    onChange={(event) =>
                      updateForm({ peringkat_code: event.target.value })
                    }
                  />
                  <Input
                    type="date"
                    label={t.ojkData.pihakLawan.fieldTanggalPemeringkatan}
                    helperText={
                      t.ojkData.pihakLawan.fieldTanggalPemeringkatanHint
                    }
                    value={form.tanggal_pemeringkatan}
                    onChange={(event) =>
                      updateForm({ tanggal_pemeringkatan: event.target.value })
                    }
                  />
                  <Input
                    type="date"
                    label={t.ojkData.pihakLawan.fieldTanggalLahir}
                    helperText={t.ojkData.pihakLawan.fieldTanggalLahirHint}
                    value={form.tanggal_lahir}
                    onChange={(event) =>
                      updateForm({ tanggal_lahir: event.target.value })
                    }
                  />
                </div>

                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {t.ojkData.pihakLawan.sectionKontak}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.pihakLawan.fieldGrupId}
                    helperText={t.ojkData.pihakLawan.fieldGrupIdHint}
                    value={form.grup_id}
                    onChange={(event) =>
                      updateForm({ grup_id: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakLawan.fieldGrupNama}
                    helperText={t.ojkData.pihakLawan.fieldGrupNamaHint}
                    value={form.grup_nama}
                    onChange={(event) =>
                      updateForm({ grup_nama: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakLawan.fieldTelepon}
                    helperText={t.ojkData.pihakLawan.fieldTeleponHint}
                    value={form.telepon}
                    onChange={(event) =>
                      updateForm({ telepon: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakLawan.fieldAlamat}
                    helperText={t.ojkData.pihakLawan.fieldAlamatHint}
                    value={form.alamat}
                    onChange={(event) =>
                      updateForm({ alamat: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.pihakLawan.fieldNote}
                  helperText={t.ojkData.pihakLawan.fieldNoteHint}
                  value={form.note}
                  onChange={(event) => updateForm({ note: event.target.value })}
                />

                <div className="flex justify-end gap-2">
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setForm(null)}
                  >
                    {t.common.cancel}
                  </Button>
                  <Button type="submit" loading={saving}>
                    {saving ? t.ojkData.saving : t.ojkData.save}
                  </Button>
                </div>
              </form>
            )}

            <DataTable
              columns={columns}
              data={items}
              keyExtractor={(row) => row.id}
              emptyMessage={t.ojkData.pihakLawan.empty}
              zebra
            />
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.pihakLawan.editTitle}
        description={t.ojkData.pihakLawan.deleteConfirm}
        confirmLabel={deleting ? t.ojkData.saving : t.ojkData.delete}
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
