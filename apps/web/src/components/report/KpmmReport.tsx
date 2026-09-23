"use client";

import { formatDate, formatRate } from "@/lib/format";
import type { KPMMATMRBaris, KPMMKomponen, KPMMReport } from "@/lib/types";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { CardContent } from "@/components/ui/Card";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { MoneyText } from "@/components/ui/MoneyText";

/**
 * Satu komponen angka KPMM. `tersedia` false ditampilkan sebagai "belum tersedia"
 * beserta alasan server, BUKAN nol: bagi regulator nol dan tidak tersedia berbeda.
 */
function KomponenNilai({
  komponen,
  persen = false,
}: {
  komponen?: KPMMKomponen;
  persen?: boolean;
}) {
  const { t } = useTranslation();
  if (!komponen || !komponen.tersedia) {
    return (
      <span className="text-ink-600">
        {t.reports.kpmmNotAvailable}
        {komponen?.alasan ? ` (${komponen.alasan})` : ""}
      </span>
    );
  }
  return persen ? (
    <span className="font-mono">{formatRate(komponen.nilai)}</span>
  ) : (
    <MoneyText value={komponen.nilai} />
  );
}

/**
 * Tampilan laporan KPMM/ATMR. Bila `lengkap` false, peringatan beserta daftar
 * alasan ditampilkan di atas angka; angkanya tetap ditampilkan apa adanya dan
 * TIDAK diubah, hanya ditandai belum final. Nilai harga di server; UI tidak
 * menghitung ulang rasio.
 */
export function KpmmReport({ report }: { report: KPMMReport }) {
  const { t } = useTranslation();

  const deductionBasis =
    report.deduction_basis === "per_kredit"
      ? t.reports.kpmmDeductionPerKredit
      : report.deduction_basis === "agregat"
        ? t.reports.kpmmDeductionAgregat
        : report.deduction_basis || "-";

  const atmrColumns: Column<KPMMATMRBaris>[] = [
    { header: t.reports.kpmmColKategori, accessorKey: "kategori" },
    {
      header: t.reports.kpmmColDasar,
      type: "money",
      cell: (row) => <MoneyText value={row.dasar} />,
    },
    {
      header: t.reports.kpmmColBobot,
      align: "right",
      cell: (row) => (
        <span className="font-mono">{formatRate(Number(row.bobot) * 100)}</span>
      ),
    },
    {
      header: t.reports.kpmmColNilai,
      type: "money",
      cell: (row) => <MoneyText value={row.nilai} />,
    },
  ];

  return (
    <>
      {!report.lengkap && (
        <div className="px-4 pt-4">
          <Alert variant="warning" title={t.reports.kpmmIncompleteTitle}>
            <p>{t.reports.kpmmIncompleteDesc}</p>
            {report.alasan_tidak_lengkap &&
              report.alasan_tidak_lengkap.length > 0 && (
                <ul className="mt-2 list-disc space-y-1 pl-5">
                  {report.alasan_tidak_lengkap.map((alasan, index) => (
                    <li key={index}>{alasan}</li>
                  ))}
                </ul>
              )}
          </Alert>
        </div>
      )}

      <CardContent>
        <DefinitionList
          items={[
            {
              label: t.reports.kpmmAsOf,
              value: formatDate(report.as_of),
              isMono: true,
            },
            {
              label: t.reports.kpmmBook,
              value: report.book || t.reports.kpmmBookConsolidated,
            },
            {
              label: t.reports.kpmmAtmr,
              value: <KomponenNilai komponen={report.atmr} />,
            },
            {
              label: t.reports.kpmmModalIntiUtama,
              value: <KomponenNilai komponen={report.modal_inti_utama} />,
            },
            {
              label: t.reports.kpmmPengurangModalInti,
              value: <KomponenNilai komponen={report.pengurang_modal_inti} />,
            },
            {
              label: t.reports.kpmmModalInti,
              value: <KomponenNilai komponen={report.modal_inti} />,
            },
            {
              label: t.reports.kpmmModalPelengkap,
              value: <KomponenNilai komponen={report.modal_pelengkap} />,
            },
            {
              label: t.reports.kpmmTotalModal,
              value: <KomponenNilai komponen={report.total_modal} />,
            },
            {
              label: t.reports.kpmmRasioKpmm,
              value: <KomponenNilai komponen={report.rasio_kpmm} persen />,
            },
            {
              label: t.reports.kpmmRasioModalInti,
              value: (
                <KomponenNilai komponen={report.rasio_modal_inti} persen />
              ),
            },
            {
              label: t.reports.kpmmDeductionBasis,
              value: deductionBasis,
            },
            ...(report.ppap_business_date
              ? [
                  {
                    label: t.reports.kpmmPpapDate,
                    value: formatDate(report.ppap_business_date),
                    isMono: true,
                  },
                ]
              : []),
          ]}
        />
      </CardContent>

      <div className="border-t border-border p-4">
        <p className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
          {t.reports.kpmmThresholdsTitle}
        </p>
        <DefinitionList
          items={[
            {
              label: t.reports.kpmmMinFrac,
              value: formatRate(Number(report.kpmm_min_frac) * 100),
              isMono: true,
            },
            {
              label: t.reports.kpmmModalIntiMinFrac,
              value: formatRate(Number(report.modal_inti_min_frac) * 100),
              isMono: true,
            },
            {
              label: t.reports.kpmmModalIntiMinAmount,
              value: <MoneyText value={report.modal_inti_min_amount} />,
            },
          ]}
        />
      </div>

      {report.atmr_baris && report.atmr_baris.length > 0 && (
        <div className="border-t border-border p-4">
          <p className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
            {t.reports.kpmmAtmrTitle}
          </p>
          <DataTable
            columns={atmrColumns}
            data={report.atmr_baris}
            keyExtractor={(row) => row.kategori}
            emptyMessage={t.reports.kpmmAtmrEmpty}
            zebra
          />
        </div>
      )}

      {report.catatan && report.catatan.length > 0 && (
        <div className="border-t border-border p-4">
          <p className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
            {t.reports.kpmmCatatanTitle}
          </p>
          <ul className="list-disc space-y-1 pl-5 text-body text-ink-900">
            {report.catatan.map((catatan, index) => (
              <li key={index}>{catatan}</li>
            ))}
          </ul>
        </div>
      )}
    </>
  );
}
