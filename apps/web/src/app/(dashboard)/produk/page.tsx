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
import { useTranslation } from "@/i18n/context";
import { formatRate } from "@/lib/format";

function ratioLabel(value: string): string {
  const num = Number(value);
  if (!Number.isFinite(num)) return value || "-";
  return formatRate(String(num * 100));
}

export default function ProdukPage() {
  const { user } = useAuth();
  const { t } = useTranslation();
  const [products, setProducts] = useState<BankingProduct[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const [book, setBook] = useState("");
  const [family, setFamily] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const bookLabel = (value: COABook): string =>
    value === "SYARIAH" ? t.products.bookSyariah : t.products.bookConventional;

  const familyLabel = (value: ProductFamily): string => {
    switch (value) {
      case "SAVINGS":
        return t.products.familySavings;
      case "TIME_DEPOSIT":
        return t.products.familyTimeDeposit;
      case "LOAN":
        return t.products.familyLoan;
      case "CURRENT_ACCOUNT":
        return t.products.familyCurrent;
      default:
        return value;
    }
  };

  const profitSchemeLabel = (value: ProfitScheme): string => {
    switch (value) {
      case "INTEREST":
        return t.products.profitInterest;
      case "MURABAHAH":
        return t.products.profitMurabahah;
      case "MUDHARABAH":
        return t.products.profitMudharabah;
      case "MUSYARAKAH":
        return t.products.profitMusyarakah;
      case "IJARAH":
        return t.products.profitIjarah;
      case "WADIAH":
        return t.products.profitWadiah;
      default:
        return value;
    }
  };

  const bookOptions = useMemo(() => {
    const options = [
      { value: "", label: t.products.bookAll },
      { value: "CONVENTIONAL", label: t.products.bookConventional },
      { value: "SYARIAH", label: t.products.bookSyariah },
    ];
    // Pemilih buku hanya menawarkan lini usaha yang aktif di instalasi. Cakupan
    // dibaca dari server (/auth/me -> active_books); web tidak menebak.
    const active = user?.active_books;
    if (!active) return options;
    return options.filter(
      (option) => option.value === "" || active.includes(option.value),
    );
  }, [user?.active_books, t]);

  const familyOptions = useMemo(
    () => [
      { value: "", label: t.products.familyAll },
      { value: "SAVINGS", label: t.products.familySavings },
      { value: "TIME_DEPOSIT", label: t.products.familyTimeDeposit },
      { value: "LOAN", label: t.products.familyLoan },
      { value: "CURRENT_ACCOUNT", label: t.products.familyCurrent },
    ],
    [t],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<BankingProduct[]>("/products");
      setProducts(response.data ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t.products.loadError);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const filtered = useMemo(
    () =>
      products.filter(
        (product) =>
          (book === "" || product.book === book) &&
          (family === "" || product.family === family),
      ),
    [products, book, family],
  );

  const columns: Column<BankingProduct>[] = [
    { header: t.products.labelCode, accessorKey: "code", isMono: true },
    { header: t.products.labelName, accessorKey: "name" },
    {
      header: t.products.labelFamily,
      cell: (row) => familyLabel(row.family),
    },
    {
      header: t.products.labelBook,
      cell: (row) => bookLabel(row.book),
    },
    {
      header: t.products.labelProfitScheme,
      cell: (row) => profitSchemeLabel(row.profit_scheme),
    },
    {
      header: t.common.status,
      cell: (row) =>
        row.is_active ? (
          <Badge variant="credit">{t.products.active}</Badge>
        ) : (
          <Badge variant="outline">{t.products.inactive}</Badge>
        ),
    },
    {
      header: t.common.actions,
      cell: (row) => (
        <Button
          size="sm"
          variant="secondary"
          onClick={() => setSelectedId(row.id)}
        >
          {t.products.detailButton}
        </Button>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title={t.products.title}
        description={t.products.description}
      />

      <Card className="mb-4">
        <CardContent className="flex flex-wrap items-end gap-4">
          <div className="w-56">
            <Select
              label={t.products.labelBook}
              value={book}
              onChange={(e) => setBook(e.target.value)}
              options={bookOptions}
            />
          </div>
          <div className="w-56">
            <Select
              label={t.products.labelFamily}
              value={family}
              onChange={(e) => setFamily(e.target.value)}
              options={familyOptions}
            />
          </div>
          <Button
            variant="secondary"
            onClick={() => {
              setBook("");
              setFamily("");
            }}
          >
            {t.products.filterReset}
          </Button>
        </CardContent>
      </Card>

      {error && !loading ? (
        <ErrorState
          title={t.products.errorTitle}
          description={error}
          action={
            <Button
              variant="secondary"
              onClick={() => setReloadKey((key) => key + 1)}
            >
              {t.common.retry}
            </Button>
          }
        />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>{t.products.listTitle}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={filtered}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={t.products.empty}
              zebra
            />
          </CardContent>
        </Card>
      )}

      {selectedId && (
        <ProductDetail
          productId={selectedId}
          onClose={() => setSelectedId(null)}
        />
      )}
    </>
  );
}

interface ProductDetailProps {
  productId: string;
  onClose: () => void;
}

function ProductDetail({ productId, onClose }: ProductDetailProps) {
  const { t } = useTranslation();
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
        err instanceof ApiError ? err.message : t.products.detailLoadError,
      );
    } finally {
      setLoading(false);
    }
  }, [productId, t]);

  useEffect(() => {
    load();
  }, [load]);

  const familyLabel = (value: ProductFamily): string => {
    switch (value) {
      case "SAVINGS":
        return t.products.familySavings;
      case "TIME_DEPOSIT":
        return t.products.familyTimeDeposit;
      case "LOAN":
        return t.products.familyLoan;
      case "CURRENT_ACCOUNT":
        return t.products.familyCurrent;
      default:
        return value;
    }
  };

  const profitSchemeLabel = (value: ProfitScheme): string => {
    switch (value) {
      case "INTEREST":
        return t.products.profitInterest;
      case "MURABAHAH":
        return t.products.profitMurabahah;
      case "MUDHARABAH":
        return t.products.profitMudharabah;
      case "MUSYARAKAH":
        return t.products.profitMusyarakah;
      case "IJARAH":
        return t.products.profitIjarah;
      case "WADIAH":
        return t.products.profitWadiah;
      default:
        return value;
    }
  };

  const scheduleLabel = (value: ScheduleMethod): string => {
    switch (value) {
      case "FLAT":
        return t.products.scheduleFlat;
      case "ANNUITY":
        return t.products.scheduleAnnuity;
      case "SLIDING":
        return t.products.scheduleSliding;
      case "BAGI_HASIL":
        return t.products.scheduleBagiHasil;
      case "NONE":
        return t.products.scheduleNone;
      default:
        return value;
    }
  };

  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle>{t.products.detailTitle}</CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          {t.common.close}
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.products.loadingDetail} />
        ) : error || !product ? (
          <ErrorState
            title={t.products.detailErrorTitle}
            description={error ?? t.products.notFound}
            action={
              <Button variant="secondary" onClick={onClose}>
                {t.common.close}
              </Button>
            }
          />
        ) : (
          <DefinitionList
            columns={2}
            items={[
              {
                label: t.products.labelCode,
                value: product.code,
                isMono: true,
              },
              { label: t.products.labelName, value: product.name },
              {
                label: t.products.labelFamily,
                value: familyLabel(product.family),
              },
              {
                label: t.products.labelBook,
                value:
                  product.book === "SYARIAH"
                    ? t.products.bookSyariah
                    : t.products.bookConventional,
              },
              {
                label: t.products.labelProfitScheme,
                value: profitSchemeLabel(product.profit_scheme),
              },
              {
                label: t.products.labelScheduleMethod,
                value: scheduleLabel(product.schedule_method),
              },
              {
                label: t.products.labelRateAnnual,
                value: formatRate(product.rate_annual),
                isMono: true,
              },
              {
                label: t.products.labelProfitSharingRatio,
                value: ratioLabel(product.profit_sharing_ratio),
                isMono: true,
              },
              {
                label: t.products.labelMinAmount,
                value: <MoneyText value={product.min_amount} />,
              },
              {
                label: t.products.labelMaxAmount,
                value: <MoneyText value={product.max_amount} />,
              },
              {
                label: t.products.labelMinTerm,
                value: `${product.min_term_months} ${t.products.termSuffix}`,
                isMono: true,
              },
              {
                label: t.products.labelMaxTerm,
                value: `${product.max_term_months} ${t.products.termSuffix}`,
                isMono: true,
              },
              {
                label: t.products.labelAdminFee,
                value: <MoneyText value={product.admin_fee} />,
              },
              {
                label: t.products.labelEarlyWithdrawalPenalty,
                value: formatRate(product.early_withdrawal_penalty_rate),
                isMono: true,
              },
              {
                label: t.products.labelTaxRate,
                value: formatRate(product.tax_rate),
                isMono: true,
              },
              {
                label: t.products.labelPartialPayment,
                value: product.allow_partial_payment
                  ? t.products.yes
                  : t.products.no,
              },
              {
                label: t.common.status,
                value: product.is_active ? (
                  <Badge variant="credit">{t.products.active}</Badge>
                ) : (
                  <Badge variant="outline">{t.products.inactive}</Badge>
                ),
              },
            ]}
          />
        )}
      </CardContent>
    </Card>
  );
}
