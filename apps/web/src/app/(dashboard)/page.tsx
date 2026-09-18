"use client";

import { useAuth } from "@/lib/useAuth";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { DefinitionList } from "@/components/ui/DefinitionList";

export default function BerandaPage() {
  const { user } = useAuth();

  return (
    <>
      <PageHeader
        title="Beranda"
        description="Ringkasan sesi kerja. Gunakan sidebar untuk berpindah modul."
      />
      <Card>
        <CardHeader>
          <CardTitle>Sesi Pengguna</CardTitle>
        </CardHeader>
        <CardContent>
          <DefinitionList
            items={[
              { label: "Nama", value: user?.full_name || user?.username || "-" },
              { label: "Username", value: user?.username || "-", isMono: true },
              { label: "Role", value: user?.role || "-" },
              { label: "Cabang", value: user?.branch_code || "-", isMono: true },
            ]}
          />
        </CardContent>
      </Card>
    </>
  );
}
