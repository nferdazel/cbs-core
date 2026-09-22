"use client";

import { useAuth } from "@/lib/useAuth";
import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { DefinitionList } from "@/components/ui/DefinitionList";

export default function BerandaPage() {
  const { user } = useAuth();
  const { t } = useTranslation();

  return (
    <>
      <PageHeader title={t.home.title} description={t.home.description} />
      <Card>
        <CardHeader>
          <CardTitle>{t.home.sessionTitle}</CardTitle>
        </CardHeader>
        <CardContent>
          <DefinitionList
            items={[
              {
                label: t.common.name,
                value: user?.full_name || user?.username || "-",
              },
              {
                label: t.common.username,
                value: user?.username || "-",
                isMono: true,
              },
              { label: t.common.role, value: user?.role || "-" },
              {
                label: t.common.branch,
                value: user?.branch_code || "-",
                isMono: true,
              },
            ]}
          />
        </CardContent>
      </Card>
    </>
  );
}
