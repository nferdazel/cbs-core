"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  BankingProduct,
  COABook,
  ProductFamily,
  ProfitScheme,
  ScheduleMethod,
} from "@/lib/operations-types";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Select } from "@/components/ui/Select";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { useAuth } from "@/lib/useAuth";
import { formatRate } from "@/lib/format";

const BOOK_LABEL: Record<COABook, string> = {
  CONVENTIONAL: "Konvensional",
  SYARIAH: "Syariah",
};

const FAMILY_LABEL: Record<ProductFamily, string> = {
  SAVINGS: "Tabungan",
  TIME_DEPOSIT: "Deposito Berjangka",
  LOAN: "Kredit",
  CURRENT_ACCOUNT: "Giro",
};

const PROFIT_SCHEME_LABEL: Record<ProfitScheme, string> = {
  INTEREST: "Bunga",
  MURABAHAH: "Murabahah",
  MUDHARABAH: "Mudharabah",
  MUSYARAKAH: "Musyarakah",
  IJARAH: "Ijarah",
  WADIAH: "Wadiah",
};

const SCHEDULE_LABEL: Record<ScheduleMethod, string> = {
  FLAT: "Flat",
  ANNUITY: "Anuitas",
  SLIDING: "Sliding",
  BAGI_HASIL: "Bagi Hasil",
  NONE: "Tanpa Jadwal",
};

const BOOK_OPTIONS = [
  { value: "", label: "Semua buku" },
  { value: "CONVENTIONAL", label: "Konvensional" },
  { value: "SYARIAH", label: "Syariah" },
];

const FAMILY_OPTIONS = [
  { value: "", label: "Semua family" },
  { value: "SAVINGS", label: "Tabungan" },
  { value: "TIME_DEPOSIT", label: "Deposito Berjangka" },
  { value: "LOAN", label: "Kredit" },
  { value: "CURRENT_ACCOUNT", label: "Giro" },
];

function ratioLabel(value: string): string {
  const num = Number(value);
  if (!Number.isFinite(num)) return value || "-";
  return formatRate(String(num * 100));
}

function yesNo(value: boolean): string {
  return value ? "Ya" : "Tidak";
}

interface ProductDetailProps {
  productId: string;
  onClose: () => void;
}

function ProductDetail({ productId, onClose }: ProductDetailProps) {
  const [product, setProduct] = useState<BankingProduct | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<BankingProduct>(`/products/${productId}`);
      setProduct(response.data ?? null);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Gagal memuat detail produk."
      );
    } finally {
      setLoading(false);
    }
  }, [productId]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle>Detail Produk</CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          Tutup
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label="Memuat detail produk..." />
        ) : error || !product ? (
          <ErrorState
            title="Gagal memuat detail produk"
            description={error ?? "Produk tidak ditemukan."}
            action={
              <Button variant="secondary" onClick={onClose}>
                Tutup
              </Button>
            }
          />
        ) : (
          <DefinitionList
            columns={2}
            items={[
              { label: "Kode", value: product.code, isMono: true },
              { label: "Nama", value: product.name },
              { label: "Family", value: FAMILY_LABEL[product.family] ?? product.family },
              { label: "Buku", value: BOOK_LABEL[product.book] ?? product.book },
              {
                label: "Skema Imbal Hasil",
                value: PROFIT_SCHEME_LABEL[product.profit_scheme] ?? product.profit_scheme,
              },
              {
                label: "Metode Jadwal",
                value: SCHEDULE_LABEL[product.schedule_method] ?? product.schedule_method,
              },
              { label: "Tarif Tahunan", value: formatRate(product.rate_annual), isMono: true },
              {
                label: "Nisbah Bagi Hasil",
                value: ratioLabel(product.profit_sharing_ratio),
                isMono: true,
              },
              {
                label: "Minimal Nominal",
                value: <MoneyText value={product.min_amount} />,
              },
              {
                label: "Maksimal Nominal",
                value: <MoneyText value={product.max_amount} />,
              },
              {
                label: "Tenor Minimum",
                value: `${product.min_term_months} bulan`,
                isMono: true,
              },
              {
                label: "Tenor Maksimum",
                value: `${product.max_term_months} bulan`,
                isMono: true,
              },
              { label: "Biaya Admin", value: <MoneyText value={product.admin_fee} /> },
              {
                label: "Tarif Denda Pencairan Dini",
                value: formatRate(product.early_withdrawal_penalty_rate),
                isMono: true,
              },
              { label: "Tarif Pajak", value: formatRate(product.tax_rate), isMono: true },
              {
                label: "Angsuran Parsial",
                value: yesNo(product.allow_partial_payment),
              },
              {
                label: "Status",
                value: product.is_active ? (
                  <Badge variant="credit">Aktif</Badge>
                ) : (
                  <Badge variant="outline">Nonaktif</Badge>
                ),
              },
            ]}
          />
        )}
      </CardContent>
    </Card>
  );
}

export default function ProdukPage() {
  const { user } = useAuth();
  const [products, setProducts] = useState<BankingProduct[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const [book, setBook] = useState("");
  const [family, setFamily] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  // Pemilih buku hanya menawarkan lini usaha yang aktif di instalasi. Cakupan
  // dibaca dari server (/auth/me -> active_books); web tidak menebak.
  const bookOptions = useMemo(() => {
    const active = user?.active_books;
    if (!active) return BOOK_OPTIONS;
    return BOOK_OPTIONS.filter(
      (option) => option.value === "" || active.includes(option.value)
    );
  }, [user?.active_books]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<BankingProduct[]>("/products");
      setProducts(response.data ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat produk.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const filtered = useMemo(
    () =>
      products.filter(
        (product) =>
          (book === "" || product.book === book) &&
          (family === "" || product.family === family)
      ),
    [products, book, family]
  );

  const columns: Column<BankingProduct>[] = [
    { header: "Kode", accessorKey: "code", isMono: true },
    { header: "Nama", accessorKey: "name" },
    {
      header: "Family",
      cell: (row) => FAMILY_LABEL[row.family] ?? row.family,
    },
    {
      header: "Buku",
      cell: (row) => BOOK_LABEL[row.book] ?? row.book,
    },
    {
      header: "Skema Imbal Hasil",
      cell: (row) => PROFIT_SCHEME_LABEL[row.profit_scheme] ?? row.profit_scheme,
    },
    {
      header: "Status",
      cell: (row) =>
        row.is_active ? (
          <Badge variant="credit">Aktif</Badge>
        ) : (
          <Badge variant="outline">Nonaktif</Badge>
        ),
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
        title="Produk"
        description="Master produk bank: konvensional dan syariah. Halaman ini hanya baca."
      />

      <Card className="mb-4">
        <CardContent className="flex flex-wrap items-end gap-4">
          <div className="w-56">
            <Select
              label="Buku"
              value={book}
              onChange={(e) => setBook(e.target.value)}
              options={bookOptions}
            />
          </div>
          <div className="w-56">
            <Select
              label="Family"
              value={family}
              onChange={(e) => setFamily(e.target.value)}
              options={FAMILY_OPTIONS}
            />
          </div>
          <Button
            variant="secondary"
            onClick={() => {
              setBook("");
              setFamily("");
            }}
          >
            Reset Filter
          </Button>
        </CardContent>
      </Card>

      {error && !loading ? (
        <ErrorState
          title="Gagal memuat produk"
          description={error}
          action={
            <Button variant="secondary" onClick={() => setReloadKey((key) => key + 1)}>
              Coba lagi
            </Button>
          }
        />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Daftar Produk</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={filtered}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage="Tidak ada produk yang sesuai filter."
              zebra
            />
          </CardContent>
        </Card>
      )}

      {selectedId && (
        <ProductDetail productId={selectedId} onClose={() => setSelectedId(null)} />
      )}
    </>
  );
}
