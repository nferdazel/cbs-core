"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { PropertiItem, PropertiItemsData } from "@/lib/operations-types";
import { formatDateISO, formatMoney } from "@/lib/format";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { DataTable, Column } from "@/components/ui/DataTable";
import { DateInput } from "@/components/ui/DateInput";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { Textarea } from "@/components/ui/Textarea";

type Option = { value: string; label: string };

function withCurrent(options: Option[], current: string): Option[] {
  if (!current || options.some((option) => option.value === current)) {
    return options;
  }
  return [...options, { value: current, label: current }];
}

function labelOf(options: Option[], value: string): string {
  return options.find((option) => option.value === value)?.label ?? value;
}

interface PropertiForm {
  id: string;
  no_register: string;
  jenis_properti_code: string;
  alamat_properti: string;
  koordinat: string;
  tanggal_penetapan: string;
  biaya_perolehan_atau_nilai_wajar: number;
  akumulasi_penyusutan_atau_amortisasi: number;
  metode_pengukuran_code: string;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: PropertiForm = {
  id: "",
  no_register: "",
  jenis_properti_code: "",
  alamat_properti: "",
  koordinat: "",
  tanggal_penetapan: "",
  biaya_perolehan_atau_nilai_wajar: 0,
  akumulasi_penyusutan_atau_amortisasi: 0,
  metode_pengukuran_code: "",
  as_of: "",
  status: "AKTIF",
  note: "",
};

/** Sandi baku Form 17.00 (domain/properti.go). */
const JENIS_CODES: string[] = ["1", "2", "3"];
const METODE_CODES: string[] = ["1", "2"];
const NO_REGISTER_MAX = 64;
const ALAMAT_MAX = 255;
const KOORDINAT_MAX = 64;

function itemFormFrom(row: PropertiItem): PropertiForm {
  return {
    id: row.id,
    no_register: row.no_register ?? "",
    jenis_properti_code: row.jenis_properti_code ?? "",
    alamat_properti: row.alamat_properti ?? "",
    koordinat: row.koordinat ?? "",
    tanggal_penetapan: formatDateISO(row.tanggal_penetapan),
    biaya_perolehan_atau_nilai_wajar:
      Number(row.biaya_perolehan_atau_nilai_wajar) || 0,
    akumulasi_penyusutan_atau_amortisasi:
      Number(row.akumulasi_penyusutan_atau_amortisasi) || 0,
    metode_pengukuran_code: row.metode_pengukuran_code ?? "",
    as_of: formatDateISO(row.as_of),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

/**
 * Jumlah kolom IX (PDF #page 232): biaya perolehan atau nilai wajar (VII) dikurangi
 * akumulasi penyusutan atau amortisasi (VIII). null bila hasilnya negatif supaya
 * ditulis "-" dan tidak menyesatkan, sama seperti domain.PropertiItem.Jumlah.
 */
function jumlah(row: PropertiItem): number | null {
  const biaya = Number(row.biaya_perolehan_atau_nilai_wajar);
  const akumulasi = Number(row.akumulasi_penyusutan_atau_amortisasi);
  if (!Number.isFinite(biaya) || !Number.isFinite(akumulasi)) return null;
  const net = biaya - akumulasi;
  return Number.isFinite(net) && net >= 0 ? net : null;
}

/**
 * Kartu pengisian register properti terbengkalai Form 17.00: satu baris per
 * properti. Data sebelumnya hanya dapat diisi lewat API/SQL. Penjagaan izin
 * sebenarnya di API (system:config); kartu hanya menyembunyikan kontrol tulis saat
 * peran tidak memegang izin itu. Kolom IX Jumlah adalah turunan, jadi tidak
 * diminta diisi. Hapus adalah soft-delete: baris menjadi NONAKTIF dan nomor
 * register tetap terkunci, sesuai aturan no reuse/no recycle.
 */
export function OJKPropertiTerbengkalaiCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<PropertiItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PropertiItem | null>(null);
  const [form, setForm] = useState<PropertiForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<PropertiItemsData>(
        "/reports/ojk/properti/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkData.properti.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const jenisOptions: Option[] = [
    { value: "1", label: t.ojkData.properti.jenisTanah },
    { value: "2", label: t.ojkData.properti.jenisBangunan },
    { value: "3", label: t.ojkData.properti.jenisTanahBangunan },
  ];
  const metodeOptions: Option[] = [
    { value: "1", label: t.ojkData.properti.metodeBiaya },
    { value: "2", label: t.ojkData.properti.metodeRevaluasi },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.properti.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.properti.statusNonaktif },
  ];

  const updateForm = (patch: Partial<PropertiForm>) =>
    setForm((prev) => (prev ? { ...prev, ...patch } : prev));

  const resetMessages = () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const openNew = () => {
    resetMessages();
    setForm({ ...EMPTY_ITEM });
  };

  const openEdit = (row: PropertiItem) => {
    resetMessages();
    setForm(itemFormFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildPropertiItem, dicek sebelum menembak API. */
  const validate = (value: PropertiForm): string | null => {
    const noRegister = value.no_register.trim();
    if (!noRegister) return t.ojkData.properti.validationNoRegister;
    if (noRegister.length > NO_REGISTER_MAX) {
      return t.ojkData.properti.validationNoRegisterLength;
    }
    if (!JENIS_CODES.includes(value.jenis_properti_code)) {
      return t.ojkData.properti.validationJenis;
    }
    const alamat = value.alamat_properti.trim();
    if (!alamat) return t.ojkData.properti.validationAlamat;
    if (alamat.length > ALAMAT_MAX) {
      return t.ojkData.properti.validationAlamatLength;
    }
    if (value.koordinat.trim().length > KOORDINAT_MAX) {
      return t.ojkData.properti.validationKoordinat;
    }
    if (value.biaya_perolehan_atau_nilai_wajar < 0) {
      return t.ojkData.properti.validationBiaya;
    }
    if (value.akumulasi_penyusutan_atau_amortisasi < 0) {
      return t.ojkData.properti.validationAkumulasi;
    }
    if (!METODE_CODES.includes(value.metode_pengukuran_code)) {
      return t.ojkData.properti.validationMetode;
    }
    if (!value.tanggal_penetapan) {
      return t.ojkData.properti.validationTanggalPenetapan;
    }
    if (!value.as_of) return t.ojkData.properti.validationAsOf;
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
      await request<PropertiItem>("/reports/ojk/properti/items", {
        method: "PUT",
        body: {
          id: form.id,
          no_register: form.no_register,
          jenis_properti_code: form.jenis_properti_code,
          alamat_properti: form.alamat_properti,
          koordinat: form.koordinat,
          tanggal_penetapan: form.tanggal_penetapan,
          biaya_perolehan_atau_nilai_wajar:
            form.biaya_perolehan_atau_nilai_wajar,
          akumulasi_penyusutan_atau_amortisasi:
            form.akumulasi_penyusutan_atau_amortisasi,
          metode_pengukuran_code: form.metode_pengukuran_code,
          as_of: form.as_of,
          status: form.status,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.properti.saved);
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
      await request(`/reports/ojk/properti/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.properti.deleted);
      setDeleteTarget(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.deleteForbidden);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkData.deleteError,
        );
      }
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  };

  const jenisSelectOptions = form
    ? withCurrent(jenisOptions, form.jenis_properti_code)
    : jenisOptions;
  const metodeSelectOptions = form
    ? withCurrent(metodeOptions, form.metode_pengukuran_code)
    : metodeOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<PropertiItem>[] = [
    {
      header: t.ojkData.properti.colNoRegister,
      accessorKey: "no_register",
      isMono: true,
    },
    {
      header: t.ojkData.properti.colJenis,
      cell: (row) => labelOf(jenisOptions, row.jenis_properti_code),
    },
    {
      header: t.ojkData.properti.colAlamat,
      accessorKey: "alamat_properti",
    },
    {
      header: t.ojkData.properti.colKoordinat,
      cell: (row) => row.koordinat || "-",
      isMono: true,
    },
    {
      header: t.ojkData.properti.colTanggalPenetapan,
      cell: (row) => formatDateISO(row.tanggal_penetapan) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.properti.colBiayaPerolehan,
      type: "money",
      accessorKey: "biaya_perolehan_atau_nilai_wajar",
    },
    {
      header: t.ojkData.properti.colAkumulasi,
      type: "money",
      accessorKey: "akumulasi_penyusutan_atau_amortisasi",
    },
    {
      header: t.ojkData.properti.colJumlah,
      align: "right",
      cell: (row) => {
        const value = jumlah(row);
        return value === null ? "-" : formatMoney(value);
      },
    },
    {
      header: t.ojkData.properti.colMetode,
      cell: (row) => labelOf(metodeOptions, row.metode_pengukuran_code),
    },
    {
      header: t.ojkData.properti.colStatus,
      cell: (row) => (
        <Badge variant={row.status === "AKTIF" ? "credit" : "outline"}>
          {labelOf(statusOptions, row.status)}
        </Badge>
      ),
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
        <CardTitle>{t.ojkData.properti.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.properti.title}
            description={t.ojkData.properti.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.properti.title}
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
              {t.ojkData.properti.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.properti.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.properti.add}
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
                    ? t.ojkData.properti.editTitle
                    : t.ojkData.properti.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.properti.fieldNoRegister}
                    helperText={t.ojkData.properti.fieldNoRegisterHint}
                    value={form.no_register}
                    onChange={(event) =>
                      updateForm({ no_register: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.properti.fieldJenis}
                    helperText={t.ojkData.properti.fieldJenisHint}
                    placeholder={t.ojkData.properti.jenisPlaceholder}
                    value={form.jenis_properti_code}
                    options={jenisSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_properti_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.properti.fieldAlamat}
                    helperText={t.ojkData.properti.fieldAlamatHint}
                    value={form.alamat_properti}
                    onChange={(event) =>
                      updateForm({ alamat_properti: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.properti.fieldKoordinat}
                    helperText={t.ojkData.properti.fieldKoordinatHint}
                    value={form.koordinat}
                    onChange={(event) =>
                      updateForm({ koordinat: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.properti.fieldTanggalPenetapan}
                    helperText={t.ojkData.properti.fieldTanggalPenetapanHint}
                    value={form.tanggal_penetapan}
                    onChange={(event) =>
                      updateForm({ tanggal_penetapan: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.properti.fieldBiayaPerolehan}
                    helperText={t.ojkData.properti.fieldBiayaPerolehanHint}
                    value={form.biaya_perolehan_atau_nilai_wajar}
                    onChange={(value) =>
                      updateForm({
                        biaya_perolehan_atau_nilai_wajar: value,
                      })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.properti.fieldAkumulasi}
                    helperText={t.ojkData.properti.fieldAkumulasiHint}
                    value={form.akumulasi_penyusutan_atau_amortisasi}
                    onChange={(value) =>
                      updateForm({
                        akumulasi_penyusutan_atau_amortisasi: value,
                      })
                    }
                  />
                  <Select
                    label={t.ojkData.properti.fieldMetode}
                    helperText={t.ojkData.properti.fieldMetodeHint}
                    placeholder={t.ojkData.properti.metodePlaceholder}
                    value={form.metode_pengukuran_code}
                    options={metodeSelectOptions}
                    onChange={(event) =>
                      updateForm({ metode_pengukuran_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.properti.fieldAsOf}
                    helperText={t.ojkData.properti.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.properti.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.properti.fieldNote}
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
              emptyMessage={t.ojkData.properti.empty}
              zebra
            />
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.properti.deleteConfirmTitle}
        description={t.ojkData.properti.deleteConfirmBody}
        confirmLabel={
          deleting
            ? t.ojkData.properti.deactivating
            : t.ojkData.properti.deactivate
        }
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
