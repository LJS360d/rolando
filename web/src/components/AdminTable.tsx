import { useRef, useState } from "preact/hooks";
import { adminFilters, adminLoading, adminPage, adminParams, adminPath, allMatching, guildFields, selected, setAdminFilters, setAdminQuery } from "../lib/admin";
import { api, formatBytes, resource, type GuildRow } from "../lib/api";
import { ChainStateBar, GuildIcon } from "./GuildLinks";
import { BatchBar, BatchPanel } from "./AdminBatch";
import FilterBar from "./FilterBar";
import Pager from "./Pager";

const columns: [string, string][] = [
  ["members", "Members"],
  ["bytes", "Size"],
  ["messages", "Messages"],
  ["words", "Words"],
  ["complexity_score", "Complexity"],
  ["reply_rate", "Reply rate"],
];

const iconBtn = "grid size-8 place-items-center rounded transition hover:no-underline";

function InviteButton({ id }: { id: string }) {
  const [state, setState] = useState<{ url?: string; error?: string }>({});
  if (state.url)
    return (
      <a class={`${iconBtn} text-emerald-400 hover:bg-emerald-600/30`} href={state.url} target="_blank" rel="noopener" title={`Open ${state.url}`}>
        <span class="icon-[fa6-solid--door-open]" />
      </a>
    );
  const create = () =>
    api<{ url: string }>("POST", `/guilds/${id}/invite`).then((r) => setState({ url: r.url }), (e: Error) => setState({ error: e.message }));
  return (
    <button
      class={`${iconBtn} ${state.error ? "text-red-400 hover:bg-red-600/30" : "text-neutral-300 hover:bg-neutral-700"}`}
      onClick={create}
      title={state.error ? `No invite: ${state.error}` : "Create invite"}
      aria-label="Create invite"
    >
      <span class="icon-[fa6-solid--door-closed]" />
    </button>
  );
}

export default function AdminTable() {
  const q = adminParams.value;
  const sort = q.get("sort") ?? "bytes";
  const dir = q.get("dir") ?? "desc";
  const page = (p: number) => setAdminQuery({ page: String(p) });
  const sortBy = (k: string) => setAdminQuery({ sort: k, dir: k === sort && dir === "desc" ? "asc" : "desc", page: "1" });
  const { data, error } = adminPage.value;
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const search = (v: string) => {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setAdminQuery({ q: v.trim(), page: "1" }), 300);
  };
  const sel = selected.value;
  const all = allMatching.value;
  const rows = data?.rows ?? [];
  const pagePicked = rows.length > 0 && (all || rows.every((g) => sel.has(g.id)));
  const pick = (fn: (next: Map<string, GuildRow>) => void) => {
    const next = new Map(sel);
    fn(next);
    selected.value = next;
    allMatching.value = false;
  };
  const toggle = (g: GuildRow) => pick((m) => m.delete(g.id) || m.set(g.id, g));
  const togglePage = () => pick((m) => rows.forEach((g) => (pagePicked ? m.delete(g.id) : m.set(g.id, g))));
  const leave = async (g: GuildRow) => {
    if (!confirm(`Leave guild ${g.name} and delete its data?`)) return;
    await api("POST", `/guilds/${g.id}/leave`).catch((e: Error) => alert(e.message));
    resource(adminPath.value, true);
  };

  return (
    <section class="card">
      <div class="mb-3 flex flex-wrap items-center gap-3">
        <label class="relative">
          <span class="icon-[fa6-solid--magnifying-glass] muted absolute top-1/2 left-2.5 -translate-y-1/2" />
          <input class="input w-56 pl-8" placeholder="Name or ID" value={q.get("q") ?? ""} onInput={(e) => search(e.currentTarget.value)} />
        </label>
        <FilterBar fields={guildFields} filters={adminFilters.value} onChange={setAdminFilters} />
        {data && <div class="ml-auto"><Pager page={data.page} pages={data.pages} onPage={page} /></div>}
      </div>
      {error && <p class="banner">{error}</p>}
      <BatchPanel />
      <BatchBar />
      {data && pagePicked && data.total > rows.length && (
        <p class="mb-3 text-center text-sm">
          {all ? (
            <>All {data.total.toLocaleString()} matching guilds are selected. <button class="link" onClick={() => (allMatching.value = false)}>Select only the ticked ones</button></>
          ) : (
            <>All {rows.length} on this page are selected. <button class="link" onClick={() => (allMatching.value = true)}>Select all {data.total.toLocaleString()} matching guilds</button></>
          )}
        </p>
      )}
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="muted text-xs uppercase">
            <tr>
              <th class="w-8"><input type="checkbox" checked={pagePicked} onChange={togglePage} aria-label="Select page" /></th>
              <th>Guild</th>
              {columns.map(([k, label]) => (
                <th class="text-right">
                  <button class={`inline-flex items-center gap-1 uppercase hover:text-neutral-200 ${sort === k ? "text-indigo-300" : ""}`} onClick={() => sortBy(k)}>
                    {label}
                    {sort === k && <span class={dir === "asc" ? "icon-[fa6-solid--arrow-up-short-wide]" : "icon-[fa6-solid--arrow-down-wide-short]"} />}
                  </button>
                </th>
              ))}
              <th class="text-right">Gif / Img / Vid</th>
              <th></th>
            </tr>
          </thead>
          <tbody class={adminLoading.value ? "animate-pulse opacity-60" : ""}>
            {data?.rows.map((g) => (
              <tr class={`hover:bg-neutral-800/40 [&>td.n]:text-right [&>td.n]:tabular-nums ${all || sel.has(g.id) ? "bg-indigo-500/10" : ""}`}>
                <td><input type="checkbox" checked={all || sel.has(g.id)} disabled={all} onChange={() => toggle(g)} aria-label={`Select ${g.name}`} /></td>
                <td>
                  <a class="flex items-center gap-2" href={`/guild?id=${g.id}`}>
                    <ChainStateBar state={g.state} class="h-7" />
                    <GuildIcon g={g} class="size-8 shrink-0 text-xs" />
                    <span class="max-w-64 truncate">{g.name}</span>
                  </a>
                </td>
                <td class="n">{g.members.toLocaleString()}</td>
                <td class="n">
                  <span title={`${formatBytes(g.analytics.bytes)} / ${g.config.max_size_mb} MB`}>{formatBytes(g.analytics.bytes)}</span>
                  <div class="mt-1 ml-auto h-1 w-20 overflow-hidden rounded-full bg-neutral-800">
                    <div class="h-full bg-indigo-400" style={{ width: `${Math.min(100, (g.analytics.bytes / (g.config.max_size_mb * 2 ** 20)) * 100)}%` }} />
                  </div>
                </td>
                <td class="n">{g.analytics.messages.toLocaleString()}</td>
                <td class="n">{g.analytics.words.toLocaleString()}</td>
                <td class="n">{g.analytics.complexity_score}</td>
                <td class="n">{g.config.reply_rate}</td>
                <td class="n muted">{g.analytics.gifs} / {g.analytics.images} / {g.analytics.videos}</td>
                <td>
                  <div class="flex justify-end gap-1">
                    <InviteButton id={g.id} />
                    <button class={`${iconBtn} text-red-400 hover:bg-red-600/30`} onClick={() => leave(g)} title="Leave guild" aria-label="Leave guild">
                      <span class="icon-[fa6-solid--right-from-bracket]" />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {data && !data.rows.length && <tr><td colSpan={columns.length + 4} class="muted py-8 text-center">No guilds match.</td></tr>}
          </tbody>
        </table>
      </div>
      {data && data.pages > 1 && <div class="mt-3 flex justify-end"><Pager page={data.page} pages={data.pages} onPage={page} /></div>}
    </section>
  );
}
