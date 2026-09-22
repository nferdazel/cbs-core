"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, isCrossBranchError, request } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { MakerCheckerRequest } from "@/lib/operations-types";
import { useAuth } from "@/lib/useAuth";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Input } from "@/components/ui/Input";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { ErrorState } from "@/components/ui/States";

type ReviewKind = "approve" | "reject";

interface ReviewTarget {
  request: MakerCheckerRequest;
  kind: ReviewKind;
}

function payloadAmount(request: MakerCheckerRequest): string | number | null {
  const amount = request.payload?.amount;
  if (amount === undefined || amount === null) return null;
  return amount as string | number;
}

export default function PersetujuanPage() {
  const { user } = useAuth();
  const { t } = useTranslation();
  const [requests, setRequests] = useState<MakerCheckerRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const [target, setTarget] = useState<ReviewTarget | null>(null);
  const [notes, setNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{ title: string; reference: string } | null>(
    null
  );

  // Jenis aksi disimpan sebagai kunci (DEPOSIT dll.); hanya tampilannya dipetakan
  // ke kamus. Jenis yang tidak dikenal ditampilkan apa adanya.
  const actionLabel = (actionType: string): string => {
    switch (actionType) {
      case "DEPOSIT":
        return t.approvals.actionDeposit;
      case "WITHDRAWAL":
        return t.approvals.actionWithdrawal;
      case "TRANSFER":
        return t.approvals.actionTransfer;
      default:
        return actionType;
    }
  };

  const reviewTitle = (kind: ReviewKind): string =>
    kind === "approve"
      ? t.approvals.reviewApproveTitle
      : t.approvals.reviewRejectTitle;

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<MakerCheckerRequest[]>(
        "/maker-checker/pending"
      );
      setRequests(response.data ?? []);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : t.approvals.loadError
      );
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const openReview = (request: MakerCheckerRequest, kind: ReviewKind) => {
    setActionError(null);
    setNotes("");
    setTarget({ request, kind });
  };

  const submitReview = async () => {
    if (!target) return;
    setSubmitting(true);
    setActionError(null);
    try {
      await request(`/maker-checker/${target.request.id}/${target.kind}`, {
        method: "POST",
        body: { notes: notes.trim() },
      });
      setFeedback({
        title:
          target.kind === "approve"
            ? t.approvals.feedbackApprovedTitle
            : t.approvals.feedbackRejectedTitle,
        reference: target.request.id,
      });
      setTarget(null);
      setReloadKey((key) => key + 1);
    } catch (err) {
      if (err instanceof ApiError) {
        // Penolakan lintas cabang sudah membawa pesan yang jelas dari backend;
        // jangan dibungkus lagi agar tidak terkesan sekadar izin kurang.
        if (err.status === 403 && !isCrossBranchError(err)) {
          setActionError(
            `${t.approvals.cannotPrefix}${
              target.kind === "approve"
                ? t.approvals.cannotApprove
                : t.approvals.cannotReject
            }${t.approvals.cannotSuffix}${err.message}`
          );
        } else {
          setActionError(err.message);
        }
      } else {
        setActionError(t.approvals.actionFailed);
      }
      setTarget(null);
    } finally {
      setSubmitting(false);
    }
  };

  const columns: Column<MakerCheckerRequest>[] = [
    { header: t.approvals.colActionType, cell: (row) => actionLabel(row.action_type) },
    {
      header: t.approvals.colAmount,
      type: "money",
      cell: (row) => <MoneyText value={payloadAmount(row)} />,
    },
    {
      header: t.approvals.colMaker,
      cell: (row) => (
        <div className="space-y-0.5">
          <span className="font-mono">{row.maker_id}</span>
          {user?.id === row.maker_id && (
            <div>
              <Badge variant="accent">{t.approvals.badgeOwn}</Badge>
            </div>
          )}
        </div>
      ),
    },
    {
      header: t.approvals.colTime,
      cell: (row) => formatDateTime(row.created_at),
      isMono: true,
    },
    { header: t.common.status, accessorKey: "status", type: "status" },
    {
      header: t.common.actions,
      cell: (row) => (
        <div className="flex gap-2">
          <Button size="sm" onClick={() => openReview(row, "approve")}>
            {t.approvals.approveButton}
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => openReview(row, "reject")}
          >
            {t.approvals.rejectButton}
          </Button>
        </div>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title={t.approvals.title}
        description={t.approvals.description}
      />

      {feedback && (
        <Alert variant="success" className="mb-4" title={feedback.title}>
          {t.approvals.referenceLabel}{" "}
          <span className="font-mono">{feedback.reference}</span>
        </Alert>
      )}

      {actionError && (
        <Alert variant="error" className="mb-4">
          {actionError}
        </Alert>
      )}

      {error && !loading ? (
        <ErrorState title={t.approvals.errorTitle} description={error} />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={requests}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={t.approvals.empty}
              zebra
            />
          </CardContent>
        </Card>
      )}

      <ConfirmDialog
        open={target !== null}
        title={target ? reviewTitle(target.kind) : t.common.confirm}
        destructive={target?.kind === "reject"}
        loading={submitting}
        confirmLabel={target ? reviewTitle(target.kind) : t.common.confirm}
        onCancel={() => setTarget(null)}
        onConfirm={submitReview}
        description={
          target && (
            <>
              <p>
                {target.kind === "approve"
                  ? t.approvals.descApprove
                  : t.approvals.descReject}
              </p>
              <dl className="space-y-1">
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.approvals.labelActionType}</dt>
                  <dd>{actionLabel(target.request.action_type)}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.approvals.colAmount}</dt>
                  <dd>
                    <MoneyText value={payloadAmount(target.request)} />
                  </dd>
                </div>
              </dl>
              <Input
                label={t.approvals.notesLabel}
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
            </>
          )
        }
      />
    </>
  );
}
