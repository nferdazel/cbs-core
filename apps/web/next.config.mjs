/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  transpilePackages: ["@cbs/shared-types"],
};

export default nextConfig;
