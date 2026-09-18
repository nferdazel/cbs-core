"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { Account, Customer } from "@cbs/shared-types";
import { ApiError, newIdempotencyKey, request, unwrap } from "@/lib/api";
import { formatDate } from "@/lib/format";
import type {
  BankingProduct,
  COABook,
  Loan,
  LoanSchedule,
  OJKCollectibility,
  ProfitType,
} from "@/lib/operations-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Pagination } from "@/components/ui/Pagination";
import { ErrorState } from "@/components/ui/States";

const PAGE_SIZE = 20;

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

const COLLECTIBILITY_META: Record<
  OJKCollectibility,
  { label: string; variant: "credit" | "accent" | "debit" }
> = {
  "1_LANCAR": { label: "1 - Lancar", variant: "credit" },
  "2_DPK": { label: "2 - Dalam Perhatian Khusus", variant: "accent" },
  "3_KURANG_LANCAR": { label: "3 - Kurang Lancar", variant: "debit" },
  "4_DIRAGUKAN": { label: "4 - Diragukan", variant: "debit" },
  "5_MACET": { label: "5 - Macet", variant: "debit" },
};

function CollectibilityBadge({ value }: { value: OJKCollectibility }) {
  const meta = COLLECTIBILITY_META[value];
  if (!meta) return <Badge variant="outline">{value}</Badge>;
  return <Badge variant={meta.variant}>{meta.label}</Badge>;
}

function profitLabel(type: ProfitType): string {
  if (type === "MARGIN") return "Margin";
  if (type === "BAGI_HASIL") return "Bagi Hasil";
  return "Bunga";
}

function rateLabel(value: string): string {
  const num = Number(value);
  if (!Number.isFinite(num)) return value || "-";
  return `${new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 }).format(num)}%`;
}

function schemeLabel(product?: BankingProduct): string {
  if (!product) return "Tidak diketahui";
  switch (product.profit_scheme) {
    case "MURABAHAH":
      return "Murabahah (margin)";
    case "MUDHARABAH":
      return "Mudharabah (bagi hasil)";
    case "MUSYARAKAH":
      return "Musyarakah (bagi hasil)";
    case "IJARAH":
      return "Ijarah (sewa)";
    case "WADIAH":
      return "Wadiah";
    default:
      return "Bunga (konvensional)";
  }
}

function customerLabel(
  customerId: string,
  names: Record<string, string>
): React.ReactNode {
  const name = names[customerId];
  if (name) return name;
  return <span className="font-mono">{customerId}</span>;
}

function ActionFeedback({
  feedback,
}: {
  feedback: { title: string; reference: string; description?: string };
}) {
  return (
    <div className="rounded-md border border-credit-700/30 bg-credit-50 px-4 py-3">
      <p className="text-title font-medium text-credit-700">{feedback.title}</p>
      <p className="mt-1 text-body text-ink-900">
        Referensi:{" "}
        <span className="font-mono">{feedback.reference}</span>
      </p>
      {feedback.description && (
        <p className="mt-0.5 text-meta text-ink-600">{feedback.description}</p>
      )}
    </div>
  );
}

interface ApplyFormProps {
  book: COABook;
  products: BankingProduct[];
  customers: Customer[];
  onApplied: (loan: Loan) => void;
  onClose: () => void;
}

function LoanApplyForm({
  book,
  products,
  customers,
  onApplied,
  onClose,
}: ApplyFormProps) {
  const syariah = book === "SYARIAH";
  const [customerId, setCustomerId] = useState("");
  const [accountId, setAccountId] = useState("");
  const [productId, setProductId] = useState("");
  const [principal, setPrincipal] = useState(0);
  const [termMonths, setTermMonths] = useState(12);
  const [margin, setMargin] = useState(0);
  const [purpose, setPurpose] = useState("");

  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountsLoading, setAccountsLoading] = useState(false);
  const [accountsError, setAccountsError] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!customerId) {
      setAccounts([]);
      setAccountId("");
      return;
    }
    let cancelled = false;
    setAccountsLoading(true);
    setAccountsError(false);
    request<Account[]>("/accounts?page=1&page_size=100")
      .then((res) => {
        if (cancelled) return;
        setAccounts(
          (res.data ?? []).filter((acc) => acc.customer_id === customerId)
        );
        setAccountId("");
      })
      .catch(() => {
        if (cancelled) return;
        setAccounts([]);
        setAccountsError(true);
      })
      .finally(() => {
        if (!cancelled) setAccountsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [customerId]);

  const selectedProduct = products.find((p) => p.id === productId);

  const validate = (): string | null => {
    if (!customerId) return "Nasabah wajib dipilih.";
    if (!accountId) return "Rekening pencairan wajib dipilih.";
    if (!productId) return "Produk wajib dipilih.";
    if (principal <= 0) return "Pokok harus lebih besar dari nol.";
    if (termMonths <= 0) return "Jangka waktu minimal 1 bulan.";
    return null;
  };

  const openConfirm = (event: React.FormEvent) => {
    event.preventDefault();
    const error = validate();
    if (error) {
      setFormError(error);
      return;
    }
    setFormError(null);
    setConfirmOpen(true);
  };

  const submit = async () => {
    setSubmitting(true);
    setFormError(null);
    try {
      const idempotencyKey = newIdempotencyKey();
      const response = await request<Loan>("/loans/apply", {
        method: "POST",
        idempotencyKey,
        body: {
          customer_id: customerId,
          product_id: productId,
          disbursement_account_id: accountId,
          principal_amount: String(principal),
          term_months: termMonths,
          margin_amount: String(margin || 0),
          purpose: purpose.trim(),
        },
      });
      onApplied(unwrap(response));
      setConfirmOpen(false);
    } catch (err) {
      setFormError(
        err instanceof ApiError ? err.message : "Pengajuan kredit gagal diproses."
      );
      setConfirmOpen(false);
    } finally {
      setSubmitting(false);
    }
  };

  const customerOptions = customers.map((c) => ({
    value: c.id,
    label: `${c.cif_number} — ${c.full_name}`,
  }));
  const accountOptions = accounts.map((a) => ({
    value: a.id,
    label: a.account_number,
  }));
  const productOptions = products.map((p) => ({
    value: p.id,
    label: `${p.code} — ${p.name}`,
  }));

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>Pengajuan {syariah ? "Pembiayaan" : "Kredit"} Baru</CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          Tutup
        </Button>
      </CardHeader>
      <CardContent>
        <form onSubmit={openConfirm} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <Select
              label="Nasabah"
              value={customerId}
              onChange={(e) => setCustomerId(e.target.value)}
              options={customerOptions}
              placeholder="Pilih nasabah"
            />
            <Select
              label="Rekening Pencairan"
              value={accountId}
              onChange={(e) => setAccountId(e.target.value)}
              options={accountOptions}
              placeholder={
                !customerId
                  ? "Pilih nasabah dahulu"
                  : accountsLoading
                    ? "Memuat rekening..."
                    : accountsError
                      ? "Gagal memuat rekening"
                      : accountOptions.length === 0
                        ? "Nasabah belum punya rekening"
                        : "Pilih rekening"
              }
              disabled={!customerId || accountsLoading}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <Select
              label="Produk"
              value={productId}
              onChange={(e) => setProductId(e.target.value)}
              options={productOptions}
              placeholder={
                productOptions.length === 0
                  ? "Tidak ada produk aktif"
                  : "Pilih produk"
              }
            />
            <Input
              label="Jangka Waktu (bulan)"
              type="number"
              min={1}
              value={termMonths}
              onChange={(e) => setTermMonths(Number(e.target.value))}
              isMono
            />
          </div>

          {selectedProduct && (
            <p className="text-meta text-ink-600">
              Rentang produk:{" "}
              <MoneyText value={selectedProduct.min_amount} /> s.d.{" "}
              <MoneyText value={selectedProduct.max_amount} />, tenor{" "}
              <span className="font-mono">
                {selectedProduct.min_term_months}-{selectedProduct.max_term_months}
              </span>{" "}
              bulan.
            </p>
          )}

          <div className="grid grid-cols-2 gap-4">
            <CurrencyInput
              label="Pokok"
              value={principal}
              onChange={setPrincipal}
            />
            <CurrencyInput
              label={syariah ? "Margin / Proyeksi Bagi Hasil" : "Margin (opsional)"}
              value={margin}
              onChange={setMargin}
            />
          </div>

          <Input
            label="Tujuan Penggunaan"
            value={purpose}
            onChange={(e) => setPurpose(e.target.value)}
          />

          {formError && (
            <p className="text-meta text-debit-700" role="alert">
              {formError}
            </p>
          )}

          <Button type="submit">Lanjut Konfirmasi</Button>
        </form>
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        title="Konfirmasi Pengajuan"
        loading={submitting}
        confirmLabel="Kirim Pengajuan"
        onCancel={() => setConfirmOpen(false)}
        onConfirm={submit}
        description={
          <>
            <p>Pengajuan akan dikirim untuk persetujuan pejabat berwenang.</p>
            <dl className="mt-2 space-y-1">
              <div className="flex justify-between">
                <dt className="text-ink-600">Produk</dt>
                <dd>{selectedProduct?.name ?? "-"}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-ink-600">Pokok</dt>
                <dd>
                  <MoneyText value={principal} />
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-ink-600">Jangka waktu</dt>
                <dd className="font-mono">{termMonths} bulan</dd>
              </div>
            </dl>
          </>
        }
      />
    </Card>
  );
}

interface DetailPanelProps {
  loanId: string;
  book: COABook;
  products: BankingProduct[];
  customerNames: Record<string, string>;
  onClose: () => void;
  onChanged: () => void;
}

type ActionKind = "approve" | "reject" | "disburse" | "pay";

const ACTION_LABEL: Record<ActionKind, string> = {
  approve: "Setujui Kredit",
  reject: "Tolak Kredit",
  disburse: "Cairkan Kredit",
  pay: "Bayar Angsuran",
};

function LoanDetailPanel({
  loanId,
  book,
  products,
  customerNames,
  onClose,
  onChanged,
}: DetailPanelProps) {
  const syariah = book === "SYARIAH";
  const [loan, setLoan] = useState<Loan | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [pending, setPending] = useState<ActionKind | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{
    title: string;
    reference: string;
    description?: string;
  } | null>(null);
  const [installmentNo, setInstallmentNo] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Loan>(`/loans/${loanId}`);
      setLoan(unwrap(response));
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Gagal memuat detail kredit."
      );
    } finally {
      setLoading(false);
    }
  }, [loanId]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const productById = useMemo(
    () => Object.fromEntries(products.map((p) => [p.id, p])),
    [products]
  );
  const product = loan?.product_id ? productById[loan.product_id] : undefined;

  const unpaidSchedules = (loan?.schedules ?? []).filter(
    (s) => s.status !== "PAID"
  );

  const openAction = (kind: ActionKind) => {
    setActionError(null);
    if (kind === "pay" && !installmentNo && unpaidSchedules.length > 0) {
      setInstallmentNo(String(unpaidSchedules[0].installment_no));
    }
    setPending(kind);
  };

  const runAction = async () => {
    if (!pending || !loan) return;
    setSubmitting(true);
    setActionError(null);
    try {
      const idempotencyKey = newIdempotencyKey();
      if (pending === "pay") {
        const response = await request<LoanSchedule>(
          `/loans/${loan.id}/pay-installment`,
          {
            method: "POST",
            idempotencyKey,
            body: { installment_no: Number(installmentNo) },
          }
        );
        const schedule = unwrap(response);
        setFeedback({
          title: "Pembayaran angsuran tercatat",
          reference: `Angsuran ke-${schedule.installment_no}`,
          description: `Status ${schedule.status}.`,
        });
      } else {
        const response = await request<Loan>(`/loans/${loan.id}/${pending}`, {
          method: "POST",
          idempotencyKey,
        });
        const updated = unwrap(response);
        setFeedback({
          title: ACTION_LABEL[pending],
          reference: updated.loan_number,
          description: `Status sekarang ${updated.status}.`,
        });
      }
      setPending(null);
      setReloadKey((key) => key + 1);
      onChanged();
    } catch (err) {
      setActionError(
        err instanceof ApiError ? err.message : "Tindakan gagal diproses."
      );
      setPending(null);
    } finally {
      setSubmitting(false);
    }
  };

  const scheduleColumns: Column<LoanSchedule>[] = [
    { header: "Ke", accessorKey: "installment_no", isMono: true },
    {
      header: "Jatuh Tempo",
      cell: (row) => formatDate(row.due_date),
      isMono: true,
    },
    {
      header: "Pokok",
      type: "money",
      cell: (row) => <MoneyText value={row.principal_amount} />,
    },
    {
      header: loan?.schedules?.[0]
        ? profitLabel(loan.schedules[0].profit_type)
        : syariah
          ? "Imbal Hasil"
          : "Bunga",
      type: "money",
      cell: (row) => <MoneyText value={row.profit_amount} />,
    },
    {
      header: "Total",
      type: "money",
      cell: (row) => <MoneyText value={row.total_installment} />,
    },
    {
      header: "Dibayar Pokok",
      type: "money",
      cell: (row) => <MoneyText value={row.paid_principal} />,
    },
    {
      header: "Dibayar Imbal",
      type: "money",
      cell: (row) => <MoneyText value={row.paid_profit} />,
    },
    { header: "Status", accessorKey: "status", type: "status" },
  ];

  if (loading) {
    return (
      <Card className="mt-4">
        <CardContent className="text-body text-ink-600">
          Memuat detail kredit...
        </CardContent>
      </Card>
    );
  }

  if (error || !loan) {
    return (
      <div className="mt-4">
        <ErrorState
          title="Gagal memuat detail kredit"
          description={error ?? "Kredit tidak ditemukan."}
          action={
            <Button variant="secondary" onClick={onClose}>
              Tutup
            </Button>
          }
        />
      </div>
    );
  }

  const canPay = loan.status === "DISBURSED";

  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle>
          Detail Kredit <span className="font-mono">{loan.loan_number}</span>
        </CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          Tutup
        </Button>
      </CardHeader>
      <CardContent>
        {feedback && <ActionFeedback feedback={feedback} />}
        {actionError && (
          <p className="text-body text-debit-700" role="alert">
            {actionError}
          </p>
        )}

        <DefinitionList
          items={[
            { label: "Nasabah", value: customerLabel(loan.customer_id, customerNames) },
            { label: "Produk", value: product ? `${product.code} — ${product.name}` : "-" },
            { label: "Status", value: loan.status },
            {
              label: "Skema Imbal Hasil",
              value: schemeLabel(product),
            },
            { label: "Pokok", value: <MoneyText value={loan.principal_amount} /> },
            {
              label: "Sisa Pokok",
              value: <MoneyText value={loan.outstanding_principal} />,
            },
            {
              label: "Total Kewajiban",
              value: <MoneyText value={loan.total_payable} />,
            },
            {
              label: "Angsuran per Bulan",
              value: <MoneyText value={loan.monthly_installment} />,
            },
            { label: "Jangka Waktu", value: `${loan.term_months} bulan`, isMono: true },
            syariah
              ? {
                  label: "Margin",
                  value: <MoneyText value={loan.margin_amount} />,
                }
              : {
                  label: "Suku Bunga per Tahun",
                  value: rateLabel(loan.interest_rate_annual),
                  isMono: true,
                },
            syariah && Number(loan.profit_sharing_ratio) > 0
              ? {
                  label: "Nisbah Bagi Hasil",
                  value: rateLabel(
                    String(Number(loan.profit_sharing_ratio) * 100)
                  ),
                  isMono: true,
                }
              : null,
            {
              label: "Kolektibilitas",
              value: <CollectibilityBadge value={loan.collectibility} />,
            },
            { label: "DPD", value: `${loan.dpd} hari`, isMono: true },
            { label: "Tujuan", value: loan.purpose || "-" },
          ].filter((item) => item !== null)}
        />

        <div className="flex flex-wrap gap-2">
          {loan.status === "PENDING_APPROVAL" && (
            <>
              <Button size="sm" onClick={() => openAction("approve")}>
                Setujui
              </Button>
              <Button
                size="sm"
                variant="danger"
                onClick={() => openAction("reject")}
              >
                Tolak
              </Button>
            </>
          )}
          {loan.status === "APPROVED" && (
            <Button size="sm" onClick={() => openAction("disburse")}>
              Cairkan
            </Button>
          )}
          {canPay && (
            <>
              <div className="w-72">
                <Select
                  label="Angsuran yang Dibayar"
                  value={installmentNo}
                  onChange={(e) => setInstallmentNo(e.target.value)}
                  options={unpaidSchedules.map((s) => ({
                    value: String(s.installment_no),
                    label: `Ke-${s.installment_no} — jatuh ${formatDate(s.due_date)}`,
                  }))}
                  placeholder={
                    unpaidSchedules.length === 0
                      ? "Semua angsuran sudah lunas"
                      : "Pilih angsuran"
                  }
                />
              </div>
              <div className="flex items-end">
                <Button
                  size="sm"
                  onClick={() => openAction("pay")}
                  disabled={!installmentNo}
                >
                  Bayar Angsuran
                </Button>
              </div>
            </>
          )}
        </div>

        <div>
          <h4 className="mb-2 text-title font-medium text-ink-900">
            Jadwal Angsuran
          </h4>
          <DataTable
            columns={scheduleColumns}
            data={loan.schedules ?? []}
            keyExtractor={(row) => row.id}
            emptyMessage="Jadwal angsuran tidak dikembalikan oleh API."
            zebra
          />
        </div>
      </CardContent>

      <ConfirmDialog
        open={pending !== null}
        title={pending ? ACTION_LABEL[pending] : "Konfirmasi"}
        destructive={pending === "reject"}
        loading={submitting}
        confirmLabel={pending ? ACTION_LABEL[pending] : "Konfirmasi"}
        onCancel={() => setPending(null)}
        onConfirm={runAction}
        description={
          pending === "disburse" ? (
            <>
              <p>
                Dana sebesar <MoneyText value={loan.principal_amount} /> akan
                dicairkan ke rekening nasabah dan jurnal pencairan diposting.
              </p>
              <p className="text-meta">Tindakan ini tidak dapat dibatalkan.</p>
            </>
          ) : pending === "pay" ? (
            <>
              <p>
                Pembayaran angsuran ke-{installmentNo || "-"} akan dicatat dan
                jurnal diterbitkan.
              </p>
              <p className="text-meta">Pastikan angsuran yang dipilih benar.</p>
            </>
          ) : pending === "reject" ? (
            <p>
              Pengajuan kredit {loan.loan_number} akan ditolak. Status menjadi
              REJECTED.
            </p>
          ) : (
            <p>
              Pengajuan kredit <span className="font-mono">{loan.loan_number}</span>{" "}
              akan disetujui. Status menjadi APPROVED dan siap dicairkan.
            </p>
          )
        }
      />
    </Card>
  );
}

export interface LoanWorkspaceProps {
  book: COABook;
}

/**
 * Daftar + detail kredit/pembiayaan. /kredit memakai buku CONVENTIONAL,
 * /pembiayaan memakai buku SYARIAH. Endpoint sama: /loans.
 */
export function LoanWorkspace({ book }: LoanWorkspaceProps) {
  const syariah = book === "SYARIAH";
  const [loans, setLoans] = useState<Loan[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [products, setProducts] = useState<BankingProduct[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const customerNames = useMemo(
    () => Object.fromEntries(customers.map((c) => [c.id, c.full_name])),
    [customers]
  );
  const [formOpen, setFormOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [listReloadKey, setListReloadKey] = useState(0);

  const loadLoans = useCallback(async (targetPage: number) => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Loan[]>(
        `/loans?page=${targetPage}&page_size=${PAGE_SIZE}`
      );
      setLoans(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Gagal memuat daftar kredit."
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadLoans(page);
  }, [page, loadLoans, listReloadKey]);

  useEffect(() => {
    let cancelled = false;
    request<BankingProduct[]>("/products")
      .then((res) => {
        if (!cancelled) setProducts(res.data ?? []);
      })
      .catch(() => undefined);
    request<Customer[]>("/customers?page=1&page_size=100")
      .then((res) => {
        if (!cancelled) setCustomers(res.data ?? []);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  const productById = useMemo(
    () => Object.fromEntries(products.map((p) => [p.id, p])),
    [products]
  );

  const applyProducts = useMemo(
    () =>
      products.filter(
        (p) => p.family === "LOAN" && p.book === book && p.is_active
      ),
    [products, book]
  );

  const productsReady = products.length > 0;
  const visibleLoans = useMemo(() => {
    if (!productsReady) return loans;
    return loans.filter(
      (loan) => loan.product_id && productById[loan.product_id]?.book === book
    );
  }, [loans, productsReady, productById, book]);

  const columns: Column<Loan>[] = [
    { header: "Nomor Kredit", accessorKey: "loan_number", isMono: true },
    {
      header: "Nasabah",
      cell: (row) => customerLabel(row.customer_id, customerNames),
    },
    {
      header: "Pokok",
      type: "money",
      cell: (row) => <MoneyText value={row.principal_amount} />,
    },
    {
      header: "Sisa Pokok",
      type: "money",
      cell: (row) => <MoneyText value={row.outstanding_principal} />,
    },
    { header: "Status", accessorKey: "status", type: "status" },
    ...(syariah
      ? [
          {
            header: "Skema Imbal Hasil",
            cell: (row: Loan) =>
              schemeLabel(row.product_id ? productById[row.product_id] : undefined),
          } as Column<Loan>,
        ]
      : []),
    {
      header: "Kolektibilitas",
      cell: (row) => <CollectibilityBadge value={row.collectibility} />,
    },
    {
      header: "",
      cell: (row) => (
        <Button
          size="sm"
          variant="secondary"
          onClick={() => setSelectedId(row.id)}
        >
          Detail
        </Button>
      ),
    },
  ];

  return (
    <>
      {formOpen && (
        <LoanApplyForm
          book={book}
          products={applyProducts}
          customers={customers}
          onApplied={(loan) => {
            setFormOpen(false);
            setSelectedId(loan.id);
            setPage(1);
            setListReloadKey((key) => key + 1);
          }}
          onClose={() => setFormOpen(false)}
        />
      )}

      {error && !loading ? (
        <ErrorState title="Gagal memuat kredit" description={error} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>
              Daftar {syariah ? "Pembiayaan" : "Kredit"}
            </CardTitle>
            <Button size="sm" onClick={() => setFormOpen((open) => !open)}>
              Ajukan Baru
            </Button>
          </CardHeader>
          <CardContent className="p-0">
            {syariah && productsReady && (
              <p className="border-b border-border px-4 py-2 text-meta text-ink-600">
                Data difilter di sisi klien ke produk buku SYARIAH karena
                /loans belum menyediakan filter buku.
              </p>
            )}
            <DataTable
              columns={columns}
              data={visibleLoans}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={
                syariah
                  ? "Belum ada pembiayaan syariah pada halaman ini."
                  : "Belum ada kredit."
              }
              zebra
            />
            {meta && (
              <Pagination
                page={meta.page}
                pageSize={meta.page_size}
                totalItems={meta.total_items}
                totalPages={meta.total_pages}
                onPageChange={(next) => setPage(next)}
              />
            )}
          </CardContent>
        </Card>
      )}

      {selectedId && (
        <LoanDetailPanel
          loanId={selectedId}
          book={book}
          products={products}
          customerNames={customerNames}
          onClose={() => setSelectedId(null)}
          onChanged={() => setListReloadKey((key) => key + 1)}
        />
      )}
    </>
  );
}
