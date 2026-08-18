import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  turbopack: {
    // MathJax v4 резолвит свои шрифт/химию-модули через subpath imports
    // (#default-font/*, #mhchem/*) из package.json — Node это понимает, а
    // сборщик Turbopack (в отличие от Node) — нет, и next build падает.
    // Алиасим точечно на реальные пути внутри уже установленных пакетов.
    resolveAlias: {
      "#default-font/svg/default.js":
        "@mathjax/mathjax-newcm-font/mjs/svg/default.js",
      "#mhchem/mhchemParser.js": "mhchemparser/esm/mhchemParser.js",
    },
  },
};

export default nextConfig;
