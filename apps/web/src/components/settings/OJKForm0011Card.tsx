"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  BankOffice,
  KelembagaanData,
  UpdateOfficeForm0011Payload,
} from "@/lib/operations-types";
import { formatDateISO } from "@/lib/format";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { DataTable, Column } from "@/components/ui/DataTable";
import { DateInput } from "@/components/ui/DateInput";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { withCurrent, labelOf, type Option } from "./register-options";

interface Form0011 {
  id: string;
  ojk_office_kind_code: string;
  parent_office_code: string;
  previous_office_code: string;
  coordinates: string;
  head_name: string;
  phone_number: string;
  ojk_change_code: string;
  implementation_date: string;
  control_office_code: string;
  ojk_approval_date: string;
}

/** Sandi baku Form 00.11 (domain/form00_11_kantor.go). */
const JENIS_CODES: string[] = ["02", "03", "04", "05", "06", "07", "08", "99"];
const KETERANGAN_CODES: string[] = ["1", "2", "3", "4", "5", "6", "7"];
const SANDI_MAX = 16;
const KOORDINAT_MAX = 64;
const PIMPINAN_MAX = 128;
const TELEPON_MAX = 32;
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

function formFrom(row: BankOffice): Form0011 {
  return {
    id: row.id,
    ojk_office_kind_code: row.ojk_office_kind_code ?? "",
    parent_office_code: row.parent_office_code ?? "",
    previous_office_code: row.previous_office_code ?? "",
    coordinates: row.coordinates ?? "",
    head_name: row.head_name ?? "",
    phone_number: row.phone_number ?? "",
    ojk_change_code: row.ojk_change_code ?? "",
    implementation_date: formatDateISO(row.implementation_date),
    control_office_code: row.control_office_code ?? "",
    ojk_approval_date: formatDateISO(row.ojk_approval_date),
  };
}

/**
 * Kartu pengisian kolom Form 00.11 pada kantor yang sudah tercatat. Ini BUKAN modul
 * kelembagaan: kartu TIDAK membuat/menghapus kantor, hanya melengkapi kolom Form
 * 00.11-nya. Kolom I Jenis wajib agar kantor masuk form; kantor pusat/cabang tidak diberi
 * sandi ini. Penjagaan izin sebenarnya di API (system:config). Kolom XII Kendali hanya
 * bermakna untuk Kantor Wilayah (07) dan SKK (08). Seluruh nilai isian bank.
 */
export function OJKForm0011Card() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<BankOffice[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<Form0011 | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<KelembagaanData>(
        "/reports/ojk/kelembagaan",
      );
      setItems(response.data?.report?.offices ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkData.form0011.loadError,
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
    { value: "02", label: t.ojkData.form0011.jenisKantorKas },
    { value: "03", label: t.ojkData.form0011.jenisKasKeliling },
    { value: "04", label: t.ojkData.form0011.jenisTitikPembayaran },
    { value: "05", label: t.ojkData.form0011.jenisATM },
    { value: "06", label: t.ojkData.form0011.jenisEDC },
    { value: "07", label: t.ojkData.form0011.jenisKantorWilayah },
    { value: "08", label: t.ojkData.form0011.jenisSentraKeuangan },
    { value: "99", label: t.ojkData.form0011.jenisLainnya },
  ];
  const keteranganOptions: Option[] = [
    { value: "1", label: t.ojkData.form0011.keteranganPembukaan },
    { value: "2", label: t.ojkData.form0011.keteranganPindahInduk },
    { value: "3", label: t.ojkData.form0011.keteranganPindah },
    { value: "4", label: t.ojkData.form0011.keteranganTidakBerubah },
    { value: "5", label: t.ojkData.form0011.keteranganKasDariSKK },
    { value: "6", label: t.ojkData.form0011.keteranganSKKDariKas },
    { value: "7", label: t.ojkData.form0011.keteranganSKKDariCabang },
  ];

  const updateForm = (patch: Partial<Form0011>) =>
    setForm((prev) => (prev ? { ...prev, ...patch } : prev));

  const resetMessages = () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const openEdit = (row: BankOffice) => {
    resetMessages();
    setForm(formFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildOfficeForm00_11, dicek sebelum menembak API. */
  const validate = (value: Form0011): string | null => {
    if (!JENIS_CODES.includes(value.ojk_office_kind_code)) {
      return t.ojkData.form0011.validationJenis;
    }
    if (
      value.ojk_change_code &&
      !KETERANGAN_CODES.includes(value.ojk_change_code)
    ) {
      return t.ojkData.form0011.validationKeterangan;
    }
    if (value.parent_office_code.trim().length > SANDI_MAX) {
      return t.ojkData.form0011.validationInduk;
    }
    if (value.previous_office_code.trim().length > SANDI_MAX) {
      return t.ojkData.form0011.validationSebelumnya;
    }
    if (value.control_office_code.trim().length > SANDI_MAX) {
      return t.ojkData.form0011.validationKendali;
    }
    if (value.coordinates.trim().length > KOORDINAT_MAX) {
      return t.ojkData.form0011.validationKoordinat;
    }
    if (value.head_name.trim().length > PIMPINAN_MAX) {
      return t.ojkData.form0011.validationPimpinan;
    }
    if (value.phone_number.trim().length > TELEPON_MAX) {
      return t.ojkData.form0011.validationTelepon;
    }
    if (value.implementation_date && !DATE_RE.test(value.implementation_date)) {
      return t.ojkData.form0011.validationPelaksanaan;
    }
    if (value.ojk_approval_date && !DATE_RE.test(value.ojk_approval_date)) {
      return t.ojkData.form0011.validationPersetujuan;
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
      const payload: UpdateOfficeForm0011Payload = {
        ojk_office_kind_code: form.ojk_office_kind_code,
        parent_office_code: form.parent_office_code,
        previous_office_code: form.previous_office_code,
        coordinates: form.coordinates,
        head_name: form.head_name,
        phone_number: form.phone_number,
        ojk_change_code: form.ojk_change_code,
        implementation_date: form.implementation_date,
        control_office_code: form.control_office_code,
        ojk_approval_date: form.ojk_approval_date,
      };
      await request(`/reports/ojk/kelembagaan/offices/${form.id}/form00-11`, {
        method: "PUT",
        body: payload,
      });
      setFeedback(t.ojkData.form0011.saved);
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

  const jenisSelectOptions = form
    ? withCurrent(jenisOptions, form.ojk_office_kind_code)
    : jenisOptions;
  const keteranganSelectOptions = form
    ? withCurrent(keteranganOptions, form.ojk_change_code)
    : keteranganOptions;

  const columns: Column<BankOffice>[] = [
    {
      header: t.ojkData.form0011.colJenis,
      cell: (row) => labelOf(jenisOptions, row.ojk_office_kind_code ?? ""),
    },
    {
      header: t.ojkData.form0011.colKode,
      cell: (row) => row.code || "-",
      isMono: true,
    },
    { header: t.ojkData.form0011.colNama, accessorKey: "name" },
    {
      header: t.ojkData.form0011.colInduk,
      cell: (row) => row.parent_office_code || "-",
      isMono: true,
    },
    {
      header: t.ojkData.form0011.colKeterangan,
      cell: (row) => labelOf(keteranganOptions, row.ojk_change_code ?? ""),
    },
    {
      header: t.ojkData.form0011.colPelaksanaan,
      cell: (row) => formatDateISO(row.implementation_date) || "-",
      isMono: true,
    },
  ];
  if (canEdit) {
    columns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <Button size="sm" variant="secondary" onClick={() => openEdit(row)}>
          {t.ojkData.edit}
        </Button>
      ),
    });
  }

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkData.form0011.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.form0011.title}
            description={t.ojkData.form0011.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.form0011.title}
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
              {t.ojkData.form0011.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <h4 className="text-title font-semibold text-ink-900">
              {t.ojkData.form0011.itemsTitle}
            </h4>

            {form && (
              <form
                onSubmit={onSubmit}
                className="space-y-4 rounded-md border border-border p-4"
              >
                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {t.ojkData.form0011.editTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Select
                    label={t.ojkData.form0011.fieldJenis}
                    helperText={t.ojkData.form0011.fieldJenisHint}
                    placeholder={t.ojkData.form0011.jenisPlaceholder}
                    value={form.ojk_office_kind_code}
                    options={jenisSelectOptions}
                    onChange={(event) =>
                      updateForm({ ojk_office_kind_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.form0011.fieldInduk}
                    helperText={t.ojkData.form0011.fieldIndukHint}
                    value={form.parent_office_code}
                    onChange={(event) =>
                      updateForm({ parent_office_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.form0011.fieldSebelumnya}
                    helperText={t.ojkData.form0011.fieldSebelumnyaHint}
                    value={form.previous_office_code}
                    onChange={(event) =>
                      updateForm({ previous_office_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.form0011.fieldKoordinat}
                    helperText={t.ojkData.form0011.fieldKoordinatHint}
                    value={form.coordinates}
                    onChange={(event) =>
                      updateForm({ coordinates: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.form0011.fieldPimpinan}
                    helperText={t.ojkData.form0011.fieldPimpinanHint}
                    value={form.head_name}
                    onChange={(event) =>
                      updateForm({ head_name: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.form0011.fieldTelepon}
                    helperText={t.ojkData.form0011.fieldTeleponHint}
                    value={form.phone_number}
                    onChange={(event) =>
                      updateForm({ phone_number: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.form0011.fieldKeterangan}
                    helperText={t.ojkData.form0011.fieldKeteranganHint}
                    placeholder={t.ojkData.form0011.keteranganPlaceholder}
                    value={form.ojk_change_code}
                    options={keteranganSelectOptions}
                    onChange={(event) =>
                      updateForm({ ojk_change_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.form0011.fieldPelaksanaan}
                    helperText={t.ojkData.form0011.fieldPelaksanaanHint}
                    value={form.implementation_date}
                    onChange={(event) =>
                      updateForm({ implementation_date: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.form0011.fieldKendali}
                    helperText={t.ojkData.form0011.fieldKendaliHint}
                    value={form.control_office_code}
                    onChange={(event) =>
                      updateForm({ control_office_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.form0011.fieldPersetujuan}
                    helperText={t.ojkData.form0011.fieldPersetujuanHint}
                    value={form.ojk_approval_date}
                    onChange={(event) =>
                      updateForm({ ojk_approval_date: event.target.value })
                    }
                  />
                </div>

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
              emptyMessage={t.ojkData.form0011.empty}
              zebra
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}
