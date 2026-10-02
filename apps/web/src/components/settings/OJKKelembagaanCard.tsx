"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  BankManagement,
  BankOffice,
  BankWorkUnit,
  KelembagaanData,
} from "@/lib/operations-types";
import { formatDateISO } from "@/lib/format";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, Column } from "@/components/ui/DataTable";
import { DateInput } from "@/components/ui/DateInput";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { Textarea } from "@/components/ui/Textarea";
import { withCurrent, labelOf, type Option } from "./register-options";

interface OfficeForm {
  id: string;
  office_type: string;
  code: string;
  name: string;
  address: string;
  city: string;
  ojk_kabupaten_code: string;
  opened_at: string;
  closed_at: string;
  status: string;
  note: string;
}

interface ManagementForm {
  id: string;
  category: string;
  name: string;
  position: string;
  ojk_position_code: string;
  license_number: string;
  license_date: string;
  started_at: string;
  ended_at: string;
  status: string;
  note: string;
}

const EMPTY_OFFICE: OfficeForm = {
  id: "",
  office_type: "",
  code: "",
  name: "",
  address: "",
  city: "",
  ojk_kabupaten_code: "",
  opened_at: "",
  closed_at: "",
  status: "AKTIF",
  note: "",
};

const EMPTY_MANAGEMENT: ManagementForm = {
  id: "",
  category: "",
  name: "",
  position: "",
  ojk_position_code: "",
  license_number: "",
  license_date: "",
  started_at: "",
  ended_at: "",
  status: "AKTIF",
  note: "",
};

function officeFormFrom(row: BankOffice): OfficeForm {
  return {
    id: row.id,
    office_type: row.office_type ?? "",
    code: row.code ?? "",
    name: row.name ?? "",
    address: row.address ?? "",
    city: row.city ?? "",
    ojk_kabupaten_code: row.ojk_kabupaten_code ?? "",
    opened_at: formatDateISO(row.opened_at),
    closed_at: formatDateISO(row.closed_at),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

function managementFormFrom(row: BankManagement): ManagementForm {
  return {
    id: row.id,
    category: row.category ?? "",
    name: row.name ?? "",
    position: row.position ?? "",
    ojk_position_code: row.ojk_position_code ?? "",
    license_number: row.license_number ?? "",
    license_date: formatDateISO(row.license_date),
    started_at: formatDateISO(row.started_at),
    ended_at: formatDateISO(row.ended_at),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

interface WorkUnitForm {
  id: string;
  code: string;
  nama: string;
  jenis: string;
  parent_code: string;
  kepala_unit: string;
  jumlah_pegawai: string;
  urutan: string;
  note: string;
}

const EMPTY_WORK_UNIT: WorkUnitForm = {
  id: "",
  code: "",
  nama: "",
  jenis: "",
  parent_code: "",
  kepala_unit: "",
  jumlah_pegawai: "",
  urutan: "",
  note: "",
};

function workUnitFormFrom(row: BankWorkUnit): WorkUnitForm {
  return {
    id: row.id,
    code: row.code ?? "",
    nama: row.nama ?? "",
    jenis: row.jenis ?? "",
    parent_code: row.parent_code ?? "",
    kepala_unit: row.kepala_unit ?? "",
    jumlah_pegawai:
      row.jumlah_pegawai != null ? String(row.jumlah_pegawai) : "",
    urutan: row.urutan ? String(row.urutan) : "",
    note: row.note ?? "",
  };
}

interface DeleteTarget {
  kind: "office" | "management" | "workunit";
  id: string;
  label: string;
}

/**
 * Kartu pengisian LAPORAN_KELEMBAGAAN: jaringan kantor (Form 00.04) dan pengurus
 * direksi/komisaris/pejabat eksekutif (Form 00.02/00.03). Data sebelumnya hanya
 * dapat diisi lewat API/SQL. Penjagaan izin sebenarnya di API; kartu hanya
 * menyembunyikan kontrol tulis saat peran tidak memegang system:config.
 */
export function OJKKelembagaanCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [offices, setOffices] = useState<BankOffice[]>([]);
  const [management, setManagement] = useState<BankManagement[]>([]);
  const [workUnits, setWorkUnits] = useState<BankWorkUnit[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [officeForm, setOfficeForm] = useState<OfficeForm | null>(null);
  const [managementForm, setManagementForm] = useState<ManagementForm | null>(
    null,
  );
  const [workUnitForm, setWorkUnitForm] = useState<WorkUnitForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<KelembagaanData>(
        "/reports/ojk/kelembagaan",
      );
      setOffices(response.data?.report?.offices ?? []);
      setManagement(response.data?.report?.management ?? []);
      setWorkUnits(response.data?.report?.work_units ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError
            ? err.message
            : t.ojkData.kelembagaan.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const officeStatusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.kelembagaan.statusAktif },
    { value: "TUTUP", label: t.ojkData.kelembagaan.statusTutup },
  ];
  const managementStatusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.kelembagaan.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.kelembagaan.statusNonaktif },
  ];
  const categoryOptions: Option[] = [
    { value: "DIREKSI", label: t.ojkData.kelembagaan.categoryDireksi },
    { value: "KOMISARIS", label: t.ojkData.kelembagaan.categoryKomisaris },
    {
      value: "PEJABAT_EKSEKUTIF",
      label: t.ojkData.kelembagaan.categoryPejabatEksekutif,
    },
  ];

  const statusTone = (value: string) =>
    value === "AKTIF" ? "credit" : "outline";

  const updateOffice = (patch: Partial<OfficeForm>) =>
    setOfficeForm((prev) => (prev ? { ...prev, ...patch } : prev));
  const updateManagement = (patch: Partial<ManagementForm>) =>
    setManagementForm((prev) => (prev ? { ...prev, ...patch } : prev));
  const updateWorkUnit = (patch: Partial<WorkUnitForm>) =>
    setWorkUnitForm((prev) => (prev ? { ...prev, ...patch } : prev));

  const resetMessages = () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const applySaveError = (err: unknown) => {
    if (err instanceof ApiError && err.status === 403) {
      setSaveError(t.ojkData.saveForbidden);
    } else if (err instanceof ApiError && err.status === 422) {
      setFieldError(err.message);
    } else {
      setSaveError(err instanceof ApiError ? err.message : t.ojkData.saveError);
    }
  };

  const saveOffice = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!officeForm) return;
    resetMessages();
    setSaving(true);
    try {
      await request("/reports/ojk/kelembagaan/offices", {
        method: "PUT",
        body: {
          id: officeForm.id,
          office_type: officeForm.office_type,
          code: officeForm.code,
          name: officeForm.name,
          address: officeForm.address,
          city: officeForm.city,
          ojk_kabupaten_code: officeForm.ojk_kabupaten_code,
          opened_at: officeForm.opened_at,
          closed_at: officeForm.closed_at,
          status: officeForm.status,
          note: officeForm.note,
        },
      });
      setFeedback(t.ojkData.kelembagaan.savedOffice);
      setOfficeForm(null);
      await load();
    } catch (err) {
      applySaveError(err);
    } finally {
      setSaving(false);
    }
  };

  const saveManagement = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!managementForm) return;
    resetMessages();
    setSaving(true);
    try {
      await request("/reports/ojk/kelembagaan/management", {
        method: "PUT",
        body: {
          id: managementForm.id,
          category: managementForm.category,
          name: managementForm.name,
          position: managementForm.position,
          ojk_position_code: managementForm.ojk_position_code,
          license_number: managementForm.license_number,
          license_date: managementForm.license_date,
          started_at: managementForm.started_at,
          ended_at: managementForm.ended_at,
          status: managementForm.status,
          note: managementForm.note,
        },
      });
      setFeedback(t.ojkData.kelembagaan.savedManagement);
      setManagementForm(null);
      await load();
    } catch (err) {
      applySaveError(err);
    } finally {
      setSaving(false);
    }
  };

  const saveWorkUnit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!workUnitForm) return;
    resetMessages();
    setSaving(true);
    try {
      await request("/reports/ojk/kelembagaan/work-units", {
        method: "PUT",
        body: {
          id: workUnitForm.id,
          code: workUnitForm.code,
          nama: workUnitForm.nama,
          jenis: workUnitForm.jenis,
          parent_code: workUnitForm.parent_code,
          kepala_unit: workUnitForm.kepala_unit,
          jumlah_pegawai: workUnitForm.jumlah_pegawai,
          urutan: workUnitForm.urutan,
          note: workUnitForm.note,
        },
      });
      setFeedback(t.ojkData.kelembagaan.savedWorkUnit);
      setWorkUnitForm(null);
      await load();
    } catch (err) {
      applySaveError(err);
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    resetMessages();
    setDeleting(true);
    const path =
      deleteTarget.kind === "office"
        ? `/reports/ojk/kelembagaan/offices/${deleteTarget.id}`
        : deleteTarget.kind === "management"
          ? `/reports/ojk/kelembagaan/management/${deleteTarget.id}`
          : `/reports/ojk/kelembagaan/work-units/${deleteTarget.id}`;
    try {
      await request(path, { method: "DELETE" });
      setFeedback(
        deleteTarget.kind === "office"
          ? t.ojkData.kelembagaan.deletedOffice
          : deleteTarget.kind === "management"
            ? t.ojkData.kelembagaan.deletedManagement
            : t.ojkData.kelembagaan.deletedWorkUnit,
      );
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

  const officeStatus = officeForm
    ? withCurrent(officeStatusOptions, officeForm.status)
    : officeStatusOptions;
  const managementStatus = managementForm
    ? withCurrent(managementStatusOptions, managementForm.status)
    : managementStatusOptions;
  const managementCategory = managementForm
    ? withCurrent(categoryOptions, managementForm.category)
    : categoryOptions;

  const officeColumns: Column<BankOffice>[] = [
    { header: t.ojkData.kelembagaan.colOfficeType, accessorKey: "office_type" },
    {
      header: t.ojkData.kelembagaan.colCode,
      cell: (row) => {
        if (!row.code) return "-";
        return <span className="font-mono">{row.code}</span>;
      },
    },
    { header: t.ojkData.kelembagaan.colName, accessorKey: "name" },
    { header: t.ojkData.kelembagaan.colCity, accessorKey: "city" },
    {
      header: t.ojkData.kelembagaan.colStatus,
      cell: (row) => (
        <Badge variant={statusTone(row.status)}>
          {labelOf(officeStatusOptions, row.status)}
        </Badge>
      ),
    },
    {
      header: t.ojkData.kelembagaan.colOpenedAt,
      cell: (row) => formatDateISO(row.opened_at) || "-",
    },
    {
      header: t.ojkData.kelembagaan.colClosedAt,
      cell: (row) => formatDateISO(row.closed_at) || "-",
    },
  ];
  if (canEdit) {
    officeColumns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <div className="flex justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              resetMessages();
              setOfficeForm(officeFormFrom(row));
            }}
          >
            {t.ojkData.edit}
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() =>
              setDeleteTarget({ kind: "office", id: row.id, label: row.name })
            }
          >
            {t.ojkData.delete}
          </Button>
        </div>
      ),
    });
  }

  const managementColumns: Column<BankManagement>[] = [
    {
      header: t.ojkData.kelembagaan.colCategory,
      cell: (row) => labelOf(categoryOptions, row.category),
    },
    { header: t.ojkData.kelembagaan.colName, accessorKey: "name" },
    { header: t.ojkData.kelembagaan.colPosition, accessorKey: "position" },
    {
      header: t.ojkData.kelembagaan.colOjkPositionCode,
      cell: (row) => {
        if (!row.ojk_position_code) return "-";
        return <span className="font-mono">{row.ojk_position_code}</span>;
      },
    },
    {
      header: t.ojkData.kelembagaan.colStatus,
      cell: (row) => (
        <Badge variant={statusTone(row.status)}>
          {labelOf(managementStatusOptions, row.status)}
        </Badge>
      ),
    },
    {
      header: t.ojkData.kelembagaan.colStartedAt,
      cell: (row) => formatDateISO(row.started_at) || "-",
    },
  ];
  if (canEdit) {
    managementColumns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <div className="flex justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              resetMessages();
              setManagementForm(managementFormFrom(row));
            }}
          >
            {t.ojkData.edit}
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() =>
              setDeleteTarget({
                kind: "management",
                id: row.id,
                label: row.name,
              })
            }
          >
            {t.ojkData.delete}
          </Button>
        </div>
      ),
    });
  }

  const workUnitColumns: Column<BankWorkUnit>[] = [
    {
      header: t.ojkData.kelembagaan.colUnitCode,
      cell: (row) => {
        if (!row.code) return "-";
        return <span className="font-mono">{row.code}</span>;
      },
    },
    { header: t.ojkData.kelembagaan.colUnitName, accessorKey: "nama" },
    { header: t.ojkData.kelembagaan.colUnitJenis, accessorKey: "jenis" },
    {
      header: t.ojkData.kelembagaan.colUnitParent,
      cell: (row) => row.parent_code || "-",
    },
    {
      header: t.ojkData.kelembagaan.colUnitKepala,
      cell: (row) => row.kepala_unit || "-",
    },
    {
      header: t.ojkData.kelembagaan.colUnitJumlah,
      cell: (row) => {
        if (row.jumlah_pegawai == null) return "-";
        return String(row.jumlah_pegawai);
      },
    },
  ];
  if (canEdit) {
    workUnitColumns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <div className="flex justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              resetMessages();
              setWorkUnitForm(workUnitFormFrom(row));
            }}
          >
            {t.ojkData.edit}
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() =>
              setDeleteTarget({ kind: "workunit", id: row.id, label: row.nama })
            }
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
        <CardTitle>{t.ojkData.kelembagaan.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.kelembagaan.title}
            description={t.ojkData.kelembagaan.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.kelembagaan.title}
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
              {t.ojkData.kelembagaan.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <section className="space-y-3">
              <div className="flex items-center justify-between gap-4">
                <h4 className="text-title font-semibold text-ink-900">
                  {t.ojkData.kelembagaan.officesTitle}
                </h4>
                {canEdit && !officeForm && (
                  <Button
                    size="sm"
                    onClick={() => {
                      resetMessages();
                      setOfficeForm({ ...EMPTY_OFFICE });
                    }}
                  >
                    {t.ojkData.kelembagaan.addOffice}
                  </Button>
                )}
              </div>

              {officeForm && (
                <form
                  onSubmit={saveOffice}
                  className="space-y-4 rounded-md border border-border p-4"
                >
                  <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                    {officeForm.id
                      ? t.ojkData.kelembagaan.editOfficeTitle
                      : t.ojkData.kelembagaan.newOfficeTitle}
                  </h5>
                  <div className="grid gap-4 md:grid-cols-2">
                    <Input
                      label={t.ojkData.kelembagaan.fieldOfficeType}
                      helperText={t.ojkData.kelembagaan.fieldOfficeTypeHint}
                      value={officeForm.office_type}
                      onChange={(event) =>
                        updateOffice({ office_type: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldCode}
                      helperText={t.ojkData.kelembagaan.fieldCodeHint}
                      isMono
                      value={officeForm.code}
                      onChange={(event) =>
                        updateOffice({ code: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldOfficeName}
                      value={officeForm.name}
                      onChange={(event) =>
                        updateOffice({ name: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldCity}
                      value={officeForm.city}
                      onChange={(event) =>
                        updateOffice({ city: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldAddress}
                      value={officeForm.address}
                      onChange={(event) =>
                        updateOffice({ address: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldKabupatenCode}
                      isMono
                      value={officeForm.ojk_kabupaten_code}
                      onChange={(event) =>
                        updateOffice({
                          ojk_kabupaten_code: event.target.value,
                        })
                      }
                    />
                    <DateInput
                      label={t.ojkData.kelembagaan.fieldOpenedAt}
                      value={officeForm.opened_at}
                      onChange={(event) =>
                        updateOffice({ opened_at: event.target.value })
                      }
                    />
                    <DateInput
                      label={t.ojkData.kelembagaan.fieldClosedAt}
                      value={officeForm.closed_at}
                      onChange={(event) =>
                        updateOffice({ closed_at: event.target.value })
                      }
                    />
                    <Select
                      label={t.ojkData.kelembagaan.fieldStatus}
                      value={officeForm.status}
                      options={officeStatus}
                      onChange={(event) =>
                        updateOffice({ status: event.target.value })
                      }
                    />
                  </div>
                  <Textarea
                    label={t.ojkData.kelembagaan.fieldNote}
                    value={officeForm.note}
                    onChange={(event) =>
                      updateOffice({ note: event.target.value })
                    }
                  />
                  <div className="flex justify-end gap-2">
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setOfficeForm(null)}
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
                columns={officeColumns}
                data={offices}
                keyExtractor={(row) => row.id}
                emptyMessage={t.ojkData.kelembagaan.officesEmpty}
                zebra
              />
            </section>

            <section className="space-y-3">
              <div className="flex items-center justify-between gap-4">
                <h4 className="text-title font-semibold text-ink-900">
                  {t.ojkData.kelembagaan.managementTitle}
                </h4>
                {canEdit && !managementForm && (
                  <Button
                    size="sm"
                    onClick={() => {
                      resetMessages();
                      setManagementForm({ ...EMPTY_MANAGEMENT });
                    }}
                  >
                    {t.ojkData.kelembagaan.addManagement}
                  </Button>
                )}
              </div>

              {managementForm && (
                <form
                  onSubmit={saveManagement}
                  className="space-y-4 rounded-md border border-border p-4"
                >
                  <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                    {managementForm.id
                      ? t.ojkData.kelembagaan.editManagementTitle
                      : t.ojkData.kelembagaan.newManagementTitle}
                  </h5>
                  <div className="grid gap-4 md:grid-cols-2">
                    <Select
                      label={t.ojkData.kelembagaan.fieldCategory}
                      placeholder={t.ojkData.kelembagaan.categoryPlaceholder}
                      value={managementForm.category}
                      options={managementCategory}
                      onChange={(event) =>
                        updateManagement({ category: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldPersonName}
                      value={managementForm.name}
                      onChange={(event) =>
                        updateManagement({ name: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldPosition}
                      value={managementForm.position}
                      onChange={(event) =>
                        updateManagement({ position: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldOjkPositionCode}
                      isMono
                      value={managementForm.ojk_position_code}
                      onChange={(event) =>
                        updateManagement({
                          ojk_position_code: event.target.value,
                        })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldLicenseNumber}
                      value={managementForm.license_number}
                      onChange={(event) =>
                        updateManagement({ license_number: event.target.value })
                      }
                    />
                    <DateInput
                      label={t.ojkData.kelembagaan.fieldLicenseDate}
                      value={managementForm.license_date}
                      onChange={(event) =>
                        updateManagement({ license_date: event.target.value })
                      }
                    />
                    <DateInput
                      label={t.ojkData.kelembagaan.fieldStartedAt}
                      value={managementForm.started_at}
                      onChange={(event) =>
                        updateManagement({ started_at: event.target.value })
                      }
                    />
                    <DateInput
                      label={t.ojkData.kelembagaan.fieldEndedAt}
                      value={managementForm.ended_at}
                      onChange={(event) =>
                        updateManagement({ ended_at: event.target.value })
                      }
                    />
                    <Select
                      label={t.ojkData.kelembagaan.fieldStatus}
                      value={managementForm.status}
                      options={managementStatus}
                      onChange={(event) =>
                        updateManagement({ status: event.target.value })
                      }
                    />
                  </div>
                  <Textarea
                    label={t.ojkData.kelembagaan.fieldNote}
                    value={managementForm.note}
                    onChange={(event) =>
                      updateManagement({ note: event.target.value })
                    }
                  />
                  <div className="flex justify-end gap-2">
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setManagementForm(null)}
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
                columns={managementColumns}
                data={management}
                keyExtractor={(row) => row.id}
                emptyMessage={t.ojkData.kelembagaan.managementEmpty}
                zebra
              />
            </section>

            <section className="space-y-3">
              <div className="flex items-center justify-between gap-4">
                <h4 className="text-title font-semibold text-ink-900">
                  {t.ojkData.kelembagaan.workUnitsTitle}
                </h4>
                {canEdit && !workUnitForm && (
                  <Button
                    size="sm"
                    onClick={() => {
                      resetMessages();
                      setWorkUnitForm({ ...EMPTY_WORK_UNIT });
                    }}
                  >
                    {t.ojkData.kelembagaan.addWorkUnit}
                  </Button>
                )}
              </div>

              <p className="text-body text-ink-600">
                {t.ojkData.kelembagaan.workUnitsHint}
              </p>

              {workUnitForm && (
                <form
                  onSubmit={saveWorkUnit}
                  className="space-y-4 rounded-md border border-border p-4"
                >
                  <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                    {workUnitForm.id
                      ? t.ojkData.kelembagaan.editWorkUnitTitle
                      : t.ojkData.kelembagaan.newWorkUnitTitle}
                  </h5>
                  <div className="grid gap-4 md:grid-cols-2">
                    <Input
                      label={t.ojkData.kelembagaan.fieldUnitCode}
                      helperText={t.ojkData.kelembagaan.fieldUnitCodeHint}
                      isMono
                      value={workUnitForm.code}
                      onChange={(event) =>
                        updateWorkUnit({ code: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldUnitName}
                      value={workUnitForm.nama}
                      onChange={(event) =>
                        updateWorkUnit({ nama: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldUnitJenis}
                      helperText={t.ojkData.kelembagaan.fieldUnitJenisHint}
                      value={workUnitForm.jenis}
                      onChange={(event) =>
                        updateWorkUnit({ jenis: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldUnitParent}
                      helperText={t.ojkData.kelembagaan.fieldUnitParentHint}
                      isMono
                      value={workUnitForm.parent_code}
                      onChange={(event) =>
                        updateWorkUnit({ parent_code: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldUnitKepala}
                      value={workUnitForm.kepala_unit}
                      onChange={(event) =>
                        updateWorkUnit({ kepala_unit: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldUnitJumlah}
                      helperText={t.ojkData.kelembagaan.fieldUnitJumlahHint}
                      inputMode="numeric"
                      value={workUnitForm.jumlah_pegawai}
                      onChange={(event) =>
                        updateWorkUnit({ jumlah_pegawai: event.target.value })
                      }
                    />
                    <Input
                      label={t.ojkData.kelembagaan.fieldUnitUrutan}
                      helperText={t.ojkData.kelembagaan.fieldUnitUrutanHint}
                      inputMode="numeric"
                      value={workUnitForm.urutan}
                      onChange={(event) =>
                        updateWorkUnit({ urutan: event.target.value })
                      }
                    />
                  </div>
                  <Textarea
                    label={t.ojkData.kelembagaan.fieldNote}
                    value={workUnitForm.note}
                    onChange={(event) =>
                      updateWorkUnit({ note: event.target.value })
                    }
                  />
                  <div className="flex justify-end gap-2">
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setWorkUnitForm(null)}
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
                columns={workUnitColumns}
                data={workUnits}
                keyExtractor={(row) => row.id}
                emptyMessage={t.ojkData.kelembagaan.workUnitsEmpty}
                zebra
              />
            </section>
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.confirmDeleteTitle}
        description={t.ojkData.confirmDeleteBody}
        confirmLabel={deleting ? t.ojkData.deleting : t.ojkData.delete}
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
