import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
      return [
        { source: '/api/:path*', destination: 'http://localhost:8080/v1/:path*' }
      ]
  },
};

export default nextConfig;
