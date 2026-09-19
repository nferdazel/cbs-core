/**
 * Alamat API internal untuk rewrite. Dipakai hanya di sisi server Next.js dan tidak
 * pernah sampai ke browser: browser selalu memanggil jalur same-origin `/api/*`,
 * sehingga cookie sesi httpOnly dan token CSRF tetap bekerja tanpa CORS.
 */
const INTERNAL_API_URL = process.env.INTERNAL_API_URL || "http://cbs-api:8080";

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  transpilePackages: ["@cbs/shared-types"],
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${INTERNAL_API_URL}/api/:path*` },
    ];
  },
};

export default nextConfig;
