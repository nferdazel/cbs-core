"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, request } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type {
  OJKMappingReview,
  OJKMappingReviewRow,
} from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { DataTable, Column } from "@/components/ui/DataTable";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { ErrorState, LoadingState } from "@/components/ui/States";

type DecisionFilter = "ALL" | "PENDING" | "APPROVED" | "NOTED";
type ReviewKind = "approve" | "note";

interface ReviewTarget {
  row: OJKMappingReviewRow;
  kind: ReviewKind;
}

/** Bulan lalu dalam format YYYY-MM, sama dengan default backend bila period kosong. */
function defaultPeriod(): string {
  const date = new Date();
  date.setDate(1);
  date.setMonth(date.getMonth() - 1);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

export default function PemetaanOJKPage() {
  const { t } = useTranslation();
  const { user } = useAuth();

  const [period, setPeriod] = useState(defaultPeriod);
  const [book, setBook] = useState("");
  const [queryPeriod, setQueryPeriod] = useState(defaultPeriod);
  const [queryBook, setQueryBook] = useState("");
  const [decisionFilter, setDecisionFilter] = useState<DecisionFilter>("ALL");
  const [search, setSearch] = useState("");

  const [data, setData] = useState<OJKMappingReview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const [target, setTarget] = useState<ReviewTarget | null>(null);
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);

  // Keputusan pemetaan dijaga coa:manage di API; izin efektif dari /auth/me.
  const canDecide = hasPermission(user, "coa:manage");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setTarget(null);
    setActionError(null);
    try {
      const params = new URLSearchParams();
      if (queryPeriod) params.set("period", queryPeriod);
      if (queryBook) params.set("book", queryBook);
      const response = await request<OJKMappingReview>(
        `/reports/ojk/mapping?${params.toString()}`
      );
      setData(response.data ?? null);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : t.ojkMapping.loadError
      );
    } finally {
      setLoading(false);
    }
  }, [queryPeriod, queryBook, reloadKey, t]);

  useEffect(() => {
    load();
  }, [load]);

  const rows = data?.rows ?? [];

  const counts = useMemo(() => {
    let approved = 0;
    let noted = 0;
    for (const row of rows) {
      if (row.decision === "DISETUJUI") approved += 1;
      else if (row.decision === "DICATAT") noted += 1;
    }
    return {
      total: rows.length,
      approved,
      noted,
      pending: rows.length - approved - noted,
    };
  }, [rows]);

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    return rows.filter((row) => {
      if (decisionFilter === "PENDING" && row.decision) return false;
      if (decisionFilter === "APPROVED" && row.decision !== "DISETUJUI") return false;
      if (decisionFilter === "NOTED" && row.decision !== "DICATAT") return false;
      if (term) {
        const haystack = `${row.coa_code} ${row.coa_name}`.toLowerCase();
        if (!haystack.includes(term)) return false;
      }
      return true;
    });
  }, [rows, decisionFilter, search]);

  const openApprove = (row: OJKMappingReviewRow) => {
    setActionError(null);
    setNote("");
    setTarget({ row, kind: "approve" });
  };

  const openNote = (row: OJKMappingReviewRow) => {
    setActionError(null);
    setNote(row.review_note ?? "");
    setTarget({ row, kind: "note" });
  };

  const submitReview = async () => {
    if (!target) return;
    const trimmed = note.trim();
    if (target.kind === "note" && !trimmed) {
      setActionError(t.ojkMapping.noteRequired);
      return;
    }
    setSubmitting(true);
    setActionError(null);
    try {
      await request("/reports/ojk/mapping/decision", {
        method: "POST",
        body: {
          form: target.row.form,
          coa_code: target.row.coa_code,
          decision: target.kind === "approve" ? "DISETUJUI" : "DICATAT",
          note: trimmed,
        },
      });
      setFeedback(
        target.kind === "approve"
          ? t.ojkMapping.feedbackApproved
          : t.ojkMapping.feedbackNoted
      );
      setTarget(null);
      setReloadKey((key) => key + 1);
    } catch (err) {
      // Dialog dibiarkan terbuka agar catatan yang sudah ditulis tidak hilang.
      setActionError(
        err instanceof ApiError ? err.message : t.ojkMapping.actionFailed
      );
    } finally {
      setSubmitting(false);
    }
  };

  const decisionBadge = (row: OJKMappingReviewRow) => {
    if (row.decision === "DISETUJUI") {
      return <Badge variant="credit">{t.ojkMapping.decisionApproved}</Badge>;
    }
    if (row.decision === "DICATAT") {
      return <Badge variant="debit">{t.ojkMapping.decisionNoted}</Badge>;
    }
    return <Badge variant="outline">{t.ojkMapping.decisionPending}</Badge>;
  };

  const columns: Column<OJKMappingReviewRow>[] = [
    { header: t.ojkMapping.colForm, accessorKey: "form", isMono: true },
    {
      header: t.ojkMapping.colCoaCode,
      cell: (row) => (
        <div className="space-y-0.5">
          <span className="font-mono">{row.coa_code}</span>
          {!row.coa_found && (
            <div>
              <Badge variant="debit">{t.ojkMapping.coaUnknown}</Badge>
            </div>
          )}
        </div>
      ),
    },
    {
      header: t.ojkMapping.colCoaName,
      cell: (row) => row.coa_name || "-",
    },
    { header: t.ojkMapping.colSandi, accessorKey: "sandi", isMono: true },
    { header: t.ojkMapping.colPos, accessorKey: "pos_name" },
    {
      header: t.ojkMapping.colDraftStatus,
      cell: (row) => (
        <Badge variant={row.draft_verified ? "credit" : "outline"}>
          {row.draft_verified
            ? t.ojkMapping.draftVerified
            : t.ojkMapping.draftUnverified}
        </Badge>
      ),
    },
    {
      header: t.ojkMapping.colDecision,
      cell: (row) => (
        <div className="space-y-0.5">
          {decisionBadge(row)}
          {row.decided_by && (
            <p className="text-meta text-ink-600">
              {t.ojkMapping.colDecidedBy}: <span className="font-mono">{row.decided_by}</span>
            </p>
          )}
          {row.review_note && <p className="text-meta text-ink-600">{row.review_note}</p>}
        </div>
      ),
    },
    {
      header: t.ojkMapping.colDecidedAt,
      cell: (row) => (row.decided_at ? formatDateTime(row.decided_at) : "-"),
      isMono: true,
    },
    ...(canDecide
      ? [
          {
            header: t.ojkMapping.colActions,
            cell: (row: OJKMappingReviewRow) => (
              <div className="flex flex-wrap gap-2">
                <Button size="sm" onClick={() => openApprove(row)}>
                  {t.ojkMapping.approve}
                </Button>
                <Button size="sm" variant="secondary" onClick={() => openNote(row)}>
                  {t.ojkMapping.note}
                </Button>
              </div>
            ),
          } satisfies Column<OJKMappingReviewRow>,
        ]
      : []),
  ];

  const unmappedCoa = data?.unmapped_coa ?? [];
  const unmappedPositions = data?.unmapped_positions ?? [];

  return (
    <>
      <PageHeader title={t.ojkMapping.title} description={t.ojkMapping.description} />

      <Card className="mb-4">
        <CardContent>
          <p className="text-body text-ink-900">{t.ojkMapping.draftNotice}</p>
          {!canDecide && (
            <p className="text-meta text-ink-600">{t.ojkMapping.readOnlyHint}</p>
          )}
          {data?.source_unbalanced && (
            <p className="text-body text-debit-700" role="alert">
              {t.ojkMapping.unbalancedWarning}
            </p>
          )}
        </CardContent>
      </Card>

      {feedback && (
        <div
          className="mb-4 rounded-md border border-credit-700/30 bg-credit-50 px-4 py-3"
          role="status"
        >
          <p className="text-body text-credit-700">{feedback}</p>
        </div>
      )}

      <Card className="mb-4">
        <CardContent className="flex flex-wrap items-end gap-4">
          <div className="w-40">
            <Input
              label={t.ojkMapping.periodLabel}
              type="month"
              value={period}
              isMono
              onChange={(event) => setPeriod(event.target.value)}
            />
          </div>
          <div className="w-48">
            <Select
              label={t.ojkMapping.bookLabel}
              value={book}
              onChange={(event) => setBook(event.target.value)}
              options={[
                { value: "", label: t.ojkMapping.bookAll },
                { value: "CONVENTIONAL", label: t.ojkMapping.bookConventional },
                { value: "SYARIAH", label: t.ojkMapping.bookSyariah },
              ]}
            />
          </div>
          <div className="w-56">
            <Select
              label={t.ojkMapping.decisionFilterLabel}
              value={decisionFilter}
              onChange={(event) =>
                setDecisionFilter(event.target.value as DecisionFilter)
              }
              options={[
                { value: "ALL", label: t.ojkMapping.filterAll },
                { value: "PENDING", label: t.ojkMapping.filterPending },
                { value: "APPROVED", label: t.ojkMapping.filterApproved },
                { value: "NOTED", label: t.ojkMapping.filterNoted },
              ]}
            />
          </div>
          <div className="w-64">
            <Input
              label={t.ojkMapping.searchLabel}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t.ojkMapping.searchPlaceholder}
            />
          </div>
          <Button
            onClick={() => {
              setQueryPeriod(period);
              setQueryBook(book);
              setReloadKey((key) => key + 1);
            }}
            loading={loading}
          >
            {t.ojkMapping.refresh}
          </Button>
        </CardContent>
      </Card>

      {error && !loading ? (
        <ErrorState title={t.ojkMapping.title} description={error} />
      ) : loading && !data ? (
        <LoadingState />
      ) : (
        <>
          <div className="mb-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
            {[
              { label: t.ojkMapping.totalLabel, value: counts.total },
              { label: t.ojkMapping.approvedCountLabel, value: counts.approved },
              { label: t.ojkMapping.notedCountLabel, value: counts.noted },
              { label: t.ojkMapping.pendingCountLabel, value: counts.pending },
            ].map((item) => (
              <Card key={item.label}>
                <CardContent className="p-4">
                  <p className="text-meta text-ink-600">{item.label}</p>
                  <p className="text-title font-semibold text-ink-900">{item.value}</p>
                </CardContent>
              </Card>
            ))}
          </div>
          {/* Ringkasan hasil saringan diumumkan ke pembaca layar. */}
          <p className="sr-only" role="status" aria-live="polite">
            {filtered.length} / {counts.total}
          </p>

          <Card className="mb-4">
            <CardHeader>
              <CardTitle>{t.ojkMapping.unmappedCoaTitle}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-meta text-ink-600">{t.ojkMapping.unmappedCoaDesc}</p>
              {unmappedCoa.length === 0 ? (
                <p className="text-body text-ink-600">{t.ojkMapping.unmappedCoaEmpty}</p>
              ) : (
                <ul className="space-y-1">
                  {unmappedCoa.map((item) => (
                    <li key={`${item.form}/${item.coa_code}`} className="text-body text-ink-900">
                      <span className="font-mono">{item.coa_code}</span>
                      {item.coa_name ? ` — ${item.coa_name}` : ""}
                      <span className="text-meta text-ink-600"> ({item.form})</span>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>

          <Card className="mb-4">
            <CardHeader>
              <CardTitle>{t.ojkMapping.unmappedPosTitle}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-meta text-ink-600">{t.ojkMapping.unmappedPosDesc}</p>
              {unmappedPositions.length === 0 ? (
                <p className="text-body text-ink-600">{t.ojkMapping.unmappedPosEmpty}</p>
              ) : (
                <ul className="space-y-1">
                  {unmappedPositions.map((item) => (
                    <li key={`${item.form}/${item.sandi}`} className="text-body text-ink-900">
                      <span className="font-mono">{item.sandi}</span> — {item.pos_name}
                      <span className="text-meta text-ink-600"> ({item.form})</span>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-0">
              <DataTable
                columns={columns}
                data={filtered}
                keyExtractor={(row) => `${row.form}/${row.coa_code}`}
                loading={loading}
                emptyMessage={t.ojkMapping.emptyFiltered}
                zebra
              />
            </CardContent>
          </Card>
        </>
      )}

      <ConfirmDialog
        open={target !== null}
        title={
          target?.kind === "note"
            ? t.ojkMapping.noteTitle
            : t.ojkMapping.approveTitle
        }
        loading={submitting}
        destructive={target?.kind === "note"}
        confirmLabel={
          target?.kind === "note"
            ? t.ojkMapping.noteConfirm
            : t.ojkMapping.approve
        }
        onCancel={() => {
          setTarget(null);
          setActionError(null);
        }}
        onConfirm={submitReview}
        description={
          target && (
            <>
              <dl className="space-y-1">
                <div className="flex justify-between gap-4">
                  <dt className="text-ink-600">{t.ojkMapping.dialogCoa}</dt>
                  <dd className="font-mono">{target.row.coa_code}</dd>
                </div>
                <div className="flex justify-between gap-4">
                  <dt className="text-ink-600">{t.ojkMapping.dialogPos}</dt>
                  <dd>{target.row.pos_name || target.row.sandi}</dd>
                </div>
              </dl>
              {target.kind === "note" && (
                <div className="space-y-1">
                  <label
                    htmlFor="ojk-mapping-note"
                    className="block text-meta font-medium text-ink-900"
                  >
                    {t.ojkMapping.noteField}
                  </label>
                  <textarea
                    id="ojk-mapping-note"
                    value={note}
                    onChange={(event) => setNote(event.target.value)}
                    placeholder={t.ojkMapping.notePlaceholder}
                    rows={3}
                    disabled={submitting}
                    className="w-full rounded-md border border-border-strong bg-surface px-3 py-2 text-body text-ink-900 placeholder:text-ink-400 focus:border-navy-600 focus:outline-none focus:ring-1 focus:ring-navy-600 disabled:opacity-50"
                  />
                </div>
              )}
              {actionError && (
                <p className="text-body text-debit-700" role="alert">
                  {actionError}
                </p>
              )}
            </>
          )
        }
      />
    </>
  );
}
