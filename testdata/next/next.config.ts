import type { NextConfig } from "next"

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      { source: "/api/:path*", destination: "https://api.example.com/:path*" },
    ]
  },
}

export default nextConfig
