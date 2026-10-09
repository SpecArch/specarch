import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // Carbon's own Sass uses functions Sass has deprecated; its warnings are
  // not this project's to act on.
  sassOptions: { quietDeps: true },
};

export default nextConfig;
