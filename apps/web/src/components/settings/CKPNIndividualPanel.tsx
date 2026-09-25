"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  CKPNIndividualCandidate,
  CKPNIndividualEntryMarkInput,
  CKPNIndividualScanResult,
} from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { formatRate } from "@/lib/format";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { MoneyText } from "@/components/ui/MoneyText";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { CKPNIndividualEntryDialog } from "./CKPNIndividualEntryDialog";

/**
 * Pintu masuk CKPN individual (T3). Pemindaian baca-saja melaporkan kredit yang
 * wajib dinilai individual beserta pemicunya; penandaan menyimpan keputusan
 * pengelola tanpa menyentuh required_ckpn. Penjagaan sebenarnya ada di API
 * (loans:read untuk memindai, loans:approve untuk menandai).
 */
export function CKPNIndividualPanel() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canMark = hasPermission(user, "loans:approve");

  const [scan, setScan] = useState<CKPNIndividualScanResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);

  const [markTarget, setMarkTarget] = useState<CKPNIndividualCandidate | null>(
    null,
  );
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [lastMarked, setLastMarked] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setForbidden(false);
    try {
      const response = await request<CKPNIndividualScanResult>(
        "/ckpn/individual/scan",
      );
      setScan(response.data ?? null);
    } catch (err) {
      setScan(null);
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setError(
          err instanceof ApiError ? err.message : t.ckpnIndividual.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const methodLabel = (value: string): string => {
    switch (value) {
      case "DCF":
        return t.ckpnIndividual.methodDcf;
      case "COLLATERAL":
        return t.ckpnIndividual.methodCollateral;
      case "MAX":
        return t.ckpnIndividual.methodMax;
      default:
        return value;
    }
  };

  const onSubmitMark = async (input: CKPNIndividualEntryMarkInput) => {
    if (!markTarget) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      await request(
        `/ckpn/individual/${encodeURIComponent(markTarget.loan_number)}/entry`,
        { method: "POST", body: input },
      );
      setLastMarked(markTarget.loan_number);
      setMarkTarget(null);
      await load();
    } catch (err) {
      setSubmitError(
        err instanceof ApiError ? err.message : t.ckpnIndividual.submitError,
      );
    } finally {
      setSubmitting(false);
    }
  };

  const columns: Column<CKPNIndividualCandidate>[] = [
    {
      header: t.ckpnIndividual.colLoan,
      cell: (row) => (
        <div>
          <span className="font-mono text-ink-900">{row.loan_number}</span>
          {row.branch_code && (
            <span className="mt-0.5 block text-meta text-ink-600">
              {row.branch_code}
            </span>
          )}
        </div>
      ),
    },
    {
      header: t.ckpnIndividual.colRank,
      width: "90px",
      align: "right",
      isMono: true,
      cell: (row) => String(row.rank),
    },
    {
      header: t.ckpnIndividual.colOutstanding,
      type: "money",
      accessorKey: "outstanding",
    },
    {
      header: t.ckpnIndividual.colTriggers,
      cell: (row) => (
        <div className="space-y-1">
          <div className="flex flex-wrap gap-1">
            {row.entry.triggers.map((trigger) => (
              <Badge
                key={trigger.code}
                variant={trigger.mandatory ? "accent" : "outline"}
              >
                {`${
                  trigger.mandatory
                    ? t.ckpnIndividual.triggerMandatory
                    : t.ckpnIndividual.triggerOptional
                }: ${trigger.code}`}
              </Badge>
            ))}
          </div>
          <ul className="space-y-0.5">
            {row.entry.triggers.map((trigger) => (
              <li
                key={`${trigger.code}-reason`}
                className="text-meta text-ink-600"
              >
                {trigger.reason}
              </li>
            ))}
          </ul>
        </div>
      ),
    },
    {
      header: t.ckpnIndividual.colMethod,
      width: "180px",
      cell: (row) => methodLabel(row.entry.suggested_method),
    },
  ];

  if (canMark) {
    columns.push({
      header: t.ckpnIndividual.colAction,
      width: "100px",
      align: "right",
      cell: (row) => (
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            setSubmitError(null);
            setMarkTarget(row);
          }}
        >
          {t.ckpnIndividual.markButton}
        </Button>
      ),
    });
  }

  const policy = scan?.policy;
  const mandatoryCodes = policy
    ? [
        policy.mandatory_on_macet ? "MACET" : null,
        policy.mandatory_on_restructured ? "RESTRUCTURED" : null,
        policy.mandatory_dpd_days > 0
          ? `DPD > ${policy.mandatory_dpd_days}`
          : null,
        policy.mandatory_on_collateral_drop ? "COLLATERAL_DROP" : null,
        policy.mandatory_on_objective_evidence ? "OBJECTIVE_EVIDENCE" : null,
      ].filter((code): code is string => code !== null)
    : [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t.ckpnIndividual.title}</CardTitle>
        <Button variant="secondary" size="sm" onClick={load} disabled={loading}>
          {t.ckpnIndividual.refresh}
        </Button>
      </CardHeader>
      <CardContent>
        <p className="text-body text-ink-600">{t.ckpnIndividual.scanNote}</p>

        {lastMarked && (
          <Alert variant="success" title={t.ckpnIndividual.markedTitle}>
            <p>{t.ckpnIndividual.markedBody}</p>
            <p className="mt-1 text-meta">
              <span className="font-mono">{lastMarked}</span>
            </p>
          </Alert>
        )}

        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ckpnIndividual.title}
            description={t.ckpnIndividual.forbidden}
          />
        ) : error ? (
          <ErrorState
            title={t.ckpnIndividual.title}
            description={error}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        ) : scan ? (
          <>
            <section>
              <div className="mb-2 flex items-center justify-between gap-4">
                <h4 className="text-title font-semibold text-ink-900">
                  {t.ckpnIndividual.candidatesTitle}
                </h4>
                <span className="text-meta text-ink-600">
                  {t.ckpnIndividual.scanned}: {scan.scanned}
                </span>
              </div>
              <p className="mb-2 text-meta text-ink-600">
                {t.ckpnIndividual.triggerLegend}
              </p>
              <DataTable
                columns={columns}
                data={scan.individual}
                keyExtractor={(row) => row.loan_id}
                emptyMessage={t.ckpnIndividual.candidatesEmpty}
              />
              {!canMark && (
                <div className="mt-2">
                  <Alert variant="info">{t.ckpnIndividual.markReadOnly}</Alert>
                </div>
              )}
            </section>

            <section>
              <h4 className="mb-2 text-title font-semibold text-ink-900">
                {t.ckpnIndividual.excludedTitle}
              </h4>
              {scan.excluded_aset_baik && scan.excluded_aset_baik.length > 0 ? (
                <>
                  <p className="mb-2 text-body text-ink-600">
                    {t.ckpnIndividual.excludedHint}
                  </p>
                  <ul className="flex flex-wrap gap-1">
                    {scan.excluded_aset_baik.map((loanNumber) => (
                      <li key={loanNumber}>
                        <Badge variant="outline">
                          <span className="font-mono">{loanNumber}</span>
                        </Badge>
                      </li>
                    ))}
                  </ul>
                </>
              ) : (
                <p className="text-body text-ink-600">
                  {t.ckpnIndividual.excludedEmpty}
                </p>
              )}
            </section>

            {policy && (
              <section>
                <h4 className="mb-2 text-title font-semibold text-ink-900">
                  {t.ckpnIndividual.policyTitle}
                </h4>
                <DefinitionList
                  items={[
                    {
                      label: t.ckpnIndividual.policyEnabled,
                      value: policy.enabled
                        ? t.ckpnIndividual.policyEnabledOn
                        : t.ckpnIndividual.policyEnabledOff,
                    },
                    {
                      label: t.ckpnIndividual.policySignificanceAmount,
                      value:
                        Number(policy.significance_amount) > 0 ? (
                          <MoneyText value={policy.significance_amount} />
                        ) : (
                          t.ckpnIndividual.policyNotSet
                        ),
                    },
                    {
                      label: t.ckpnIndividual.policySignificanceTopN,
                      value:
                        policy.significance_top_n > 0
                          ? String(policy.significance_top_n)
                          : t.ckpnIndividual.policyNotSet,
                      isMono: true,
                    },
                    {
                      label: t.ckpnIndividual.policyMethod,
                      value: policy.method || t.ckpnIndividual.policyNotSet,
                    },
                    {
                      label: t.ckpnIndividual.policyDiscountRate,
                      value: policy.discount_rate_annual_pct
                        ? formatRate(policy.discount_rate_annual_pct)
                        : t.ckpnIndividual.policyNotSet,
                      isMono: true,
                    },
                    {
                      label: t.ckpnIndividual.policyMandatoryOn,
                      value:
                        mandatoryCodes.length > 0
                          ? mandatoryCodes.join(", ")
                          : t.ckpnIndividual.policyMandatoryNone,
                      isMono: true,
                    },
                  ]}
                />
              </section>
            )}
          </>
        ) : null}
      </CardContent>

      {markTarget && (
        <CKPNIndividualEntryDialog
          candidate={markTarget}
          submitting={submitting}
          error={submitError}
          onCancel={() => {
            if (submitting) return;
            setMarkTarget(null);
            setSubmitError(null);
          }}
          onSubmit={onSubmitMark}
        />
      )}
    </Card>
  );
}
