"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { CKPNActivation } from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ErrorState, LoadingState } from "@/components/ui/States";

/**
 * Kunci system_config yang dikelola pintu aktivasi CKPN. Sengaja didaftar di klien agar
 * formulir menampilkan kunci yang benar; nilai tetap dibaca dari server, tidak dikarang.
 */
const K = {
  pd1: "ckpn.pd_frac.gol_1",
  pd2: "ckpn.pd_frac.gol_2",
  pd3: "ckpn.pd_frac.gol_3",
  pd4: "ckpn.pd_frac.gol_4",
  pd5: "ckpn.pd_frac.gol_5",
  lgd: "ckpn.lgd_frac",
  coaExpenseSyariah: "ckpn.coa.expense.syariah",
  coaReserveSyariah: "ckpn.coa.reserve.syariah",
  status: "ckpn.parameters.status",
  since: "ckpn.parameters.temporary_since",
  baNumber: "ckpn.ratification.ba_number",
  baDate: "ckpn.ratification.ba_date",
  approvedBy: "ckpn.ratification.approved_by",
  basis: "ckpn.ratification.pd_lgd_basis",
  fromBank: "ckpn.ratification.pd_lgd_from_bank",
  shadow: "ckpn.shadow_mode.enabled",
  enabled: "ckpn.enabled",
} as const;

type FormKey =
  | "pd_frac_gol_1"
  | "pd_frac_gol_2"
  | "pd_frac_gol_3"
  | "pd_frac_gol_4"
  | "pd_frac_gol_5"
  | "lgd_frac"
  | "coa_expense_syariah"
  | "coa_reserve_syariah"
  | "parameters_status"
  | "parameters_temporary_since"
  | "ratification_ba_number"
  | "ratification_ba_date"
  | "ratification_approved_by"
  | "ratification_pd_lgd_basis"
  | "ratification_pd_lgd_from_bank"
  | "shadow_mode_enabled";

type FormState = Record<FormKey, string>;

const EMPTY_FORM: FormState = {
  pd_frac_gol_1: "",
  pd_frac_gol_2: "",
  pd_frac_gol_3: "",
  pd_frac_gol_4: "",
  pd_frac_gol_5: "",
  lgd_frac: "",
  coa_expense_syariah: "",
  coa_reserve_syariah: "",
  parameters_status: "SEMENTARA",
  parameters_temporary_since: "",
  ratification_ba_number: "",
  ratification_ba_date: "",
  ratification_approved_by: "",
  ratification_pd_lgd_basis: "",
  ratification_pd_lgd_from_bank: "false",
  shadow_mode_enabled: "false",
};

function toForm(activation: CKPNActivation | null): FormState {
  const v = (key: string) => activation?.values?.[key] ?? "";
  return {
    pd_frac_gol_1: v(K.pd1),
    pd_frac_gol_2: v(K.pd2),
    pd_frac_gol_3: v(K.pd3),
    pd_frac_gol_4: v(K.pd4),
    pd_frac_gol_5: v(K.pd5),
    lgd_frac: v(K.lgd),
    coa_expense_syariah: v(K.coaExpenseSyariah),
    coa_reserve_syariah: v(K.coaReserveSyariah),
    parameters_status: (v(K.status) || "SEMENTARA").toUpperCase(),
    parameters_temporary_since: v(K.since),
    ratification_ba_number: v(K.baNumber),
    ratification_ba_date: v(K.baDate),
    ratification_approved_by: v(K.approvedBy),
    ratification_pd_lgd_basis: v(K.basis),
    ratification_pd_lgd_from_bank: v(K.fromBank) === "true" ? "true" : "false",
    shadow_mode_enabled: v(K.shadow) === "true" ? "true" : "false",
  };
}

/** Satu bidang teks formulir; label dan petunjuk berasal dari kamus. */
interface TextFieldSpec {
  key: FormKey;
  label: string;
  hint?: string;
  isDate?: boolean;
  isMono?: boolean;
}

/**
 * Pengaturan aktivasi CKPN. Menampilkan status parameter dan sisa penahan penyalakan
 * lebih dahulu karena itulah keputusan yang sedang diambil; formulir parameter berada di
 * bawahnya. Menyalakan CKPN adalah aksi terpisah dengan konfirmasi bertulis karena ia
 * mengubah perlakuan akuntansi, bukan sekadar menyimpan setelan.
 */
export function CKPNActivationCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [activation, setActivation] = useState<CKPNActivation | null>(null);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [confirmEnable, setConfirmEnable] = useState(false);
  const [enabling, setEnabling] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setForbidden(false);
    try {
      const response = await request<CKPNActivation>("/system/ckpn-activation");
      const data = response.data ?? null;
      setActivation(data);
      setForm(toForm(data));
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setError(
          err instanceof ApiError ? err.message : t.ckpnActivation.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const update = (field: FormKey, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setFeedback(null);
    setSaveError(null);
    setFieldError(null);
    setSaving(true);
    try {
      // ckpn_enabled SENGAJA tidak dikirim di sini: penyalakan hanya lewat aksi khusus
      // berkonfirmasi, sehingga menyimpan parameter tidak pernah menyalakan CKPN diam-diam.
      const payload = {
        pd_frac_gol_1: form.pd_frac_gol_1.trim(),
        pd_frac_gol_2: form.pd_frac_gol_2.trim(),
        pd_frac_gol_3: form.pd_frac_gol_3.trim(),
        pd_frac_gol_4: form.pd_frac_gol_4.trim(),
        pd_frac_gol_5: form.pd_frac_gol_5.trim(),
        lgd_frac: form.lgd_frac.trim(),
        coa_expense_syariah: form.coa_expense_syariah.trim(),
        coa_reserve_syariah: form.coa_reserve_syariah.trim(),
        parameters_status: form.parameters_status,
        parameters_temporary_since: form.parameters_temporary_since.trim(),
        ratification_ba_number: form.ratification_ba_number.trim(),
        ratification_ba_date: form.ratification_ba_date.trim(),
        ratification_approved_by: form.ratification_approved_by.trim(),
        ratification_pd_lgd_basis: form.ratification_pd_lgd_basis.trim(),
        ratification_pd_lgd_from_bank:
          form.ratification_pd_lgd_from_bank === "true",
        shadow_mode_enabled: form.shadow_mode_enabled === "true",
      };
      const response = await request<CKPNActivation>(
        "/system/ckpn-activation",
        { method: "PUT", body: payload },
      );
      const data = response.data ?? null;
      setActivation(data);
      setForm(toForm(data));
      setFeedback(t.ckpnActivation.saved);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ckpnActivation.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        // Pesan validasi server menyebut kunci dan aturan yang dilanggar apa adanya.
        setFieldError(err.message);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ckpnActivation.saveError,
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const onEnable = async () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
    setEnabling(true);
    try {
      const response = await request<CKPNActivation>(
        "/system/ckpn-activation",
        {
          method: "PUT",
          body: { ckpn_enabled: true },
        },
      );
      const data = response.data ?? null;
      setActivation(data);
      setForm(toForm(data));
      setConfirmEnable(false);
      setFeedback(t.ckpnActivation.enabledNotice);
    } catch (err) {
      setConfirmEnable(false);
      if (err instanceof ApiError && err.status === 422) {
        setFieldError(err.message);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ckpnActivation.saveError,
        );
      }
      await load();
    } finally {
      setEnabling(false);
    }
  };

  const parameterFields: TextFieldSpec[] = [
    {
      key: "pd_frac_gol_1",
      label: t.ckpnActivation.pdGol1,
      hint: t.ckpnActivation.fractionHint,
      isMono: true,
    },
    {
      key: "pd_frac_gol_2",
      label: t.ckpnActivation.pdGol2,
      hint: t.ckpnActivation.fractionHint,
      isMono: true,
    },
    {
      key: "pd_frac_gol_3",
      label: t.ckpnActivation.pdGol3,
      hint: t.ckpnActivation.fractionHint,
      isMono: true,
    },
    {
      key: "pd_frac_gol_4",
      label: t.ckpnActivation.pdGol4,
      hint: t.ckpnActivation.fractionHint,
      isMono: true,
    },
    {
      key: "pd_frac_gol_5",
      label: t.ckpnActivation.pdGol5,
      hint: t.ckpnActivation.fractionHint,
      isMono: true,
    },
    {
      key: "lgd_frac",
      label: t.ckpnActivation.lgdFrac,
      hint: t.ckpnActivation.fractionHint,
      isMono: true,
    },
  ];

  const coaFields: TextFieldSpec[] = [
    {
      key: "coa_expense_syariah",
      label: t.ckpnActivation.coaExpenseSyariah,
      isMono: true,
    },
    {
      key: "coa_reserve_syariah",
      label: t.ckpnActivation.coaReserveSyariah,
      isMono: true,
    },
  ];

  const ratificationFields: TextFieldSpec[] = [
    { key: "ratification_ba_number", label: t.ckpnActivation.baNumber },
    {
      key: "ratification_ba_date",
      label: t.ckpnActivation.baDate,
      isDate: true,
    },
    { key: "ratification_approved_by", label: t.ckpnActivation.approvedBy },
    { key: "ratification_pd_lgd_basis", label: t.ckpnActivation.pdLgdBasis },
    {
      key: "parameters_temporary_since",
      label: t.ckpnActivation.temporarySince,
      isDate: true,
    },
  ];

  const status = activation?.status;
  const isEnabled = activation?.values?.[K.enabled] === "true";
  const gaps = activation?.enablement_gaps ?? [];
  const warnings = status?.warnings ?? [];

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ckpnActivation.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ckpnActivation.title}
            description={t.ckpnActivation.forbidden}
          />
        ) : error ? (
          <ErrorState
            title={t.ckpnActivation.title}
            description={error}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        ) : (
          <div className="space-y-4">
            <p className="text-body text-ink-600">
              {t.ckpnActivation.description}
            </p>

            {/* Status lebih dahulu: keputusan yang sedang diambil adalah "sudah boleh
                dinyalakan atau belum", bukan pengisian satu bidang. */}
            <section>
              <h4 className="text-title font-semibold text-ink-900">
                {t.ckpnActivation.statusTitle}
              </h4>
              <div className="mt-3">
                {status?.sementara ? (
                  <Alert
                    variant="warning"
                    title={t.ckpnActivation.statusSementara}
                  >
                    {t.ckpnActivation.statusSementaraBody}
                  </Alert>
                ) : (
                  <Alert variant="success" title={t.ckpnActivation.statusFinal}>
                    {t.ckpnActivation.statusFinalBody}
                  </Alert>
                )}
              </div>
              <div className="mt-3">
                <DefinitionList
                  columns={2}
                  items={[
                    {
                      label: t.ckpnActivation.fieldStatus,
                      value: status?.status ?? t.common.notAvailable,
                      isMono: true,
                    },
                    {
                      label: t.ckpnActivation.temporarySince,
                      value:
                        status?.temporary_since?.trim() ||
                        t.common.notAvailable,
                      isMono: true,
                    },
                    {
                      label: t.ckpnActivation.ratificationDeadline,
                      value:
                        status?.ratification_deadline?.trim() ||
                        t.common.notAvailable,
                      isMono: true,
                    },
                    {
                      label: t.ckpnActivation.floorPpka,
                      value: status?.floor_ppka_enforced
                        ? t.common.yes
                        : t.common.no,
                    },
                    {
                      label: t.ckpnActivation.ojkExportBlocked,
                      value: status?.ojk_export_blocked
                        ? t.common.yes
                        : t.common.no,
                    },
                    {
                      label: t.ckpnActivation.ckpnEnabledLabel,
                      value: isEnabled ? t.common.yes : t.common.no,
                    },
                  ]}
                />
              </div>
              {warnings.length > 0 && (
                <div className="mt-3">
                  <Alert
                    variant="warning"
                    title={t.ckpnActivation.warningsTitle}
                  >
                    <ul className="list-disc space-y-1 pl-5">
                      {warnings.map((warning) => (
                        <li key={warning}>{warning}</li>
                      ))}
                    </ul>
                  </Alert>
                </div>
              )}
            </section>

            {/* Penahan penyalakan: daftar yang harus habis sebelum tombol aktif. */}
            <section>
              <h4 className="text-title font-semibold text-ink-900">
                {t.ckpnActivation.gapsTitle}
              </h4>
              <div className="mt-3">
                {activation?.enablement_ready ? (
                  <Alert variant="success">{t.ckpnActivation.gapsReady}</Alert>
                ) : (
                  <Alert variant="warning">
                    <ul className="list-disc space-y-1 pl-5">
                      {gaps.map((gap) => (
                        <li key={gap}>{gap}</li>
                      ))}
                    </ul>
                  </Alert>
                )}
              </div>
            </section>

            {/* Penyalakan CKPN: aksi terpisah, berkonfirmasi bertulis, dan hanya aktif
                setelah daftar penahan kosong. Server tetap penjaga terakhir. */}
            <section className="rounded-md border border-border p-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ckpnActivation.enableTitle}
              </h4>
              <p className="mt-1 text-body text-ink-600">
                {t.ckpnActivation.enableDescription}
              </p>
              <div className="mt-3 flex items-center gap-3">
                {isEnabled ? (
                  <Alert variant="success">
                    {t.ckpnActivation.enabledNotice}
                  </Alert>
                ) : canEdit ? (
                  <Button
                    onClick={() => setConfirmEnable(true)}
                    disabled={!activation?.enablement_ready || enabling}
                  >
                    {t.ckpnActivation.enableAction}
                  </Button>
                ) : (
                  <Alert variant="info">{t.ckpnActivation.readOnly}</Alert>
                )}
              </div>
            </section>

            {canEdit ? (
              <form onSubmit={onSubmit} className="space-y-4">
                <fieldset className="rounded-md border border-border p-4">
                  <legend className="px-1 text-meta font-medium uppercase tracking-wide text-ink-600">
                    {t.ckpnActivation.sectionParameter}
                  </legend>
                  <div className="grid gap-4 md:grid-cols-2">
                    {parameterFields.map((field) => (
                      <Input
                        key={field.key}
                        label={field.label}
                        helperText={field.hint}
                        isMono={field.isMono}
                        placeholder={t.common.notAvailable}
                        value={form[field.key]}
                        onChange={(event) =>
                          update(field.key, event.target.value)
                        }
                      />
                    ))}
                  </div>
                </fieldset>

                <fieldset className="rounded-md border border-border p-4">
                  <legend className="px-1 text-meta font-medium uppercase tracking-wide text-ink-600">
                    {t.ckpnActivation.sectionCoaSyariah}
                  </legend>
                  <p className="mb-3 text-meta text-ink-600">
                    {t.ckpnActivation.coaSyariahHint}
                  </p>
                  <div className="grid gap-4 md:grid-cols-2">
                    {coaFields.map((field) => (
                      <Input
                        key={field.key}
                        label={field.label}
                        isMono={field.isMono}
                        placeholder={t.common.notAvailable}
                        value={form[field.key]}
                        onChange={(event) =>
                          update(field.key, event.target.value)
                        }
                      />
                    ))}
                  </div>
                </fieldset>

                <fieldset className="rounded-md border border-border p-4">
                  <legend className="px-1 text-meta font-medium uppercase tracking-wide text-ink-600">
                    {t.ckpnActivation.sectionRatification}
                  </legend>
                  <div className="grid gap-4 md:grid-cols-2">
                    {ratificationFields.map((field) => (
                      <Input
                        key={field.key}
                        type={field.isDate ? "date" : "text"}
                        label={field.label}
                        isMono={field.isDate ? true : field.isMono}
                        placeholder={t.common.notAvailable}
                        value={form[field.key]}
                        onChange={(event) =>
                          update(field.key, event.target.value)
                        }
                      />
                    ))}
                    <Select
                      label={t.ckpnActivation.pdLgdFromBank}
                      helperText={t.ckpnActivation.pdLgdFromBankHint}
                      options={[
                        { value: "true", label: t.common.yes },
                        { value: "false", label: t.common.no },
                      ]}
                      value={form.ratification_pd_lgd_from_bank}
                      onChange={(event) =>
                        update(
                          "ratification_pd_lgd_from_bank",
                          event.target.value,
                        )
                      }
                    />
                  </div>
                </fieldset>

                <fieldset className="rounded-md border border-border p-4">
                  <legend className="px-1 text-meta font-medium uppercase tracking-wide text-ink-600">
                    {t.ckpnActivation.sectionFlags}
                  </legend>
                  <div className="grid gap-4 md:grid-cols-2">
                    <Select
                      label={t.ckpnActivation.fieldStatus}
                      helperText={t.ckpnActivation.statusHint}
                      options={[
                        {
                          value: "SEMENTARA",
                          label: t.ckpnActivation.optSementara,
                        },
                        { value: "FINAL", label: t.ckpnActivation.optFinal },
                      ]}
                      value={form.parameters_status}
                      onChange={(event) =>
                        update("parameters_status", event.target.value)
                      }
                    />
                    <Select
                      label={t.ckpnActivation.shadowMode}
                      helperText={t.ckpnActivation.shadowModeHint}
                      options={[
                        { value: "true", label: t.common.yes },
                        { value: "false", label: t.common.no },
                      ]}
                      value={form.shadow_mode_enabled}
                      onChange={(event) =>
                        update("shadow_mode_enabled", event.target.value)
                      }
                    />
                  </div>
                </fieldset>

                {fieldError && <Alert variant="error">{fieldError}</Alert>}
                {feedback && <Alert variant="success">{feedback}</Alert>}
                {saveError && <Alert variant="error">{saveError}</Alert>}

                <div className="flex justify-end">
                  <Button type="submit" loading={saving}>
                    {saving ? t.ckpnActivation.saving : t.ckpnActivation.save}
                  </Button>
                </div>
              </form>
            ) : (
              <Alert variant="info">{t.ckpnActivation.readOnly}</Alert>
            )}
          </div>
        )}
      </CardContent>

      <ConfirmDialog
        open={confirmEnable}
        title={t.ckpnActivation.enableConfirmTitle}
        description={t.ckpnActivation.enableConfirmBody}
        confirmLabel={t.ckpnActivation.enableAction}
        destructive
        loading={enabling}
        requireKeyword={t.ckpnActivation.enableConfirmKeyword}
        onConfirm={onEnable}
        onCancel={() => setConfirmEnable(false)}
      />
    </Card>
  );
}
