import type { APIRoute } from 'astro';

/**
 * RFC 9116. A route, not a static file, so `Expires` rolls forward on every
 * build (the RFC wants it under a year out) and the policy/canonical URLs use
 * the deploy's own origin.
 */
const CONTACT = 'luca.lencioni09@gmail.com';

export const GET: APIRoute = ({ site }) => {
  const expires = new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toISOString();

  const body = `Contact: mailto:${CONTACT}
Expires: ${expires}
Preferred-Languages: en, it
Canonical: ${new URL('.well-known/security.txt', site).href}
Policy: ${new URL('privacy-policy', site).href}
`;

  return new Response(body, {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
};
