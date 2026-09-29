"use client";

import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { PrintButton } from "@/components/ui/PrintButton";

/**
 * Kartu Form 00.19 "Struktur Organisasi BPR". Form ini BUKAN tabel angka yang ikut
 * bundel, melainkan berkas PDF yang wajib disampaikan tersendiri; API menyediakan
 * dokumen cetak HTML (dari bank_offices + bank_management) dan bank menyimpannya
 * sebagai PDF dari dialog cetak browser. Karena itu kartu ini hanya menyediakan
 * tombol cetak, tidak ada tabel untuk diisi.
 */
export function OJKStrukturOrganisasiCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canExport = hasPermission(user, "reports:export");

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkData.strukturOrganisasi.title}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-body text-ink-600">
          {t.ojkData.strukturOrganisasi.description}
        </p>
        {!canExport && (
          <Alert variant="info">{t.ojkData.strukturOrganisasi.forbidden}</Alert>
        )}
        <div className="mt-4">
          <PrintButton
            url="/reports/ojk/struktur-organisasi"
            label={t.ojkData.strukturOrganisasi.print}
          />
        </div>
      </CardContent>
    </Card>
  );
}
