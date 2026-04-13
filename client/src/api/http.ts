export type ApiFetchInit = RequestInit & { token?: string };

export function apiFetch(path: string, init?: ApiFetchInit): Promise<Response> {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  const url = `/api${normalized}`;
  const headers = new Headers(init?.headers);
  if (init?.token) {
    headers.set("Authorization", init.token);
  }
  const { token, ...rest } = init ?? {};
  return fetch(url, { ...rest, headers });
}
