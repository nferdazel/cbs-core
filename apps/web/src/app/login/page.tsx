"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import { Building2 } from "lucide-react";
import { useTranslation } from "@/i18n/context";
import { useAppInfo } from "@/components/AppInfoProvider";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { ApiError, request, unwrap } from "@/lib/api";
import { setStoredUser } from "@/lib/auth";
import type { LoginResponse } from "@/lib/types";

export default function LoginPage() {
  const router = useRouter();
  const { language, setLanguage, t } = useTranslation();
  // Nama aplikasi di halaman login berasal dari identitas server; tidak ada sesi
  // pada titik ini, jadi endpoint /app-info memang publik.
  const appInfo = useAppInfo();

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const response = await request<LoginResponse>("/auth/login", {
        method: "POST",
        auth: false,
        body: { username, password },
      });
      // Token sudah ditetapkan backend sebagai httpOnly cookie; simpan hanya
      // profil non-sensitif untuk tampilan.
      setStoredUser(unwrap(response).user);
      router.replace("/");
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : t.login.networkError;
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-canvas p-4">
      <div className="w-full max-w-sm space-y-4">
        <div className="flex justify-end">
          <div
            role="group"
            aria-label={t.common.language}
            className="flex items-center rounded-md border border-border bg-surface p-0.5 text-meta"
          >
            <button
              type="button"
              aria-pressed={language === "id"}
              onClick={() => setLanguage("id")}
              className={`rounded-sm px-2.5 py-1 transition-colors duration-fast ${
                language === "id"
                  ? "bg-navy-700 font-medium text-white"
                  : "text-ink-600 hover:text-ink-900"
              }`}
            >
              ID
            </button>
            <button
              type="button"
              aria-pressed={language === "en"}
              onClick={() => setLanguage("en")}
              className={`rounded-sm px-2.5 py-1 transition-colors duration-fast ${
                language === "en"
                  ? "bg-navy-700 font-medium text-white"
                  : "text-ink-600 hover:text-ink-900"
              }`}
            >
              EN
            </button>
          </div>
        </div>

        <Card>
          <CardHeader className="flex flex-col items-center justify-center py-6 text-center">
            <div className="mb-2 inline-flex h-10 w-10 items-center justify-center rounded-md bg-navy-700 text-white">
              <Building2 className="h-5 w-5" aria-hidden />
            </div>
            <CardTitle>{appInfo.display_name}</CardTitle>
            <CardDescription>{appInfo.description}</CardDescription>
          </CardHeader>

          <CardContent>
            {error && <Alert variant="error">{error}</Alert>}

            <form onSubmit={handleLogin} className="space-y-4">
              <Input
                label={t.login.staffUsername}
                type="text"
                required
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />

              <Input
                label={t.login.password}
                type="password"
                required
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />

              <Button type="submit" loading={loading} className="w-full">
                {loading ? t.login.processing : t.login.submitBtn}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
