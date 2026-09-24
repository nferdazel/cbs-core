"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  CollateralWeightActivationPending,
  CollateralWeightAssessment,
  CollateralWeightCategory,
} from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { formatDate, formatRate } from "@/lib/format";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { MoneyText } from "@/components/ui/MoneyText";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { ErrorState, LoadingState } from "@/components/ui/States";

/**
 * Daftar kategori TIDAK disalin di web: sumbernya tabel
 * `collateral_lampiran_ii_weights` dan dibaca lewat `GET /collateral/weights`, supaya
 * kategori baru (kebijakan bank/regulasi) langsung tampil tanpa rilis ulang.
 */

/** Kode syarat C1-C9 plus gerbang tambahan pada `failures` server. */
const CONDITION_CODES = [
  "C1",
  "C2",
  "C3",
  "C4",
  "C5",
  "C6",
  "C7",
  "C8",
  "C9",
  "SHADOW",
  "COVERAGE",
] as const;

interface ConditionRow {
  code: string;
  label: string;
  passed: boolean;
  reasons: string[];
}

/** Pisahkan `failures` per kode syarat; sisa yang tak dikenal ditampilkan terpisah. */
function reasonsFor(code: string, failures: string[]): string[] {
  const prefix = `${code}: `;
  return failures
    .filter((failure) => failure.startsWith(prefix))
    .map((failure) => failure.slice(prefix.length).trim());
}

function isKnownCode(failure: string): boolean {
  return CONDITION_CODES.some((code) => failure.startsWith(`${code}: `));
}

/**
 * Gerbang aktivasi bobot risiko agunan (GET /collateral/weights/{kategori}). Menilai
 * C1-C9, cakupan, dan mode bayangan tanpa mengubah apa pun. Aktivasi HANYA diajukan
 * ke maker-checker (POST .../activate) dan baru berlaku setelah pemeriksa lain
 * menyetujui; UI tidak pernah menampilkan bobot sebagai aktif sebelum itu.
 */
export function CollateralWeightGate() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canActivate = hasPermission(user, "system:config");

  const [category, setCategory] = useState("");
  const [assessment, setAssessment] =
    useState<CollateralWeightAssessment | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);

  const [policyNumber, setPolicyNumber] = useState("");
  const [policyDate, setPolicyDate] = useState("");
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [pending, setPending] =
    useState<CollateralWeightActivationPending | null>(null);

  const [categories, setCategories] = useState<CollateralWeightCategory[]>([]);
  const [categoriesError, setCategoriesError] = useState<string | null>(null);
  const [categoriesLoading, setCategoriesLoading] = useState(true);

  // Daftar kategori diambil dari API (sumbernya tabel), bukan dari konstanta di web.
  const loadCategories = useCallback(async () => {
    setCategoriesLoading(true);
    setCategoriesError(null);
    try {
      const response = await request<CollateralWeightCategory[]>(
        "/collateral/weights",
      );
      setCategories(response.data ?? []);
    } catch (err) {
      setCategories([]);
      setCategoriesError(
        err instanceof ApiError ? err.message : t.collateralWeight.loadError,
      );
    } finally {
      setCategoriesLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadCategories();
  }, [loadCategories]);

  const load = useCallback(async () => {
    if (!category) return;
    setLoading(true);
    setError(null);
    setForbidden(false);
    try {
      const response = await request<CollateralWeightAssessment>(
        `/collateral/weights/${encodeURIComponent(category)}`,
      );
      setAssessment(response.data ?? null);
    } catch (err) {
      setAssessment(null);
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setError(
          err instanceof ApiError ? err.message : t.collateralWeight.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [category, t]);

  useEffect(() => {
    load();
  }, [load]);

  const onCategoryChange = (value: string) => {
    setCategory(value);
    setAssessment(null);
    setPending(null);
    setSubmitError(null);
  };

  const onActivate = async () => {
    setConfirmOpen(false);
    setSubmitError(null);
    if (!policyNumber.trim() || !policyDate) {
      setSubmitError(t.collateralWeight.policyRequired);
      return;
    }
    setSubmitting(true);
    try {
      const response = await request<CollateralWeightActivationPending>(
        `/collateral/weights/${encodeURIComponent(category)}/activate`,
        {
          method: "POST",
          body: {
            direksi_policy_number: policyNumber.trim(),
            direksi_policy_date: policyDate,
          },
        },
      );
      setPending(response.data ?? null);
    } catch (err) {
      setSubmitError(
        err instanceof ApiError ? err.message : t.collateralWeight.submitError,
      );
    } finally {
      setSubmitting(false);
    }
  };

  const categoryOptions = categories.map((item) => ({
    value: item.category_code,
    // Label dari basis data; jangan ditempel "belum aktif" di sini karena status
    // kategori sudah ditampilkan pada panel penilaian setelah dipilih.
    label: item.label,
  }));

  const conditionLabels: Record<string, string> = {
    C1: t.collateralWeight.conditions.C1,
    C2: t.collateralWeight.conditions.C2,
    C3: t.collateralWeight.conditions.C3,
    C4: t.collateralWeight.conditions.C4,
    C5: t.collateralWeight.conditions.C5,
    C6: t.collateralWeight.conditions.C6,
    C7: t.collateralWeight.conditions.C7,
    C8: t.collateralWeight.conditions.C8,
    C9: t.collateralWeight.conditions.C9,
    SHADOW: t.collateralWeight.conditions.SHADOW,
    COVERAGE: t.collateralWeight.conditions.COVERAGE,
  };

  const failures = assessment?.failures ?? [];
  const rows: ConditionRow[] = CONDITION_CODES.map((code) => {
    const reasons = reasonsFor(code, failures);
    return {
      code,
      label: conditionLabels[code],
      passed: reasons.length === 0,
      reasons,
    };
  });
  const otherFailures = failures.filter((failure) => !isKnownCode(failure));

  const columns: Column<ConditionRow>[] = [
    { header: t.collateralWeight.colCondition, cell: (row) => row.label },
    {
      header: t.collateralWeight.colResult,
      width: "110px",
      cell: (row) => (
        <StatusBadge
          status={
            row.passed ? t.collateralWeight.pass : t.collateralWeight.fail
          }
          tone={row.passed ? "credit" : "debit"}
        />
      ),
    },
    {
      header: t.collateralWeight.colReason,
      cell: (row) =>
        row.reasons.length > 0 ? (
          <span className="text-meta text-ink-600">
            {row.reasons.join("; ")}
          </span>
        ) : (
          t.collateralWeight.noReason
        ),
    },
  ];

  const hasPending = pending !== null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t.collateralWeight.title}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-body text-ink-600">
          {t.collateralWeight.description}
        </p>

        <div className="max-w-md">
          <Select
            label={t.collateralWeight.category}
            value={category}
            onChange={(event) => onCategoryChange(event.target.value)}
            placeholder={t.collateralWeight.categoryPlaceholder}
            options={categoryOptions}
            disabled={categoriesLoading || categories.length === 0}
          />
        </div>

        {categoriesLoading ? (
          <LoadingState label={t.states.loading} />
        ) : categoriesError ? (
          <ErrorState
            title={t.collateralWeight.title}
            description={categoriesError}
            action={
              <Button variant="secondary" onClick={loadCategories}>
                {t.common.retry}
              </Button>
            }
          />
        ) : categories.length === 0 ? (
          <Alert variant="warning" title={t.collateralWeight.noCategoriesTitle}>
            {t.collateralWeight.noCategoriesBody}
          </Alert>
        ) : !category ? (
          <p className="text-body text-ink-600">
            {t.collateralWeight.promptSelect}
          </p>
        ) : loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.collateralWeight.title}
            description={t.collateralWeight.forbidden}
          />
        ) : error ? (
          <ErrorState
            title={t.collateralWeight.title}
            description={error}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        ) : assessment ? (
          <>
            {assessment.enabled ? (
              <Alert variant="success" title={t.collateralWeight.statusActive}>
                {t.collateralWeight.activeBody}
              </Alert>
            ) : (
              <Alert
                variant="warning"
                title={t.collateralWeight.notActiveTitle}
              >
                {t.collateralWeight.notActiveBody}
              </Alert>
            )}

            <DataTable
              columns={columns}
              data={rows}
              keyExtractor={(row) => row.code}
            />

            {otherFailures.length > 0 && (
              <Alert variant="warning" title={t.collateralWeight.otherFailures}>
                <ul className="list-disc space-y-1 pl-5">
                  {otherFailures.map((failure) => (
                    <li key={failure}>{failure}</li>
                  ))}
                </ul>
              </Alert>
            )}

            <div className="grid gap-6 md:grid-cols-2">
              <section>
                <h4 className="mb-2 text-title font-semibold text-ink-900">
                  {t.collateralWeight.coverageTitle}
                </h4>
                <DefinitionList
                  items={[
                    {
                      label: t.collateralWeight.coverageFrac,
                      value: formatRate(Number(assessment.coverage_frac) * 100),
                      isMono: true,
                    },
                    {
                      label: t.collateralWeight.coverageMin,
                      value: formatRate(
                        Number(assessment.coverage_min_frac) * 100,
                      ),
                      isMono: true,
                    },
                    {
                      label: t.collateralWeight.coverageEligible,
                      value: <MoneyText value={assessment.eligible_value} />,
                    },
                    {
                      label: t.collateralWeight.coverageTotal,
                      value: <MoneyText value={assessment.total_value} />,
                    },
                  ]}
                />
              </section>
              <section>
                <h4 className="mb-2 text-title font-semibold text-ink-900">
                  {t.collateralWeight.shadowTitle}
                </h4>
                <DefinitionList
                  items={[
                    {
                      label: t.collateralWeight.shadowStarted,
                      value: assessment.shadow_started_at
                        ? formatDate(assessment.shadow_started_at)
                        : t.collateralWeight.shadowNotStarted,
                      isMono: true,
                    },
                    {
                      label: t.collateralWeight.shadowElapsed,
                      value:
                        assessment.shadow_months_elapsed < 0
                          ? t.collateralWeight.shadowNotStarted
                          : String(assessment.shadow_months_elapsed),
                      isMono: true,
                    },
                    {
                      label: t.collateralWeight.shadowRequired,
                      value: String(assessment.shadow_months_required),
                      isMono: true,
                    },
                  ]}
                />
              </section>
            </div>

            {hasPending ? (
              <Alert
                variant="success"
                title={t.collateralWeight.submittedTitle}
              >
                <p>{t.collateralWeight.submittedBody}</p>
                <p className="mt-1 text-meta">
                  {t.collateralWeight.requestId}:{" "}
                  <span className="font-mono">{pending.request_id}</span>{" "}
                  <StatusBadge status={pending.status} domain="approval" />
                </p>
              </Alert>
            ) : assessment.enabled ? null : canActivate ? (
              <section className="space-y-3">
                <h4 className="text-title font-semibold text-ink-900">
                  {t.collateralWeight.activationTitle}
                </h4>
                <p className="text-body text-ink-600">
                  {t.collateralWeight.activationDescription}
                </p>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.collateralWeight.policyNumber}
                    value={policyNumber}
                    onChange={(event) => setPolicyNumber(event.target.value)}
                    placeholder={t.collateralWeight.policyNumberPlaceholder}
                  />
                  <Input
                    label={t.collateralWeight.policyDate}
                    type="date"
                    value={policyDate}
                    onChange={(event) => setPolicyDate(event.target.value)}
                  />
                </div>
                {submitError && <Alert variant="error">{submitError}</Alert>}
                <div>
                  <Button onClick={() => setConfirmOpen(true)}>
                    {t.collateralWeight.activateButton}
                  </Button>
                </div>
              </section>
            ) : (
              <Alert variant="info">
                {t.collateralWeight.readOnlyActivate}
              </Alert>
            )}
          </>
        ) : null}

        <ConfirmDialog
          open={confirmOpen}
          title={t.collateralWeight.confirmTitle}
          description={t.collateralWeight.confirmDescription}
          confirmLabel={t.collateralWeight.confirmAction}
          loading={submitting}
          onConfirm={onActivate}
          onCancel={() => setConfirmOpen(false)}
        />
      </CardContent>
    </Card>
  );
}
