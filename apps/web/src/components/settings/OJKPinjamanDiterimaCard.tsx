"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { PinjamanItem, PinjamanItemsData } from "@/lib/operations-types";
import { formatDateISO, formatMoney, formatRate } from "@/lib/format";
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

interface PinjamanForm {
  id: string;
  counterparty_id: string;
  creditor_group_code: string;
  bank_code: string;
  location_code: string;
  jenis_code: string;
  relationship_code: string;
  start_date: string;
  maturity_date: string;
  interest_rate: number;
  interest_calc_code: string;
  plafon: number;
  collateral_type_code: string;
  collateral_amount: number;
  baki_debet: number;
  unamortized_transaction_cost: number;
  unamortized_discount: number;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: PinjamanForm = {
  id: "",
  counterparty_id: "",
  creditor_group_code: "",
  bank_code: "",
  location_code: "",
  jenis_code: "",
  relationship_code: "",
  start_date: "",
  maturity_date: "",
  interest_rate: 0,
  interest_calc_code: "",
  plafon: 0,
  collateral_type_code: "",
  collateral_amount: 0,
  baki_debet: 0,
  unamortized_transaction_cost: 0,
  unamortized_discount: 0,
  as_of: "",
  status: "AKTIF",
  note: "",
};

/** Sandi baku Form 00.07 (domain/pinjaman.go). */
const JENIS_CODES: string[] = ["10", "20", "31", "32", "41", "42", "99"];
const RELATIONSHIP_CODES: string[] = ["12", "20"];
const INTEREST_CALC_CODES: string[] = ["11", "12", "21", "22"];
const TANPA_AGUNAN_CODE = "299";
const BANK_CODE_PATTERN = /^[0-9]{6}$/;

function itemFormFrom(row: PinjamanItem): PinjamanForm {
  return {
    id: row.id,
    counterparty_id: row.counterparty_id ?? "",
    creditor_group_code: row.creditor_group_code ?? "",
    bank_code: row.bank_code ?? "",
    location_code: row.location_code ?? "",
    jenis_code: row.jenis_code ?? "",
    relationship_code: row.relationship_code ?? "",
    start_date: formatDateISO(row.start_date),
    maturity_date: formatDateISO(row.maturity_date),
    interest_rate: Number(row.interest_rate) || 0,
    interest_calc_code: row.interest_calc_code ?? "",
    plafon: Number(row.plafon) || 0,
    collateral_type_code: row.collateral_type_code ?? "",
    collateral_amount: Number(row.collateral_amount) || 0,
    baki_debet: Number(row.baki_debet) || 0,
    unamortized_transaction_cost: Number(row.unamortized_transaction_cost) || 0,
    unamortized_discount: Number(row.unamortized_discount) || 0,
    as_of: formatDateISO(row.as_of),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

/**
 * Baki debet neto kolom XV (PDF #page 254): baki debet dikurangi biaya transaksi
 * dan diskonto belum diamortisasi. Nilai negatif ditulis "-" supaya tidak
 * menyesatkan, sama seperti domain.BakiDebetNeto.
 */
function bakiDebetNeto(row: PinjamanItem): number {
  const net =
    Number(row.baki_debet) -
    Number(row.unamortized_transaction_cost) -
    Number(row.unamortized_discount);
  return Number.isFinite(net) ? net : 0;
}

/**
 * Kartu pengisian register pinjaman yang diterima Form 00.07: satu baris per
 * pinjaman/kreditur. Data sebelumnya hanya dapat diisi lewat API/SQL. Penjagaan
 * izin sebenarnya di API (system:config); kartu hanya menyembunyikan kontrol
 * tulis saat peran tidak memegang izin itu. Golongan kreditur (Lampiran 02) dan
 * lokasi kreditur (Lampiran 03) belum punya daftar sandi dari API, jadi
 * keduanya diisi sebagai teks sandi apa adanya, bukan pilihan yang dikarang.
 */
export function OJKPinjamanDiterimaCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<PinjamanItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PinjamanItem | null>(null);
  const [form, setForm] = useState<PinjamanForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<PinjamanItemsData>(
        "/reports/ojk/pinjaman/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkData.pinjaman.loadError,
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
    { value: "10", label: t.ojkData.pinjaman.jenisBilateral },
    { value: "20", label: t.ojkData.pinjaman.jenisSindikasi },
    { value: "31", label: t.ojkData.pinjaman.jenisKhususLembagaPengayom },
    { value: "32", label: t.ojkData.pinjaman.jenisKhususLinkage },
    { value: "41", label: t.ojkData.pinjaman.jenisTertentuModalInti },
    { value: "42", label: t.ojkData.pinjaman.jenisTertentuModalPelengkap },
    { value: "99", label: t.ojkData.pinjaman.jenisLainnya },
  ];
  const relationshipOptions: Option[] = [
    { value: "12", label: t.ojkData.pinjaman.relationshipTerkait },
    { value: "20", label: t.ojkData.pinjaman.relationshipTidakTerkait },
  ];
  const interestCalcOptions: Option[] = [
    { value: "11", label: t.ojkData.pinjaman.calcFlatTetap },
    { value: "12", label: t.ojkData.pinjaman.calcFlatMengambang },
    { value: "21", label: t.ojkData.pinjaman.calcTidakFlatTetap },
    { value: "22", label: t.ojkData.pinjaman.calcTidakFlatMengambang },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.pinjaman.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.pinjaman.statusNonaktif },
  ];

  const updateForm = (patch: Partial<PinjamanForm>) =>
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

  const openEdit = (row: PinjamanItem) => {
    resetMessages();
    setForm(itemFormFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildPinjamanItem, dicek sebelum menembak API. */
  const validate = (value: PinjamanForm): string | null => {
    const counterparty = value.counterparty_id.trim();
    if (!counterparty) return t.ojkData.pinjaman.validationCounterpartyID;
    if (counterparty.length > 64) {
      return t.ojkData.pinjaman.validationCounterpartyIDLength;
    }
    if (!value.creditor_group_code.trim()) {
      return t.ojkData.pinjaman.validationCreditorGroup;
    }
    if (!BANK_CODE_PATTERN.test(value.bank_code.trim())) {
      return t.ojkData.pinjaman.validationBankCode;
    }
    if (!value.location_code.trim()) {
      return t.ojkData.pinjaman.validationLocation;
    }
    if (!JENIS_CODES.includes(value.jenis_code)) {
      return t.ojkData.pinjaman.validationJenis;
    }
    if (!RELATIONSHIP_CODES.includes(value.relationship_code)) {
      return t.ojkData.pinjaman.validationRelationship;
    }
    if (!value.start_date) return t.ojkData.pinjaman.validationStartDate;
    if (!value.maturity_date) return t.ojkData.pinjaman.validationMaturityDate;
    if (!INTEREST_CALC_CODES.includes(value.interest_calc_code)) {
      return t.ojkData.pinjaman.validationInterestCalc;
    }
    if (value.interest_rate < 0) {
      return t.ojkData.pinjaman.validationInterestRate;
    }
    if (value.plafon < 0) return t.ojkData.pinjaman.validationPlafon;
    const collateralType = value.collateral_type_code.trim();
    if (!collateralType) return t.ojkData.pinjaman.validationCollateralType;
    if (value.collateral_amount < 0) {
      return t.ojkData.pinjaman.validationCollateralAmount;
    }
    // Aturan tanpa agunan (PDF #page 253): sandi 299 hanya sah bila nominal 0,
    // dan nominal 0 hanya sah bila sandinya 299.
    const tanpaAgunan = collateralType === TANPA_AGUNAN_CODE;
    if (tanpaAgunan && value.collateral_amount !== 0) {
      return t.ojkData.pinjaman.validationCollateralConsistency;
    }
    if (!tanpaAgunan && value.collateral_amount === 0) {
      return t.ojkData.pinjaman.validationCollateralZero;
    }
    if (value.baki_debet < 0) return t.ojkData.pinjaman.validationBakiDebet;
    if (value.unamortized_transaction_cost < 0) {
      return t.ojkData.pinjaman.validationTransactionCost;
    }
    if (value.unamortized_discount < 0) {
      return t.ojkData.pinjaman.validationDiscount;
    }
    if (!value.as_of) return t.ojkData.pinjaman.validationAsOf;
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
      await request<PinjamanItem>("/reports/ojk/pinjaman/items", {
        method: "PUT",
        body: {
          id: form.id,
          counterparty_id: form.counterparty_id,
          creditor_group_code: form.creditor_group_code,
          bank_code: form.bank_code,
          location_code: form.location_code,
          jenis_code: form.jenis_code,
          relationship_code: form.relationship_code,
          start_date: form.start_date,
          maturity_date: form.maturity_date,
          interest_rate: form.interest_rate,
          interest_calc_code: form.interest_calc_code,
          plafon: form.plafon,
          collateral_type_code: form.collateral_type_code,
          collateral_amount: form.collateral_amount,
          baki_debet: form.baki_debet,
          unamortized_transaction_cost: form.unamortized_transaction_cost,
          unamortized_discount: form.unamortized_discount,
          as_of: form.as_of,
          status: form.status,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.pinjaman.saved);
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
      await request(`/reports/ojk/pinjaman/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.pinjaman.deleted);
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
    ? withCurrent(jenisOptions, form.jenis_code)
    : jenisOptions;
  const relationshipSelectOptions = form
    ? withCurrent(relationshipOptions, form.relationship_code)
    : relationshipOptions;
  const interestCalcSelectOptions = form
    ? withCurrent(interestCalcOptions, form.interest_calc_code)
    : interestCalcOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<PinjamanItem>[] = [
    {
      header: t.ojkData.pinjaman.colCounterpartyID,
      accessorKey: "counterparty_id",
      isMono: true,
    },
    {
      header: t.ojkData.pinjaman.colCreditorGroup,
      accessorKey: "creditor_group_code",
      isMono: true,
    },
    {
      header: t.ojkData.pinjaman.colBankCode,
      accessorKey: "bank_code",
      isMono: true,
    },
    {
      header: t.ojkData.pinjaman.colLocation,
      accessorKey: "location_code",
      isMono: true,
    },
    {
      header: t.ojkData.pinjaman.colJenis,
      cell: (row) => labelOf(jenisOptions, row.jenis_code),
    },
    {
      header: t.ojkData.pinjaman.colRelationship,
      cell: (row) => labelOf(relationshipOptions, row.relationship_code),
    },
    {
      header: t.ojkData.pinjaman.colTenor,
      cell: (row) => (
        <div className="space-y-0.5">
          <div className="font-mono text-meta">
            {formatDateISO(row.start_date) || "-"}
          </div>
          <div className="font-mono text-meta text-ink-600">
            {formatDateISO(row.maturity_date) || "-"}
          </div>
        </div>
      ),
    },
    {
      header: t.ojkData.pinjaman.colInterestRate,
      align: "right",
      cell: (row) => formatRate(row.interest_rate),
    },
    {
      header: t.ojkData.pinjaman.colPlafon,
      type: "money",
      accessorKey: "plafon",
    },
    {
      header: t.ojkData.pinjaman.colBakiDebet,
      type: "money",
      accessorKey: "baki_debet",
    },
    {
      header: t.ojkData.pinjaman.colBakiDebetNeto,
      align: "right",
      cell: (row) => {
        const neto = bakiDebetNeto(row);
        return neto < 0 ? "-" : formatMoney(neto);
      },
    },
    {
      header: t.ojkData.pinjaman.colStatus,
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
        <CardTitle>{t.ojkData.pinjaman.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.pinjaman.title}
            description={t.ojkData.pinjaman.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.pinjaman.title}
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
              {t.ojkData.pinjaman.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.pinjaman.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.pinjaman.add}
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
                    ? t.ojkData.pinjaman.editTitle
                    : t.ojkData.pinjaman.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.pinjaman.fieldCounterpartyID}
                    helperText={t.ojkData.pinjaman.fieldCounterpartyIDHint}
                    value={form.counterparty_id}
                    onChange={(event) =>
                      updateForm({ counterparty_id: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pinjaman.fieldCreditorGroup}
                    helperText={t.ojkData.pinjaman.fieldCreditorGroupHint}
                    value={form.creditor_group_code}
                    onChange={(event) =>
                      updateForm({ creditor_group_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pinjaman.fieldBankCode}
                    helperText={t.ojkData.pinjaman.fieldBankCodeHint}
                    inputMode="numeric"
                    value={form.bank_code}
                    onChange={(event) =>
                      updateForm({ bank_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pinjaman.fieldLocation}
                    helperText={t.ojkData.pinjaman.fieldLocationHint}
                    value={form.location_code}
                    onChange={(event) =>
                      updateForm({ location_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.pinjaman.fieldJenis}
                    helperText={t.ojkData.pinjaman.fieldJenisHint}
                    placeholder={t.ojkData.pinjaman.jenisPlaceholder}
                    value={form.jenis_code}
                    options={jenisSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.pinjaman.fieldRelationship}
                    helperText={t.ojkData.pinjaman.fieldRelationshipHint}
                    placeholder={t.ojkData.pinjaman.relationshipPlaceholder}
                    value={form.relationship_code}
                    options={relationshipSelectOptions}
                    onChange={(event) =>
                      updateForm({ relationship_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.pinjaman.fieldStartDate}
                    helperText={t.ojkData.pinjaman.fieldStartDateHint}
                    value={form.start_date}
                    onChange={(event) =>
                      updateForm({ start_date: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.pinjaman.fieldMaturityDate}
                    helperText={t.ojkData.pinjaman.fieldMaturityDateHint}
                    value={form.maturity_date}
                    onChange={(event) =>
                      updateForm({ maturity_date: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.pinjaman.fieldInterestRate}
                    helperText={t.ojkData.pinjaman.fieldInterestRateHint}
                    currencyPrefix="%"
                    allowDecimals
                    value={form.interest_rate}
                    onChange={(value) => updateForm({ interest_rate: value })}
                  />
                  <Select
                    label={t.ojkData.pinjaman.fieldInterestCalc}
                    helperText={t.ojkData.pinjaman.fieldInterestCalcHint}
                    placeholder={t.ojkData.pinjaman.interestCalcPlaceholder}
                    value={form.interest_calc_code}
                    options={interestCalcSelectOptions}
                    onChange={(event) =>
                      updateForm({ interest_calc_code: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.pinjaman.fieldPlafon}
                    helperText={t.ojkData.pinjaman.fieldPlafonHint}
                    value={form.plafon}
                    onChange={(value) => updateForm({ plafon: value })}
                  />
                  <Input
                    label={t.ojkData.pinjaman.fieldCollateralType}
                    helperText={t.ojkData.pinjaman.fieldCollateralTypeHint}
                    value={form.collateral_type_code}
                    onChange={(event) =>
                      updateForm({ collateral_type_code: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.pinjaman.fieldCollateralAmount}
                    helperText={t.ojkData.pinjaman.fieldCollateralAmountHint}
                    value={form.collateral_amount}
                    onChange={(value) =>
                      updateForm({ collateral_amount: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.pinjaman.fieldBakiDebet}
                    helperText={t.ojkData.pinjaman.fieldBakiDebetHint}
                    value={form.baki_debet}
                    onChange={(value) => updateForm({ baki_debet: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.pinjaman.fieldTransactionCost}
                    helperText={t.ojkData.pinjaman.fieldTransactionCostHint}
                    value={form.unamortized_transaction_cost}
                    onChange={(value) =>
                      updateForm({ unamortized_transaction_cost: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.pinjaman.fieldDiscount}
                    helperText={t.ojkData.pinjaman.fieldDiscountHint}
                    value={form.unamortized_discount}
                    onChange={(value) =>
                      updateForm({ unamortized_discount: value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.pinjaman.fieldAsOf}
                    helperText={t.ojkData.pinjaman.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.pinjaman.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.pinjaman.fieldNote}
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
              emptyMessage={t.ojkData.pinjaman.empty}
              zebra
            />
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
