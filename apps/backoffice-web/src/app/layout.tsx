import "./globals.css";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "CBS Portal | Core Banking System",
  description: "Next-generation Core Banking Management & Ledger Portal",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="antialiased min-h-screen bg-slate-950 text-slate-100 flex flex-col">
        {children}
      </body>
    </html>
  );
}
