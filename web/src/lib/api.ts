import { signal, type Signal } from "@preact/signals";

export type Session = { u: string; n: string; a?: string; c?: number; o: boolean; g: string[]; e: number };

export type Config = {
  reply_rate: number;
  reaction_rate: number;
  vc_join_rate: number;
  max_size_mb: number;
  n_gram_size: number;
  markov_max_branches: number;
  pings: boolean;
  premium: boolean;
};

export type Analytics = {
  complexity_score: number;
  gifs: number;
  images: number;
  videos: number;
  words: number;
  messages: number;
  bytes: number;
};

export type GuildRow = { id: string; name: string; icon: string; members: number; state: ChainState; config: Config; analytics: Analytics };

export type ChainState = "loaded" | "unloaded" | "fetching" | "empty";

export type Link = { w: string; c: number };
export type Graph = { word: string; in: Link[] | null; out: Link[] | null; in_total: number; out_total: number };

export type Bot = {
  name: string;
  avatar: string;
  invite: string;
  support: string;
  guilds: number;
  commands: { name: string; description: string }[];
};

export type AdminPage = {
  rows: GuildRow[];
  total: number;
  errors: Record<string, string>;
  sort: string;
  page: number;
  pages: number;
  started: number;
  loaded: number;
  batch: BatchStatus | null;
};

export type BatchStatus = { op: string; total: number; ok: number; failed: number; started: number };

export type MessagesPage = { messages: string[]; mentions?: Record<string, string>; page: number; pages: number; first: number; last: number };

export const browser = typeof window !== "undefined";
export const login = () => location.assign("/api/auth/login");

async function request(method: string, path: string, body?: unknown) {
  const res = await fetch(`/api${path}`, {
    method,
    headers: method === "GET" ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 401) login();
  if (res.status === 429) throw new Error("Too many requests, slow down a bit.");
  if (!res.ok) throw new Error((await res.text()) || res.statusText);
  return res;
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await request(method, path, body);
  return res.status === 204 ? (undefined as T) : res.json();
}

// ndjson yields each line of a streamed NDJSON response as it arrives.
export async function* ndjson<T>(method: string, path: string, body?: unknown): AsyncGenerator<T> {
  const reader = (await request(method, path, body)).body!.pipeThrough(new TextDecoderStream()).getReader();
  let buf = "";
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      const lines = (buf + value).split("\n");
      buf = lines.pop()!;
      for (const l of lines) if (l.trim()) yield JSON.parse(l);
    }
    if (buf.trim()) yield JSON.parse(buf);
  } finally {
    reader.cancel().catch(() => {});
  }
}

export type Resource<T> = { data?: T; error?: string };

const cache = new Map<string, Signal<Resource<any>>>();
const latest = new Map<string, number>();

// resource fetches path once per page load and shares the result with every island that asks for it.
export function resource<T>(path: string, refresh = false): Signal<Resource<T>> {
  let s = cache.get(path);
  if (!s || refresh) {
    s ??= signal({});
    cache.set(path, s);
    const target = s;
    const n = (latest.get(path) ?? 0) + 1;
    latest.set(path, n);
    // a refresh that fails keeps the data it was refreshing; an older response never overwrites a newer one
    if (browser)
      api<T>("GET", path).then(
        (data) => latest.get(path) === n && (target.value = { data }),
        (e: Error) => latest.get(path) === n && (target.value = { data: target.value.data, error: e.message }),
      );
  }
  return s;
}

// forget drops cached resources whose path starts with prefix (except keep), so they refetch on next use.
export function forget(prefix: string, keep?: string) {
  for (const k of cache.keys()) if (k.startsWith(prefix) && k !== keep) cache.delete(k);
}

export const session = () => resource<Session | null>("/session");

export const guildId = browser ? (new URLSearchParams(location.search).get("id") ?? "") : "";

export function formatBytes(n: number) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) (n /= 1024), i++;
  return `${n.toFixed(i ? 2 : 0)} ${units[i]}`;
}
