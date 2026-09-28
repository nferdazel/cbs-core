"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { CKPNPABL } from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { ErrorState, LoadingState } from "@/components/ui/States";

/**
 * Kunci system_config yang dikelola pintu CKPN penempatan pada bank lain (PABL).
 * Sengaja didaftar di klien agar formulir menampilkan kunci yang benar; nilai tetap
 * dibaca dari server, tidak dikarang.
 */
const K = {
  pd1: "ckpn.pabl.pd_frac.gol_1",
  pd3: "ckpn.pabl.pd_frac.gol_3",
  pd5: "ckpn.pabl.pd_frac.gol_5",
  lgd: "ckpn.pabl.lgd_frac",
  asetBaik: "ckpn.pabl.aset_baik_bentuk_ckpn",
  enabled: "ckpn.pabl.enabled",
} as const;

type FormKey =
  | "pd_frac_gol_1"
  | "pd_frac_gol_3"
  | "pd_frac_gol_5"
  | "lgd_frac"
  | "aset_baik_bentuk_ckpn"
  | "enabled";

type FormState = Record<FormKey, string>;

const FRACTION_KEYS = [
  "pd_frac_gol_1",
  "pd_frac_gol_3",
  "pd_frac_gol_5",
  "lgd_frac",
] as const;

const EMPTY_FORM: FormState = {
  pd_frac_gol_1: "",
  pd_frac_gol_3: "",
  pd_frac_gol_5: "",
  lgd_frac: "",
  aset_baik_bentuk_ckpn: "false",
  enabled: "false",
};

function toForm(pabl: CKPNPABL | null): FormState {
  const v = (key: string) => pabl?.values?.[key] ?? "";
  return {
    pd_frac_gol_1: v(K.pd1),
    pd_frac_gol_3: v(K.pd3),
    pd_frac_gol_5: v(K.pd5),
    lgd_frac: v(K.lgd),
    aset_baik_bentuk_ckpn: v(K.asetBaik) === "true" ? "true" : "false",
    enabled: v(K.enabled) === "true" ? "true" : "false",
  };
}

/**
 * Body PUT: hanya bidang yang benar-benar berubah terhadap nilai tersimpan. Bidang
 * teks yang tidak berubah tidak dikirim; mengosongkan nilai mengirim "" (bukan
 * menghapus bidang), sesuai kontrak endpoint.
 */
function changedFields(
  form: FormState,
  saved: FormState,
): Record<string, string | boolean> {
  const body: Record<string, string | boolean> = {};
  for (const key of FRACTION_KEYS) {
    const value = form[key].trim();
    if (value !== saved[key].trim()) body[key] = value;
  }
  if (form.aset_baik_bentuk_ckpn !== saved.aset_baik_bentuk_ckpn) {
    body.aset_baik_bentuk_ckpn = form.aset_baik_bentuk_ckpn === "true";
  }
  if (form.enabled !== saved.enabled) {
    body.enabled = form.enabled === "true";
  }
  return body;
}

/**
 * Pengaturan CKPN penempatan pada bank lain (PABL): fraksi PD/LGD mesin kolektif dan
 * dua saklar kebijakan. Hambatan penyalakan ditampilkan lebih dahulu karena itulah
 * keputusan yang sedang diambil; server tetap penjaga terakhir dan menolak penyalakan
 * prematur. Formulir hanya menyimpan bidang yang berubah.
 */
export function CKPNPABLCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [pabl, setPabl] = useState<CKPNPABL | null>(null);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [saved, setSaved] = useState<FormState>(EMPTY_FORM);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setForbidden(false);
    try {
      const response = await request<CKPNPABL>("/system/ckpn-pabl");
      const data = response.data ?? null;
      setPabl(data);
      const next = toForm(data);
      setForm(next);
      setSaved(next);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setError(err instanceof ApiError ? err.message : t.ckpnPabl.loadError);
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

  const body = useMemo(() => changedFields(form, saved), [form, saved]);
  const hasChanges = Object.keys(body).length > 0;

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setFeedback(null);
    setSaveError(null);
    setFieldError(null);
    if (!hasChanges) return;
    setSaving(true);
    try {
      const response = await request<CKPNPABL>("/system/ckpn-pabl", {
        method: "PUT",
        body,
      });
      const data = response.data ?? null;
      setPabl(data);
      const next = toForm(data);
      setForm(next);
      setSaved(next);
      setFeedback(t.ckpnPabl.saved);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ckpnPabl.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        // Pesan validasi server menyebut kunci dan aturan yang dilanggar apa adanya.
        // Muat ulang agar sisa hambatan penyalakan tampil dari server, bukan dikarang.
        setFieldError(err.message);
        await load();
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ckpnPabl.saveError,
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const gaps = pabl?.enablement_gaps ?? [];

  const fractionFields = [
    { key: "pd_frac_gol_1" as const, label: t.ckpnPabl.pdGol1 },
    { key: "pd_frac_gol_3" as const, label: t.ckpnPabl.pdGol3 },
    { key: "pd_frac_gol_5" as const, label: t.ckpnPabl.pdGol5 },
    { key: "lgd_frac" as const, label: t.ckpnPabl.lgdFrac },
  ];

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ckpnPabl.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ckpnPabl.title}
            description={t.ckpnPabl.forbidden}
          />
        ) : error ? (
          <ErrorState
            title={t.ckpnPabl.title}
            description={error}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        ) : (
          <div className="space-y-4">
            <p className="text-body text-ink-600">{t.ckpnPabl.description}</p>

            {/* Hambatan lebih dahulu: keputusan yang sedang diambil adalah "sudah
                boleh dinyalakan atau belum", bukan pengisian satu bidang. */}
            <section>
              <h4 className="text-title font-semibold text-ink-900">
                {t.ckpnPabl.gapsTitle}
              </h4>
              <div className="mt-3">
                {pabl?.enablement_ready ? (
                  <Alert variant="success">{t.ckpnPabl.gapsReady}</Alert>
                ) : (
                  <Alert variant="warning">
                    <ul className="list-disc space-y-1 pl-5">
                      {gaps.map((gap) => (
                        <li key={gap} className="font-mono">
                          {gap}
                        </li>
                      ))}
                    </ul>
                  </Alert>
                )}
              </div>
            </section>

            {canEdit ? (
              <form onSubmit={onSubmit} className="space-y-4">
                <fieldset className="rounded-md border border-border p-4">
                  <legend className="px-1 text-meta font-medium uppercase tracking-wide text-ink-600">
                    {t.ckpnPabl.sectionParameter}
                  </legend>
                  <div className="grid gap-4 md:grid-cols-2">
                    {fractionFields.map((field) => (
                      <Input
                        key={field.key}
                        label={field.label}
                        helperText={t.ckpnPabl.fractionHint}
                        isMono
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
                    {t.ckpnPabl.sectionFlags}
                  </legend>
                  <div className="grid gap-4 md:grid-cols-2">
                    <Select
                      label={t.ckpnPabl.asetBaik}
                      helperText={t.ckpnPabl.asetBaikHint}
                      options={[
                        { value: "true", label: t.common.yes },
                        { value: "false", label: t.common.no },
                      ]}
                      value={form.aset_baik_bentuk_ckpn}
                      onChange={(event) =>
                        update("aset_baik_bentuk_ckpn", event.target.value)
                      }
                    />
                    <Select
                      label={t.ckpnPabl.enabledLabel}
                      helperText={t.ckpnPabl.enabledHint}
                      options={[
                        { value: "true", label: t.common.yes },
                        { value: "false", label: t.common.no },
                      ]}
                      value={form.enabled}
                      onChange={(event) =>
                        update("enabled", event.target.value)
                      }
                    />
                  </div>
                </fieldset>

                {fieldError && <Alert variant="error">{fieldError}</Alert>}
                {feedback && <Alert variant="success">{feedback}</Alert>}
                {saveError && <Alert variant="error">{saveError}</Alert>}

                <div className="flex flex-wrap items-center justify-between gap-3">
                  <p className="text-meta text-ink-600">
                    {t.ckpnPabl.changeHint}
                  </p>
                  <Button type="submit" loading={saving} disabled={!hasChanges}>
                    {saving ? t.ckpnPabl.saving : t.ckpnPabl.save}
                  </Button>
                </div>
              </form>
            ) : (
              <Alert variant="info">{t.ckpnPabl.readOnly}</Alert>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
