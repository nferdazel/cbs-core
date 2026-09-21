"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, newIdempotencyKey, request, unwrap } from "@/lib/api";
import { formatDate } from "@/lib/format";
import type {
  AROInstruction,
  BankingProduct,
  Deposit,
  DepositPreview,
  DepositStatus,
  ProfitType,
} from "@/lib/operations-types";
import { PageHeader } from "@/components/ui/PageHeader";
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

const DEPOSIT_STATUS: Record<
  DepositStatus,
  { variant: "neutral" | "accent" | "credit" | "debit" }
> = {
  PLACED: { variant: "credit" },
  MATURED: { variant: "accent" },
  CLOSED: { variant: "neutral" },
  BROKEN: { variant: "debit" },
};

function DepositStatusBadge({ status }: { status: DepositStatus }) {
  return <Badge variant={DEPOSIT_STATUS[status]?.variant ?? "outline"}>{status}</Badge>;
}

function profitTypeLabel(type: ProfitType): string {
  if (type === "MARGIN") return "Margin";
  if (type === "BAGI_HASIL") return "Bagi Hasil";
  return "Bunga";
}

function rateLabel(value: string): string {
  const num = Number(value);
  if (!Number.isFinite(num)) return value || "-";
  return `${new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 }).format(num)}%`;
}

function aroLabel(instruction: AROInstruction): string {
  if (instruction === "PRINCIPAL_AND_PROFIT") return "Pokok + imbal hasil";
  if (instruction === "PRINCIPAL") return "Pokok saja";
  return "Tanpa ARO";
}

function customerLabel(
  customerId: string,
  names: Record<string, string>
): React.ReactNode {
  const name = names[customerId];
  if (name) return name;
  return <span className="font-mono">{customerId}</span>;
}

interface PlaceFormProps {
  products: BankingProduct[];
  customers: Customer[];
  onPlaced: (deposit: Deposit) => void;
  onClose: () => void;
}

function DepositPlaceForm({
  products,
  customers,
  onPlaced,
  onClose,
}: PlaceFormProps) {
  const [customerId, setCustomerId] = useState("");
  const [productId, setProductId] = useState("");
  const [amount, setAmount] = useState(0);
  const [termMonths, setTermMonths] = useState(12);
  const [aro, setAro] = useState(false);
  const [instruction, setInstruction] = useState<AROInstruction>("PRINCIPAL");
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [preview, setPreview] = useState<DepositPreview | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const selectedProduct = products.find((p) => p.id === productId);

  const validate = (): string | null => {
    if (!customerId) return "Nasabah wajib dipilih.";
    if (!productId) return "Produk wajib dipilih.";
    if (amount <= 0) return "Pokok harus lebih besar dari nol.";
    if (termMonths <= 0) return "Jangka waktu minimal 1 bulan.";
    return null;
  };

  // Pratinjau dihitung backend lewat /deposits/preview, memakai fungsi yang sama
  // dengan akrual. Frontend sengaja TIDAK menghitung bunga/pajak sendiri agar angka
  // yang diperiksa teller identik dengan yang nanti diposting.
  const openConfirm = async (event: React.FormEvent) => {
    event.preventDefault();
    const error = validate();
    if (error) {
      setFormError(error);
      return;
    }
    setFormError(null);
    setPreviewLoading(true);
    try {
      const response = await request<DepositPreview>("/deposits/preview", {
        method: "POST",
        body: {
          customer_id: customerId,
          product_id: productId,
          placement_amount: String(amount),
          term_months: termMonths,
          currency: "IDR",
          aro,
          aro_instruction: aro ? instruction : "NONE",
        },
      });
      setPreview(unwrap(response));
      setConfirmOpen(true);
    } catch (err) {
      setPreview(null);
      setFormError(
        err instanceof ApiError ? err.message : "Pratinjau deposito gagal dihitung."
      );
    } finally {
      setPreviewLoading(false);
    }
  };

  const submit = async () => {
    setSubmitting(true);
    setFormError(null);
    try {
      const idempotencyKey = newIdempotencyKey();
      const response = await request<Deposit>("/deposits/place", {
        method: "POST",
        idempotencyKey,
        body: {
          customer_id: customerId,
          product_id: productId,
          placement_amount: String(amount),
          term_months: termMonths,
          currency: "IDR",
          aro,
          aro_instruction: aro ? instruction : "NONE",
        },
      });
      onPlaced(unwrap(response));
      setConfirmOpen(false);
    } catch (err) {
      setFormError(
        err instanceof ApiError ? err.message : "Penempatan deposito gagal."
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
  const productOptions = products.map((p) => ({
    value: p.id,
    label: `${p.code} — ${p.name}`,
  }));

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>Penempatan Deposito Baru</CardTitle>
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
              label="Produk Deposito"
              value={productId}
              onChange={(e) => setProductId(e.target.value)}
              options={productOptions}
              placeholder={
                productOptions.length === 0
                  ? "Tidak ada produk deposito aktif"
                  : "Pilih produk"
              }
            />
          </div>

          {selectedProduct && (
            <p className="text-meta text-ink-600">
              Rentang produk: <MoneyText value={selectedProduct.min_amount} /> s.d.{" "}
              <MoneyText value={selectedProduct.max_amount} />, tenor{" "}
              <span className="font-mono">
                {selectedProduct.min_term_months}-{selectedProduct.max_term_months}
              </span>{" "}
              bulan.
            </p>
          )}

          <div className="grid grid-cols-2 gap-4">
            <CurrencyInput label="Pokok" value={amount} onChange={setAmount} />
            <Input
              label="Jangka Waktu (bulan)"
              type="number"
              min={1}
              value={termMonths}
              onChange={(e) => setTermMonths(Number(e.target.value))}
              isMono
            />
          </div>

          <div className="flex items-end gap-4">
            <label className="flex items-center gap-2 text-body text-ink-900">
              <input
                type="checkbox"
                checked={aro}
                onChange={(e) => setAro(e.target.checked)}
                className="h-4 w-4 rounded-sm border border-border-strong"
              />
              Perpanjangan otomatis (ARO)
            </label>
            {aro && (
              <div className="w-64">
                <Select
                  label="Instruksi ARO"
                  value={instruction}
                  onChange={(e) =>
                    setInstruction(e.target.value as AROInstruction)
                  }
                  options={[
                    { value: "PRINCIPAL", label: "Pokok saja" },
                    {
                      value: "PRINCIPAL_AND_PROFIT",
                      label: "Pokok + imbal hasil (kapitalisasi)",
                    },
                  ]}
                />
              </div>
            )}
          </div>

          {formError && (
            <p className="text-meta text-debit-700" role="alert">
              {formError}
            </p>
          )}

          <Button type="submit" loading={previewLoading}>
            {previewLoading ? "Menghitung pratinjau..." : "Lanjut Konfirmasi"}
          </Button>
        </form>
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        title="Konfirmasi Penempatan Deposito"
        loading={submitting}
        confirmLabel="Tempatkan Deposito"
        onCancel={() => setConfirmOpen(false)}
        onConfirm={submit}
        description={
          preview ? (
            <>
              <p>
                Periksa proyeksi perhitungan di bawah. Jurnal penempatan baru
                dibuat setelah Anda menekan simpan.
              </p>
              <dl className="mt-2 space-y-1">
                <div className="flex justify-between">
                  <dt className="text-ink-600">Produk</dt>
                  <dd>{selectedProduct?.name ?? "-"}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">Nominal</dt>
                  <dd>
                    <MoneyText value={preview.placement_amount} />
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">Jangka waktu</dt>
                  <dd className="font-mono">{preview.term_months} bulan</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">Jatuh tempo</dt>
                  <dd className="font-mono">
                    {formatDate(preview.maturity_date)}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">
                    {preview.profit_type === "MARGIN"
                      ? "Margin"
                      : preview.profit_type === "BAGI_HASIL"
                        ? "Nisbah Pemilik Dana"
                        : "Bunga per Tahun"}
                  </dt>
                  <dd className="font-mono">
                    {preview.profit_type === "BAGI_HASIL"
                      ? rateLabel(String(Number(preview.profit_rate) * 100))
                      : rateLabel(preview.profit_rate)}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">
                    Estimasi {profitTypeLabel(preview.profit_type)}
                  </dt>
                  <dd>
                    <MoneyText value={preview.estimated_profit} />
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">Estimasi Pajak (PPh)</dt>
                  <dd>
                    <MoneyText value={preview.estimated_tax} />
                  </dd>
                </div>
                <div className="flex justify-between font-medium">
                  <dt>Nilai Jatuh Tempo</dt>
                  <dd>
                    <MoneyText value={preview.maturity_proceeds} />
                  </dd>
                </div>
              </dl>
            </>
          ) : (
            <p>Rincian pratinjau tidak tersedia.</p>
          )
        }
      />
    </Card>
  );
}

interface DetailPanelProps {
  depositId: string;
  products: BankingProduct[];
  customerNames: Record<string, string>;
  onClose: () => void;
  onChanged: () => void;
}

function DepositDetailPanel({
  depositId,
  products,
  customerNames,
  onClose,
  onChanged,
}: DetailPanelProps) {
  const [deposit, setDeposit] = useState<Deposit | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [pending, setPending] = useState<"accrue" | "withdraw" | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{
    title: string;
    reference: string;
    detail?: React.ReactNode;
  } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Deposit>(`/deposits/${depositId}`);
      setDeposit(unwrap(response));
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Gagal memuat detail deposito."
      );
    } finally {
      setLoading(false);
    }
  }, [depositId]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const product = products.find((p) => p.id === deposit?.product_id);
  const isActive =
    deposit?.status === "PLACED" || deposit?.status === "MATURED";
  const early =
    isActive &&
    deposit !== null &&
    new Date(deposit.maturity_date).getTime() > Date.now();

  const runAction = async () => {
    if (!pending || !deposit) return;
    setSubmitting(true);
    setActionError(null);
    try {
      const response = await request<Deposit>(
        `/deposits/${deposit.id}/${pending}`,
        { method: "POST", idempotencyKey: newIdempotencyKey() }
      );
      const updated = unwrap(response);
      if (pending === "withdraw") {
        setFeedback({
          title: "Deposito dicairkan",
          reference: updated.account_number,
          detail: (
            <DefinitionList
              columns={2}
              items={[
                {
                  label: "Hasil Pencairan",
                  value: <MoneyText value={updated.maturity_proceeds} />,
                },
                {
                  label: "Denda",
                  value: <MoneyText value={updated.early_withdrawal_penalty} />,
                },
                {
                  label: "Pajak Dibayar",
                  value: <MoneyText value={updated.paid_tax} />,
                },
                { label: "Status", value: updated.status },
              ]}
            />
          ),
        });
      } else {
        setFeedback({
          title: "Akrual imbal hasil diposting",
          reference: updated.account_number,
          detail: (
            <DefinitionList
              columns={2}
              items={[
                {
                  label: "Akrual Imbal Hasil",
                  value: <MoneyText value={updated.accrued_profit} />,
                },
                {
                  label: "Akrual Pajak",
                  value: <MoneyText value={updated.accrued_tax} />,
                },
              ]}
            />
          ),
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

  if (loading) {
    return (
      <Card className="mt-4">
        <CardContent className="text-body text-ink-600">
          Memuat detail deposito...
        </CardContent>
      </Card>
    );
  }

  if (error || !deposit) {
    return (
      <div className="mt-4">
        <ErrorState
          title="Gagal memuat detail deposito"
          description={error ?? "Deposito tidak ditemukan."}
          action={
            <Button variant="secondary" onClick={onClose}>
              Tutup
            </Button>
          }
        />
      </div>
    );
  }

  const bagiHasil = deposit.profit_type === "BAGI_HASIL";

  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle>
          Detail Deposito{" "}
          <span className="font-mono">{deposit.account_number}</span>
        </CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          Tutup
        </Button>
      </CardHeader>
      <CardContent>
        {feedback && (
          <div className="rounded-md border border-credit-700/30 bg-credit-50 px-4 py-3">
            <p className="text-title font-medium text-credit-700">
              {feedback.title}
            </p>
            <p className="mt-1 text-body text-ink-900">
              Referensi: <span className="font-mono">{feedback.reference}</span>
            </p>
            {feedback.detail && <div className="mt-2">{feedback.detail}</div>}
          </div>
        )}

        {actionError && (
          <p className="text-body text-debit-700" role="alert">
            {actionError}
          </p>
        )}

        {early && (
          <div className="rounded-md border border-accent-600/30 bg-accent-50 px-4 py-3">
            <p className="text-body text-accent-600">
              Deposito belum jatuh tempo pada{" "}
              <span className="font-mono">{formatDate(deposit.maturity_date)}</span>.
              Pencairan lebih awal akan dikenakan denda
              {product
                ? ` ${rateLabel(product.early_withdrawal_penalty_rate)} dari pokok`
                : ""}
              .
            </p>
          </div>
        )}

        <DefinitionList
          items={[
            {
              label: "Nasabah",
              value: customerLabel(deposit.customer_id, customerNames),
            },
            {
              label: "Produk",
              value: product ? `${product.code} — ${product.name}` : "-",
            },
            {
              label: "Status",
              value: <DepositStatusBadge status={deposit.status} />,
            },
            { label: "Pokok", value: <MoneyText value={deposit.placement_amount} /> },
            {
              label: "Tanggal Mulai",
              value: formatDate(deposit.start_date),
              isMono: true,
            },
            {
              label: "Jatuh Tempo",
              value: formatDate(deposit.maturity_date),
              isMono: true,
            },
            {
              label: "Skema Imbal Hasil",
              value: profitTypeLabel(deposit.profit_type),
            },
            bagiHasil
              ? {
                  label: "Nisbah Pemilik Dana",
                  value: rateLabel(String(Number(deposit.profit_rate) * 100)),
                  isMono: true,
                }
              : {
                  label: "Bunga per Tahun",
                  value: rateLabel(deposit.profit_rate),
                  isMono: true,
                },
            ...(bagiHasil
              ? [
                  {
                    label: "Proyeksi Imbal Hasil Tahunan",
                    value: rateLabel(deposit.yield_rate),
                    isMono: true,
                  },
                ]
              : []),
            {
              label: "Akrual Imbal Hasil",
              value: <MoneyText value={deposit.accrued_profit} />,
            },
            {
              label: "Akrual Pajak (PPh)",
              value: <MoneyText value={deposit.accrued_tax} />,
            },
            {
              label: "Tarif Pajak",
              value: rateLabel(deposit.tax_rate),
              isMono: true,
            },
            ...(deposit.status === "CLOSED" || deposit.status === "BROKEN"
              ? [
                  {
                    label: "Hasil Pencairan",
                    value: <MoneyText value={deposit.maturity_proceeds} />,
                  },
                  {
                    label: "Denda Pencairan",
                    value: (
                      <MoneyText value={deposit.early_withdrawal_penalty} />
                    ),
                  },
                ]
              : []),
            { label: "ARO", value: deposit.aro ? aroLabel(deposit.aro_instruction) : "Tidak" },
          ]}
        />

        <div className="flex gap-2">
          {isActive && (
            <>
              <Button size="sm" onClick={() => setPending("accrue")}>
                Akrual Imbal Hasil
              </Button>
              <Button
                size="sm"
                variant={early ? "danger" : "primary"}
                onClick={() => setPending("withdraw")}
              >
                Cairkan
              </Button>
            </>
          )}
        </div>
      </CardContent>

      <ConfirmDialog
        open={pending !== null}
        title={pending === "withdraw" ? "Cairkan Deposito" : "Akrual Imbal Hasil"}
        destructive={pending === "withdraw" && early}
        loading={submitting}
        confirmLabel={pending === "withdraw" ? "Cairkan Deposito" : "Jalankan Akrual"}
        onCancel={() => setPending(null)}
        onConfirm={runAction}
        description={
          pending === "withdraw" ? (
            <>
              <p>
                Deposito <span className="font-mono">{deposit.account_number}</span>{" "}
                akan ditutup dan dana pokok beserta imbal hasil bersih dibayarkan.
              </p>
              {early && (
                <p className="text-debit-700">
                  Pencairan sebelum jatuh tempo dikenakan denda sesuai tarif
                  produk. Hasil akhir dihitung backend dan ditampilkan setelah
                  pencairan.
                </p>
              )}
            </>
          ) : (
            <p>
              Akrual imbal hasil satu hari berjalan akan diposting untuk deposito{" "}
              <span className="font-mono">{deposit.account_number}</span>.
            </p>
          )
        }
      />
    </Card>
  );
}

export default function DepositoPage() {
  const [deposits, setDeposits] = useState<Deposit[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [listReloadKey, setListReloadKey] = useState(0);

  const [products, setProducts] = useState<BankingProduct[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const customerNames = useMemo(
    () => Object.fromEntries(customers.map((c) => [c.id, c.full_name])),
    [customers]
  );

  const [formOpen, setFormOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{ title: string; reference: string } | null>(
    null
  );

  const [aroOpen, setAroOpen] = useState(false);
  const [aroSubmitting, setAroSubmitting] = useState(false);
  const [aroError, setAroError] = useState<string | null>(null);

  const loadDeposits = useCallback(async (targetPage: number) => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Deposit[]>(
        `/deposits?page=${targetPage}&page_size=${PAGE_SIZE}`
      );
      setDeposits(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Gagal memuat daftar deposito."
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadDeposits(page);
  }, [page, loadDeposits, listReloadKey]);

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

  const depositProducts = useMemo(
    () =>
      products.filter((p) => p.family === "TIME_DEPOSIT" && p.is_active),
    [products]
  );

  const runAro = async () => {
    setAroSubmitting(true);
    setAroError(null);
    try {
      const response = await request<{ processed: number }>("/deposits/run-aro", {
        method: "POST",
        idempotencyKey: newIdempotencyKey(),
      });
      const processed = response.data?.processed ?? 0;
      setFeedback({
        title: "ARO dijalankan",
        reference: `${processed} kontrak diperpanjang`,
      });
      setAroOpen(false);
      setListReloadKey((key) => key + 1);
    } catch (err) {
      setAroError(
        err instanceof ApiError ? err.message : "Proses ARO gagal dijalankan."
      );
      setAroOpen(false);
    } finally {
      setAroSubmitting(false);
    }
  };

  const columns: Column<Deposit>[] = [
    { header: "Nomor Rekening", accessorKey: "account_number", isMono: true },
    {
      header: "Nasabah",
      cell: (row) => customerLabel(row.customer_id, customerNames),
    },
    {
      header: "Pokok",
      type: "money",
      cell: (row) => <MoneyText value={row.placement_amount} />,
    },
    {
      header: "Skema",
      cell: (row) => profitTypeLabel(row.profit_type),
    },
    {
      header: "Jatuh Tempo",
      cell: (row) => formatDate(row.maturity_date),
      isMono: true,
    },
    {
      header: "Akrual Imbal Hasil",
      type: "money",
      cell: (row) => <MoneyText value={row.accrued_profit} />,
    },
    {
      header: "Status",
      cell: (row) => <DepositStatusBadge status={row.status} />,
    },
    {
      header: "Aksi",
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
      <PageHeader
        title="Deposito"
        description="Deposito berjangka: penempatan, akrual imbal hasil, ARO, dan pencairan."
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

      {aroError && (
        <div
          className="mb-4 rounded-md border border-debit-700/30 bg-debit-50 px-4 py-3"
          role="alert"
        >
          <p className="text-body text-debit-700">{aroError}</p>
        </div>
      )}

      {formOpen && (
        <DepositPlaceForm
          products={depositProducts}
          customers={customers}
          onPlaced={(deposit) => {
            setFormOpen(false);
            setSelectedId(deposit.id);
            setPage(1);
            setListReloadKey((key) => key + 1);
          }}
          onClose={() => setFormOpen(false)}
        />
      )}

      {error && !loading ? (
        <ErrorState title="Gagal memuat deposito" description={error} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Daftar Deposito</CardTitle>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="secondary"
                onClick={() => setAroOpen(true)}
              >
                Jalankan ARO
              </Button>
              <Button size="sm" onClick={() => setFormOpen((open) => !open)}>
                Tempatkan Deposito
              </Button>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={deposits}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage="Belum ada deposito."
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
        <DepositDetailPanel
          depositId={selectedId}
          products={products}
          customerNames={customerNames}
          onClose={() => setSelectedId(null)}
          onChanged={() => setListReloadKey((key) => key + 1)}
        />
      )}

      <ConfirmDialog
        open={aroOpen}
        title="Jalankan ARO"
        loading={aroSubmitting}
        confirmLabel="Jalankan ARO"
        onCancel={() => setAroOpen(false)}
        onConfirm={runAro}
        description={
          <p>
            Sistem akan memperpanjang semua deposito ber-ARO yang sudah jatuh
            tempo sesuai instruksinya. Kapitalisasi imbal hasil ikut diposting.
          </p>
        }
      />
    </>
  );
}
