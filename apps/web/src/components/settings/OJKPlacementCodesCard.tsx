"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  OJKPlacementCodesData,
  OJKPlacementOption,
  OJKPlacementsData,
} from "@/lib/operations-types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";

// Sandi inline Hubungan dengan Bank Form 05.00 (12 terkait, 20 tidak terkait;
// Lampiran II). Nilainya sama dengan yang dipakai validator API.
const HUBUNGAN_BANK_CODES = ["12", "20"];

interface PlacementCodesForm {
  kabupaten: string;
  hubunganBank: string;
  alasanDiblokir: string;
  counterpartyCif: string;
  klasifikasiAset: string;
  blockedAmount: string;
  accruedInterestReceivable: string;
  accruedInterestPending: string;
}

const EMPTY_PLACEMENT_CODES: PlacementCodesForm = {
  kabupaten: "",
  hubunganBank: "",
  alasanDiblokir: "",
  counterpartyCif: "",
  klasifikasiAset: "",
  blockedAmount: "",
  accruedInterestReceivable: "",
  accruedInterestPending: "",
};

/** Nilai API bisa string desimal, angka, atau null; formulir selalu memakai string. */
function textValue(value: string | number | null | undefined): string {
  if (value === null || value === undefined) return "";
  return String(value);
}

function placementCodesFrom(
  data: OJKPlacementCodesData | null,
): PlacementCodesForm {
  if (!data) return { ...EMPTY_PLACEMENT_CODES };
  return {
    kabupaten: data.ojk_kabupaten_code ?? "",
    hubunganBank: data.ojk_hubungan_bank_code ?? "",
    alasanDiblokir: data.ojk_alasan_diblokir_code ?? "",
    counterpartyCif: data.counterparty_cif ?? "",
    klasifikasiAset: data.ojk_klasifikasi_aset_code ?? "",
    blockedAmount: textValue(data.blocked_amount),
    accruedInterestReceivable: textValue(data.accrued_interest_receivable),
    accruedInterestPending: textValue(data.accrued_interest_pending),
  };
}

/** Membandingkan dua nilai nominal sebagai angka; string kosong = tidak diisi. */
function sameNumber(a: string, b: string): boolean {
  if (a === "" || b === "") return a === b;
  return Number(a) === Number(b);
}

/**
 * Sandi OJK penempatan pada bank lain (Form 05.00) lewat endpoint khusus
 * PUT /ojk/placement-codes/{placementId}. Penempatan dipilih dari daftar Form 05.00,
 * lalu nilai tersimpan diambil lewat GET /ojk/placement-codes/{placementId} dan
 * ditampilkan. Hanya bidang yang berbeda dari nilai tersimpan yang dikirim; mengosongkan
 * bidang mengirim string kosong untuk menghapus nilai (bidang yang tidak disentuh absen).
 */
export function OJKPlacementCodesCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [placements, setPlacements] = useState<OJKPlacementOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState("");
  const [form, setForm] = useState<PlacementCodesForm>(EMPTY_PLACEMENT_CODES);
  const [saved, setSaved] = useState<PlacementCodesForm>(EMPTY_PLACEMENT_CODES);
  const [loadingCodes, setLoadingCodes] = useState(false);
  const [codesError, setCodesError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  // Penanda pemilihan terbaru agar balasan GET yang datang telat tidak menimpa
  // penempatan yang kini dipilih.
  const selectedIdRef = useRef("");

  const load = useCallback(async () => {
    setLoading(true);
    setForbidden(false);
    setLoadError(null);
    try {
      const response = await request<OJKPlacementsData>(
        "/reports/ojk/placements",
      );
      setPlacements(response.data?.placements ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkPlacement.selectError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const selectPlacement = async (id: string) => {
    setSelectedId(id);
    selectedIdRef.current = id;
    const empty = { ...EMPTY_PLACEMENT_CODES };
    setForm(empty);
    setSaved(empty);
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
    setCodesError(null);
    if (!id) return;

    setLoadingCodes(true);
    try {
      const response = await request<OJKPlacementCodesData>(
        `/ojk/placement-codes/${id}`,
      );
      if (selectedIdRef.current !== id) return;
      const stored = placementCodesFrom(response.data ?? null);
      setSaved(stored);
      setForm(stored);
    } catch (err) {
      if (selectedIdRef.current !== id) return;
      setCodesError(
        err instanceof ApiError ? err.message : t.ojkPlacement.loadError,
      );
    } finally {
      if (selectedIdRef.current === id) setLoadingCodes(false);
    }
  };

  const update = (field: keyof PlacementCodesForm, value: string) =>
    setForm((prev) => ({ ...prev, [field]: value }));

  // Simpan digit mentah, bukan angka terurai: "0" yang diketik harus tetap "0"
  // dan tidak dianggap kosong.
  const amountChange = (field: keyof PlacementCodesForm, raw: string) =>
    update(field, raw);

  const buildBody = (): Record<string, string> => {
    const body: Record<string, string> = {};
    const setText = (
      field: string,
      current: string,
      original: string,
    ): void => {
      if (current !== original) body[field] = current;
    };
    const setAmount = (
      field: string,
      current: string,
      original: string,
    ): void => {
      if (sameNumber(current, original)) return;
      body[field] = current === "" ? "" : String(Number(current));
    };

    setText("ojk_kabupaten_code", form.kabupaten, saved.kabupaten);
    setText("ojk_hubungan_bank_code", form.hubunganBank, saved.hubunganBank);
    setText(
      "ojk_alasan_diblokir_code",
      form.alasanDiblokir,
      saved.alasanDiblokir,
    );
    setText("counterparty_cif", form.counterpartyCif, saved.counterpartyCif);
    setText(
      "ojk_klasifikasi_aset_code",
      form.klasifikasiAset,
      saved.klasifikasiAset,
    );
    setAmount("blocked_amount", form.blockedAmount, saved.blockedAmount);
    setAmount(
      "accrued_interest_receivable",
      form.accruedInterestReceivable,
      saved.accruedInterestReceivable,
    );
    setAmount(
      "accrued_interest_pending",
      form.accruedInterestPending,
      saved.accruedInterestPending,
    );
    return body;
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedId) return;
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);

    const body = buildBody();
    if (Object.keys(body).length === 0) {
      setFieldError(t.ojkPlacement.noChanges);
      return;
    }

    setSaving(true);
    try {
      const response = await request<OJKPlacementCodesData>(
        `/ojk/placement-codes/${selectedId}`,
        { method: "PUT", body },
      );
      if (response.data) {
        const stored = placementCodesFrom(response.data);
        setSaved(stored);
        setForm(stored);
      }
      setFeedback(t.ojkPlacement.saved);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkPlacement.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        setFieldError(err.message);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkPlacement.saveError,
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const amountValue = (raw: string) => (raw === "" ? "" : Number(raw));

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkPlacement.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.ojkPlacement.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkPlacement.title}
            description={t.ojkPlacement.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkPlacement.title}
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
              {t.ojkPlacement.description}
            </p>

            {!canEdit ? (
              <Alert variant="info">{t.ojkPlacement.readOnly}</Alert>
            ) : placements.length === 0 ? (
              <Alert variant="info">{t.ojkPlacement.empty}</Alert>
            ) : (
              <form onSubmit={onSubmit} className="mt-3 space-y-4">
                <div className="w-80">
                  <Select
                    label={t.ojkPlacement.selectLabel}
                    placeholder={t.ojkPlacement.selectPlaceholder}
                    value={selectedId}
                    options={placements.map((placement) => ({
                      value: placement.id,
                      label: placement.label,
                    }))}
                    onChange={(event) => selectPlacement(event.target.value)}
                  />
                </div>

                {selectedId &&
                  (loadingCodes ? (
                    <LoadingState label={t.ojkPlacement.loadingCodes} />
                  ) : codesError ? (
                    <Alert variant="error">{codesError}</Alert>
                  ) : (
                    <>
                      <Alert variant="info">
                        {t.ojkPlacement.onlyChangedHint}
                      </Alert>

                      <div className="grid gap-4 md:grid-cols-2">
                        <Input
                          label={t.ojkPlacement.kabupatenLabel}
                          helperText={t.ojkPlacement.kabupatenHint}
                          isMono
                          value={form.kabupaten}
                          onChange={(event) =>
                            update("kabupaten", event.target.value)
                          }
                        />
                        <Select
                          label={t.ojkPlacement.hubunganBankLabel}
                          placeholder={t.ojkCodes.inlineEmptyOption}
                          value={form.hubunganBank}
                          options={HUBUNGAN_BANK_CODES.map((code) => ({
                            value: code,
                            label:
                              code === "12"
                                ? t.ojkPlacement.hubunganTerkait
                                : t.ojkPlacement.hubunganTidakTerkait,
                          }))}
                          onChange={(event) =>
                            update("hubunganBank", event.target.value)
                          }
                        />
                        <Input
                          label={t.ojkPlacement.alasanDiblokirLabel}
                          helperText={t.ojkPlacement.alasanDiblokirHint}
                          isMono
                          value={form.alasanDiblokir}
                          onChange={(event) =>
                            update("alasanDiblokir", event.target.value)
                          }
                        />
                        <Input
                          label={t.ojkPlacement.counterpartyCifLabel}
                          helperText={t.ojkPlacement.counterpartyCifHint}
                          isMono
                          value={form.counterpartyCif}
                          onChange={(event) =>
                            update("counterpartyCif", event.target.value)
                          }
                        />
                        <Input
                          label={t.ojkPlacement.klasifikasiAsetLabel}
                          helperText={t.ojkPlacement.klasifikasiAsetHint}
                          isMono
                          value={form.klasifikasiAset}
                          onChange={(event) =>
                            update("klasifikasiAset", event.target.value)
                          }
                        />
                      </div>

                      <div className="grid gap-4 md:grid-cols-2">
                        <CurrencyInput
                          label={t.ojkPlacement.blockedAmountLabel}
                          allowDecimals
                          helperText={t.ojkPlacement.blockedAmountHint}
                          value={amountValue(form.blockedAmount)}
                          onChange={(_value, raw) =>
                            amountChange("blockedAmount", raw)
                          }
                        />
                        <CurrencyInput
                          label={t.ojkPlacement.accruedInterestReceivableLabel}
                          allowDecimals
                          value={amountValue(form.accruedInterestReceivable)}
                          onChange={(_value, raw) =>
                            amountChange("accruedInterestReceivable", raw)
                          }
                        />
                        <CurrencyInput
                          label={t.ojkPlacement.accruedInterestPendingLabel}
                          allowDecimals
                          value={amountValue(form.accruedInterestPending)}
                          onChange={(_value, raw) =>
                            amountChange("accruedInterestPending", raw)
                          }
                        />
                      </div>

                      {fieldError && (
                        <Alert variant="error">{fieldError}</Alert>
                      )}
                      {feedback && <Alert variant="success">{feedback}</Alert>}
                      {saveError && <Alert variant="error">{saveError}</Alert>}

                      <div className="flex justify-end">
                        <Button type="submit" loading={saving}>
                          {saving ? t.ojkPlacement.saving : t.ojkPlacement.save}
                        </Button>
                      </div>
                    </>
                  ))}
              </form>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
