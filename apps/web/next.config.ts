import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  transpilePackages: ["@hocuspocus/provider", "y-codemirror.next"],
};

export default nextConfig;
