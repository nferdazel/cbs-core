"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, isCrossBranchError, request } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { MakerCheckerRequest } from "@/lib/operations-types";
import { useAuth } from "@/lib/useAuth";
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

const REVIEW_TITLE: Record<ReviewKind, string> = {
  approve: "Setujui Permintaan",
  reject: "Tolak Permintaan",
};

function payloadAmount(request: MakerCheckerRequest): string | number | null {
  const amount = request.payload?.amount;
  if (amount === undefined || amount === null) return null;
  return amount as string | number;
}

const ACTION_LABEL: Record<string, string> = {
  DEPOSIT: "Setoran",
  WITHDRAWAL: "Penarikan",
  TRANSFER: "Transfer",
};

function actionLabel(actionType: string): string {
  return ACTION_LABEL[actionType] ?? actionType;
}

export default function PersetujuanPage() {
  const { user } = useAuth();
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
          : "Gagal memuat antrean persetujuan."
      );
    } finally {
      setLoading(false);
    }
  }, []);

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
            ? "Permintaan disetujui"
            : "Permintaan ditolak",
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
            `Tidak dapat ${
              target.kind === "approve" ? "menyetujui" : "menolak"
            } (403): ${err.message}`
          );
        } else {
          setActionError(err.message);
        }
      } else {
        setActionError("Tindakan gagal diproses.");
      }
      setTarget(null);
    } finally {
      setSubmitting(false);
    }
  };

  const columns: Column<MakerCheckerRequest>[] = [
    { header: "Jenis Aksi", cell: (row) => actionLabel(row.action_type) },
    {
      header: "Nominal",
      type: "money",
      cell: (row) => <MoneyText value={payloadAmount(row)} />,
    },
    {
      header: "Pembuat",
      cell: (row) => (
        <div className="space-y-0.5">
          <span className="font-mono">{row.maker_id}</span>
          {user?.id === row.maker_id && (
            <div>
              <Badge variant="accent">Permintaan Anda</Badge>
            </div>
          )}
        </div>
      ),
    },
    {
      header: "Waktu",
      cell: (row) => formatDateTime(row.created_at),
      isMono: true,
    },
    { header: "Status", accessorKey: "status", type: "status" },
    {
      header: "Aksi",
      cell: (row) => (
        <div className="flex gap-2">
          <Button size="sm" onClick={() => openReview(row, "approve")}>
            Setujui
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => openReview(row, "reject")}
          >
            Tolak
          </Button>
        </div>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Persetujuan"
        description="Antrean maker-checker. Pembuat permintaan tidak dapat menyetujui permintaannya sendiri."
      />

      {feedback && (
        <div className="mb-4 rounded-md border border-credit-700/30 bg-credit-50 px-4 py-3">
          <p className="text-title font-medium text-credit-700">
            {feedback.title}
          </p>
          <p className="mt-1 text-body text-ink-900">
            Referensi: <span className="font-mono">{feedback.reference}</span>
          </p>
        </div>
      )}

      {actionError && (
        <div
          className="mb-4 rounded-md border border-debit-700/30 bg-debit-50 px-4 py-3"
          role="alert"
        >
          <p className="text-body text-debit-700">{actionError}</p>
        </div>
      )}

      {error && !loading ? (
        <ErrorState title="Gagal memuat antrean" description={error} />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={requests}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage="Tidak ada permintaan yang menunggu persetujuan."
              zebra
            />
          </CardContent>
        </Card>
      )}

      <ConfirmDialog
        open={target !== null}
        title={target ? REVIEW_TITLE[target.kind] : "Konfirmasi"}
        destructive={target?.kind === "reject"}
        loading={submitting}
        confirmLabel={target ? REVIEW_TITLE[target.kind] : "Konfirmasi"}
        onCancel={() => setTarget(null)}
        onConfirm={submitReview}
        description={
          target && (
            <>
              <p>
                {target.kind === "approve"
                  ? "Menyetujui permintaan akan langsung memposting efek transaksinya."
                  : "Menolak permintaan hanya mengubah status; tidak ada jurnal yang diposting."}
              </p>
              <dl className="space-y-1">
                <div className="flex justify-between">
                  <dt className="text-ink-600">Jenis aksi</dt>
                  <dd>{actionLabel(target.request.action_type)}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">Nominal</dt>
                  <dd>
                    <MoneyText value={payloadAmount(target.request)} />
                  </dd>
                </div>
              </dl>
              <Input
                label="Catatan (opsional)"
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
