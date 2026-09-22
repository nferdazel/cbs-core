import "./globals.css";
import type { Metadata } from "next";
import { IBM_Plex_Sans, IBM_Plex_Mono } from "next/font/google";
import { AppInfoProvider } from "@/components/AppInfoProvider";
import { LanguageProvider } from "@/i18n/context";
import { fetchAppInfo } from "@/lib/app-info-server";

const ibmPlexSans = IBM_Plex_Sans({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-ibm-sans",
  display: "swap",
});

const ibmPlexMono = IBM_Plex_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-ibm-mono",
  display: "swap",
});

// Judul & deskripsi tab mengikuti identitas dari server (endpoint /api/v1/app-info),
// bukan literal, supaya perubahan nama aplikasi berlaku tanpa build ulang.
export async function generateMetadata(): Promise<Metadata> {
  const info = await fetchAppInfo();
  return {
    title: info.display_name,
    description: info.description,
  };
}

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const info = await fetchAppInfo();
  return (
    <html
      lang="id"
      className={`${ibmPlexSans.variable} ${ibmPlexMono.variable}`}
    >
      <body className="min-h-screen bg-canvas font-sans text-ink-900 antialiased">
        <AppInfoProvider initial={info}>
          <LanguageProvider>{children}</LanguageProvider>
        </AppInfoProvider>
      </body>
    </html>
  );
}
