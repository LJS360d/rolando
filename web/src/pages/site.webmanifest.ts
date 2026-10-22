import type { APIRoute } from 'astro';

export const GET: APIRoute = async () => {
  const manifest = {
    id: '/',
    name: "Rolando",
    short_name: "Rolando",
    description: "Rolando",
    lang: "en",
    dir: 'ltr',
    start_url: "/",
    scope: '/',
    display: 'browser',
    theme_color: "#ffffff",
    background_color: "#ffffff",
    icons: [
      { src: '/android-chrome-192x192.png', sizes: '192x192', type: 'image/png' },
      { src: '/android-chrome-512x512.png', sizes: '512x512', type: 'image/png' },
      {
        src: '/android-chrome-512x512.png',
        sizes: '512x512',
        type: 'image/png',
        purpose: 'maskable',
      },
    ],
  };

  return new Response(JSON.stringify(manifest, null, 2), {
    headers: { 'Content-Type': 'application/manifest+json; charset=utf-8' },
  });
};
