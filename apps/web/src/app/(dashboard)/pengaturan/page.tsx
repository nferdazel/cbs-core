"use client";

import { useAuth } from "@/lib/useAuth";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { EmptyState } from "@/components/ui/States";

export default function PengaturanPage() {
  const { user } = useAuth();

  return (
    <>
      <PageHeader
        title="Pengaturan"
        description="Profil pengguna dan konfigurasi sistem."
      />
      <Card className="mb-4">
        <CardHeader>
          <CardTitle>Profil Pengguna</CardTitle>
        </CardHeader>
        <CardContent>
          <DefinitionList
            items={[
              { label: "Nama", value: user?.full_name || user?.username || "-" },
              { label: "Role", value: user?.role || "-" },
              { label: "Cabang", value: user?.branch_code || "-", isMono: true },
            ]}
          />
        </CardContent>
      </Card>
      <EmptyState
        title="Konfigurasi sistem belum tersedia"
        description="Belum ada endpoint konfigurasi (parameter, limit, kalender) yang siap untuk ditampilkan."
      />
    </>
  );
}
