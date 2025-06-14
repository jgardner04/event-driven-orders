/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  async rewrites() {
    return [
      {
        source: '/api/proxy/:path*',
        destination: 'http://localhost:8080/:path*',
      },
      {
        source: '/api/orders/:path*',
        destination: 'http://localhost:8081/:path*',
      },
      {
        source: '/api/sap/:path*',
        destination: 'http://localhost:8082/:path*',
      },
    ];
  },
  webpack: (config) => {
    config.experiments = {
      ...config.experiments,
      topLevelAwait: true,
    };
    return config;
  },
};

module.exports = nextConfig;