"use client";

import React, { useEffect, useState } from "react";
import type {
  CKPNIndividualCandidate,
  CKPNIndividualEntryMarkInput,
  CKPNIndividualMethod,
} from "@/lib/types";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { MoneyText } from "@/components/ui/MoneyText";
import { Select } from "@/components/ui/Select";

const METHODS: CKPNIndividualMethod[] = ["DCF", "COLLATERAL", "MAX"];

function isMethod(value: string): value is CKPNIndividualMethod {
  return (METHODS as string[]).includes(value);
}

export interface CKPNIndividualEntryDialogProps {
  candidate: CKPNIndividualCandidate;
  submitting: boolean;
  error: string | null;
  onCancel: () => void;
  onSubmit: (input: CKPNIndividualEntryMarkInput) => void;
}

/**
 * Penandaan keputusan pintu masuk individual (T3). Dua langkah dalam satu dialog:
 * isi keputusan, lalu tinjau ringkasan dan konsekuensinya sebelum menyimpan.
 * Penandaan tidak menulis jurnal, jadi langkah tinjau memastikan pengelola sadar
 * apa yang dicatat pada jejak audit. Bisa ditutup dengan Escape.
 */
export function CKPNIndividualEntryDialog({
  candidate,
  submitting,
  error,
  onCancel,
  onSubmit,
}: CKPNIndividualEntryDialogProps) {
  const { t } = useTranslation();
  const [step, setStep] = useState<"form" | "review">("form");
  const [method, setMethod] = useState<CKPNIndividualMethod>(() =>
    isMethod(candidate.entry.suggested_method)
      ? candidate.entry.suggested_method
      : "DCF",
  );
  const [significant, setSignificant] = useState(candidate.entry.significance);
  const [objectiveEvidence, setObjectiveEvidence] = useState(false);
  const [excluded, setExcluded] = useState(false);

  useEffect(() => {
    if (submitting) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onCancel();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onCancel, submitting]);

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

  const methodOptions = METHODS.map((value) => ({
    value,
    label: methodLabel(value),
  }));

  const yesNo = (value: boolean) =>
    value ? t.ckpnIndividual.yes : t.ckpnIndividual.no;

  const recordedSignificant = excluded ? false : significant;
  const recordedObjectiveEvidence = excluded ? false : objectiveEvidence;

  const onFormSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setStep("review");
  };

  const onConfirm = () => {
    onSubmit({
      method,
      significant: recordedSignificant,
      objective_evidence: recordedObjectiveEvidence,
      excluded_aset_baik: excluded,
    });
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-6"
      onClick={() => {
        if (!submitting) onCancel();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t.ckpnIndividual.dialogTitle}
        className="w-full max-w-lg rounded-md border border-border bg-surface shadow-overlay"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center justify-between gap-4 border-b border-border px-4 py-3">
          <h2 className="text-title font-semibold text-ink-900">
            {step === "form"
              ? t.ckpnIndividual.dialogTitle
              : t.ckpnIndividual.reviewTitle}
          </h2>
          <Button
            variant="ghost"
            size="sm"
            type="button"
            onClick={onCancel}
            disabled={submitting}
          >
            {t.common.close}
          </Button>
        </div>

        {step === "form" ? (
          <form onSubmit={onFormSubmit}>
            <div className="space-y-4 px-4 py-4">
              <DefinitionList
                items={[
                  {
                    label: t.ckpnIndividual.dialogLoan,
                    value: (
                      <span className="font-mono">{candidate.loan_number}</span>
                    ),
                  },
                  {
                    label: t.ckpnIndividual.dialogOutstanding,
                    value: <MoneyText value={candidate.outstanding} />,
                  },
                  {
                    label: t.ckpnIndividual.dialogSuggested,
                    value: methodLabel(candidate.entry.suggested_method),
                  },
                ]}
              />

              <Select
                label={t.ckpnIndividual.formMethod}
                helperText={t.ckpnIndividual.formMethodHint}
                value={method}
                autoFocus
                disabled={excluded || submitting}
                onChange={(event) => {
                  const value = event.target.value;
                  if (isMethod(value)) setMethod(value);
                }}
                options={methodOptions}
              />

              <label className="flex items-start gap-2 text-body text-ink-900">
                <input
                  type="checkbox"
                  className="mt-0.5 h-4 w-4 rounded-sm border border-border-strong"
                  checked={significant}
                  disabled={excluded || submitting}
                  onChange={(event) => setSignificant(event.target.checked)}
                />
                <span>
                  {t.ckpnIndividual.formSignificant}
                  <span className="mt-0.5 block text-meta text-ink-600">
                    {t.ckpnIndividual.formSignificantHint}
                  </span>
                </span>
              </label>

              <label className="flex items-start gap-2 text-body text-ink-900">
                <input
                  type="checkbox"
                  className="mt-0.5 h-4 w-4 rounded-sm border border-border-strong"
                  checked={objectiveEvidence}
                  disabled={excluded || submitting}
                  onChange={(event) =>
                    setObjectiveEvidence(event.target.checked)
                  }
                />
                <span>
                  {t.ckpnIndividual.formObjectiveEvidence}
                  <span className="mt-0.5 block text-meta text-ink-600">
                    {t.ckpnIndividual.formObjectiveEvidenceHint}
                  </span>
                </span>
              </label>

              <label className="flex items-start gap-2 text-body text-ink-900">
                <input
                  type="checkbox"
                  className="mt-0.5 h-4 w-4 rounded-sm border border-border-strong"
                  checked={excluded}
                  disabled={submitting}
                  onChange={(event) => setExcluded(event.target.checked)}
                />
                <span>{t.ckpnIndividual.formExcluded}</span>
              </label>

              {excluded && (
                <Alert
                  variant="warning"
                  title={t.ckpnIndividual.excludedWarningTitle}
                >
                  {t.ckpnIndividual.excludedWarningBody}
                </Alert>
              )}

              {error && <Alert variant="error">{error}</Alert>}
            </div>

            <div className="flex justify-end gap-2 border-t border-border px-4 py-3">
              <Button
                variant="secondary"
                type="button"
                onClick={onCancel}
                disabled={submitting}
              >
                {t.common.cancel}
              </Button>
              <Button type="submit" disabled={submitting}>
                {t.ckpnIndividual.reviewButton}
              </Button>
            </div>
          </form>
        ) : (
          <>
            <div className="space-y-4 px-4 py-4">
              <DefinitionList
                columns={1}
                items={[
                  {
                    label: t.ckpnIndividual.dialogLoan,
                    value: (
                      <span className="font-mono">{candidate.loan_number}</span>
                    ),
                  },
                  {
                    label: t.ckpnIndividual.reviewMethod,
                    value: excluded
                      ? t.ckpnIndividual.methodIgnored
                      : methodLabel(method),
                  },
                  {
                    label: t.ckpnIndividual.reviewSignificant,
                    value: yesNo(recordedSignificant),
                  },
                  {
                    label: t.ckpnIndividual.reviewObjectiveEvidence,
                    value: yesNo(recordedObjectiveEvidence),
                  },
                  {
                    label: t.ckpnIndividual.reviewExcluded,
                    value: yesNo(excluded),
                  },
                ]}
              />

              <Alert
                variant={excluded ? "warning" : "info"}
                title={t.ckpnIndividual.reviewWhat}
              >
                {excluded
                  ? t.ckpnIndividual.reviewConsequenceExcluded
                  : t.ckpnIndividual.reviewConsequence}
              </Alert>

              {error && <Alert variant="error">{error}</Alert>}
            </div>

            <div className="flex justify-end gap-2 border-t border-border px-4 py-3">
              <Button
                variant="secondary"
                type="button"
                onClick={() => setStep("form")}
                disabled={submitting}
              >
                {t.ckpnIndividual.back}
              </Button>
              <Button
                type="button"
                autoFocus
                loading={submitting}
                onClick={onConfirm}
              >
                {t.ckpnIndividual.submitButton}
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
