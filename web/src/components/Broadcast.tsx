import { useState } from "preact/hooks";
import { api, resource, type AdminPage } from "../lib/api";
import { ChainStateBar, GuildIcon } from "./GuildLinks";

// handedOff takes the selection the admin table's "Broadcast to selected" left behind.
function handedOff() {
  try {
    const ids: string[] = JSON.parse(sessionStorage.getItem("broadcast-ids") ?? "[]");
    sessionStorage.removeItem("broadcast-ids");
    return new Set(ids);
  } catch {
    return new Set<string>();
  }
}

export default function Broadcast() {
  const { data, error } = resource<AdminPage>("/admin/guilds?all=1&sort=members").value;
  const [picked, setPicked] = useState<Set<string>>(handedOff);
  const [filter, setFilter] = useState("");
  const [result, setResult] = useState<string>();
  const [busy, setBusy] = useState(false);

  const shown = (data?.rows ?? []).filter((g) => g.name.toLowerCase().includes(filter.toLowerCase()) || g.id === filter);
  const toggle = (id: string) => setPicked((p) => (p.has(id) ? (p.delete(id), new Set(p)) : new Set(p.add(id))));
  const pickShown = (on: boolean) => setPicked((p) => (shown.forEach((g) => (on ? p.add(g.id) : p.delete(g.id))), new Set(p)));

  const send = async (e: SubmitEvent) => {
    e.preventDefault();
    const content = new FormData(e.currentTarget as HTMLFormElement).get("content");
    const target = picked.size ? `${picked.size} selected servers` : "ALL servers";
    if (!confirm(`Send to ${target}?`)) return;
    setBusy(true);
    try {
      const r = await api<{ sent: number; failed: number[] }>("POST", "/admin/broadcast", { content, guilds: [...picked] });
      setResult(`Sent to ${r.sent} servers.${r.failed.length ? ` Unreachable nodes: ${r.failed.join(", ")}.` : ""}`);
    } catch (err) {
      setResult((err as Error).message);
    }
    setBusy(false);
  };

  return (
    <form class="grid items-start gap-4 lg:grid-cols-2" onSubmit={send}>
      <section class="card mb-0 flex flex-col gap-3">
        <h2 class="mb-0 flex items-center gap-2"><span class="icon-[fa6-solid--message] text-indigo-400" />Message</h2>
        <textarea class="input min-h-48" name="content" rows={10} required placeholder="Sent to each server's system channel." />
        {result && <p class="banner mb-0">{result}</p>}
        <button class="btn flex items-center justify-center gap-2" disabled={busy}>
          <span class={busy ? "icon-[fa6-solid--spinner] animate-spin" : "icon-[fa6-solid--paper-plane]"} />
          Send to {picked.size ? `${picked.size} server${picked.size > 1 ? "s" : ""}` : "all servers"}
        </button>
      </section>
      <section class="card mb-0">
        <div class="mb-3 flex flex-wrap items-center gap-2">
          <h2 class="mb-0 mr-auto flex items-center gap-2"><span class="icon-[fa6-solid--server] text-indigo-400" />Servers</h2>
          <button type="button" class="link text-sm" onClick={() => pickShown(true)}>Select shown</button>
          <span class="muted">·</span>
          <button type="button" class="link text-sm" onClick={() => pickShown(false)}>Clear shown</button>
        </div>
        <input class="input mb-2 w-full" placeholder="Filter by name or ID" value={filter} onInput={(e) => setFilter(e.currentTarget.value)} />
        <p class="muted mb-2 text-xs">None selected = all servers.</p>
        {error && <p class="banner">{error}</p>}
        <ul class="max-h-[28rem] overflow-y-auto rounded-lg bg-neutral-950/60">
          {!data && <li class="muted p-4 text-center">Loading…</li>}
          {shown.map((g) => (
            <li>
              <label class={`flex cursor-pointer items-center gap-3 px-3 py-2 hover:bg-neutral-800/50 ${picked.has(g.id) ? "bg-indigo-500/10" : ""}`}>
                <input type="checkbox" checked={picked.has(g.id)} onChange={() => toggle(g.id)} />
                <ChainStateBar state={g.state} class="h-6" />
                <GuildIcon g={g} class="size-7 shrink-0 text-[10px]" />
                <span class="min-w-0 flex-1 truncate">{g.name}</span>
                <span class="muted text-xs tabular-nums">{g.members.toLocaleString()}</span>
              </label>
            </li>
          ))}
        </ul>
      </section>
    </form>
  );
}
