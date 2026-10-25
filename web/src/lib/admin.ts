import { computed, effect, signal } from "@preact/signals";
import type { Field, Filter } from "../components/FilterBar";
import { api, browser, ndjson, resource, type AdminPage, type GuildRow, type Resource } from "./api";

export const adminPath = signal(`/admin/guilds?${(browser && location.search.slice(1)) || "sort=bytes&dir=desc&page=1"}`);
export const adminQuery = computed(() => adminPath.value.split("?")[1] ?? "");
export const adminParams = computed(() => new URLSearchParams(adminQuery.value));

if (browser) effect(() => history.replaceState(null, "", `?${adminQuery.value}`));

// setAdminQuery replaces the given params, keeping the rest (filters included).
export function setAdminQuery(patch: Record<string, string | string[]>) {
  const p = new URLSearchParams(adminQuery.value);
  for (const [k, v] of Object.entries(patch)) {
    p.delete(k);
    for (const x of [v].flat()) if (x !== "") p.append(k, x);
  }
  adminPath.value = `/admin/guilds?${p}`;
}

export const adminFilters = computed(() =>
  adminParams.value.getAll("f").map((s): Filter => {
    const [key, op, ...rest] = s.split(":");
    return { key, op, value: rest.join(":") };
  }),
);
export const setAdminFilters = (fs: Filter[]) => setAdminQuery({ f: fs.map((f) => `${f.key}:${f.op}:${f.value}`), page: "1" });

export const guildFields: Field[] = [
  { key: "name", label: "Name", type: "string" },
  { key: "id", label: "ID", type: "string" },
  { key: "state", label: "Chain", type: "enum", values: ["loaded", "unloaded", "fetching", "empty"] },
  { key: "members", label: "Members", type: "number" },
  { key: "analytics.bytes", label: "Size", type: "number", scale: 2 ** 20, unit: "MB" },
  { key: "analytics.messages", label: "Messages", type: "number" },
  { key: "analytics.words", label: "Words", type: "number" },
  { key: "analytics.complexity_score", label: "Complexity", type: "number" },
  { key: "analytics.gifs", label: "Gifs", type: "number" },
  { key: "analytics.images", label: "Images", type: "number" },
  { key: "analytics.videos", label: "Videos", type: "number" },
  { key: "config.reply_rate", label: "Reply rate", type: "number" },
  { key: "config.reaction_rate", label: "Reaction rate", type: "number" },
  { key: "config.vc_join_rate", label: "VC join rate", type: "number" },
  { key: "config.max_size_mb", label: "Max size (MB)", type: "number" },
  { key: "config.n_gram_size", label: "N-gram size", type: "number" },
  { key: "config.markov_max_branches", label: "Max branches", type: "number" },
  { key: "config.pings", label: "Pings", type: "bool" },
  { key: "config.premium", label: "Premium", type: "bool" },
];

let last: Resource<AdminPage> = {};
// adminPage keeps the last settled result while the next path loads, so islands keep their content (and stats, being path-independent, don't rerender).
export const adminPage = computed(() => {
  const r = resource<AdminPage>(adminPath.value).value;
  return (last = r.data || r.error ? r : last);
});
export const adminLoading = computed(() => {
  const r = resource<AdminPage>(adminPath.value).value;
  return !r.data && !r.error;
});

export const selected = signal<Map<string, GuildRow>>(new Map());
// allMatching targets every guild matching the search and filters it was picked under.
export const allMatching = signal(false);
const matchKey = computed(() => JSON.stringify([adminParams.value.get("q"), adminParams.value.getAll("f")]));
effect(() => {
  matchKey.value;
  allMatching.value = false;
});
export const clearSelection = () => ((selected.value = new Map()), (allMatching.value = false));

export type BatchOp = "load" | "unload" | "config" | "reset" | "leave";
export type BatchTarget = { id: string; name: string };
export type BatchRun = {
  op: BatchOp;
  total: number;
  ok: number;
  failed: number;
  errors: (BatchTarget & { error: string })[];
  done?: boolean;
  cancelled?: boolean;
  error?: string;
};

export const batchRun = signal<BatchRun | null>(null);

const selection = () => (allMatching.value ? { query: adminQuery.value } : { ids: [...selected.value.keys()] });

// batchTargets asks the server exactly which guilds the op would hit.
export const batchTargets = (op: BatchOp) =>
  api<{ targets: BatchTarget[] }>("POST", "/admin/batch", { op, ...selection(), dry: true }).then((r) => r.targets);

export async function selectedRows(): Promise<GuildRow[]> {
  if (!allMatching.value) return [...selected.value.values()];
  // page would offset the single all=1 page past the end
  const p = new URLSearchParams(adminQuery.value);
  p.delete("page");
  p.set("all", "1");
  return (await api<AdminPage>("GET", `/admin/guilds?${p}`)).rows;
}

export const cancelBatch = () => api("POST", "/admin/batch/cancel");

type BatchEvent = { t: "start" | "item" | "ping" | "done"; id?: string; name?: string; error?: string; cancelled?: boolean };

// runBatch streams the batch's NDJSON events into batchRun. If the stream
// drops the server keeps going, and the panel falls back to the listing's batch status.
export async function runBatch(op: BatchOp, targets: BatchTarget[], params?: Record<string, number | boolean>) {
  batchRun.value = { op, total: targets.length, ok: 0, failed: 0, errors: [] };
  const update = (fn: (r: BatchRun) => Partial<BatchRun>) => (batchRun.value = { ...batchRun.value!, ...fn(batchRun.value!) });
  let streaming = false;
  try {
    for await (const e of ndjson<BatchEvent>("POST", "/admin/batch", { op, ids: targets.map((t) => t.id), params })) {
      streaming = true;
      if (e.t === "item") update((r) => (e.error ? { failed: r.failed + 1, errors: [...r.errors, { id: e.id!, name: e.name ?? "", error: e.error }] } : { ok: r.ok + 1 }));
      if (e.t === "done") update(() => ({ done: true, cancelled: e.cancelled }));
    }
    if (!batchRun.value?.done) batchRun.value = null;
  } catch (e) {
    if (streaming) batchRun.value = null;
    else update(() => ({ done: true, error: (e as Error).message }));
  }
  resource(adminPath.value, true);
}
