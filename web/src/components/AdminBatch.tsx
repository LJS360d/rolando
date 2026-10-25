import { useEffect, useRef, useState } from "preact/hooks";
import {
  adminPage,
  adminPath,
  allMatching,
  batchRun,
  batchTargets,
  cancelBatch,
  clearSelection,
  runBatch,
  selected,
  selectedRows,
  type BatchOp,
  type BatchTarget,
} from "../lib/admin";
import { resource, type Config, type GuildRow } from "../lib/api";
import { configBools, configNumbers } from "./ConfigForm";

const ops: Record<BatchOp, { label: string; icon: string; describe: string; danger?: boolean }> = {
  load: { label: "Load", icon: "icon-[fa6-solid--play]", describe: "Load the chain of" },
  unload: { label: "Unload", icon: "icon-[fa6-solid--pause]", describe: "Unload the chain of" },
  config: { label: "Edit config", icon: "icon-[fa6-solid--sliders]", describe: "Change the config of" },
  reset: { label: "Clear data", icon: "icon-[fa6-solid--eraser]", describe: "Delete all messages and media (config is kept) of", danger: true },
  leave: { label: "Leave", icon: "icon-[fa6-solid--right-from-bracket]", describe: "Leave and delete all data of", danger: true },
};

const tool = "grid size-8 place-items-center rounded hover:bg-neutral-800";

const flat = (g: GuildRow): Record<string, unknown> => ({
  id: g.id,
  name: g.name,
  members: g.members,
  state: g.state,
  ...Object.fromEntries(Object.entries(g.config).map(([k, v]) => [`config.${k}`, v])),
  ...Object.fromEntries(Object.entries(g.analytics).map(([k, v]) => [`analytics.${k}`, v])),
});

const csvCell = (v: unknown) => {
  const s = String(v ?? "");
  return /[",\n]/.test(s) ? `"${s.replaceAll('"', '""')}"` : s;
};

function download(name: string, type: string, text: string) {
  const a = document.createElement("a");
  a.href = URL.createObjectURL(new Blob([text], { type }));
  a.download = name;
  a.click();
  URL.revokeObjectURL(a.href);
}

export function BatchBar() {
  const [op, setOp] = useState<BatchOp | null>(null);
  const [note, setNote] = useState<string>();
  const data = adminPage.value.data;
  const count = allMatching.value ? (data?.total ?? 0) : selected.value.size;
  const running = (batchRun.value && !batchRun.value.done) || !!data?.batch;
  useEffect(() => {
    if (!count) setNote(undefined);
  }, [count]);
  if (!count) return null;

  const withRows = (fn: (rows: GuildRow[]) => unknown) => () => selectedRows().then(fn).catch((e: Error) => setNote(e.message));
  const copy = withRows(async (rows) => {
    await navigator.clipboard.writeText(rows.map((g) => g.id).join("\n"));
    setNote(`Copied ${rows.length} IDs`);
  });
  const exportCSV = withRows((rows) => {
    const flats = rows.map(flat);
    const cols = Object.keys(flats[0] ?? {});
    download("guilds.csv", "text/csv", [cols, ...flats.map((r) => cols.map((c) => r[c]))].map((r) => r.map(csvCell).join(",")).join("\n"));
  });
  const exportJSON = withRows((rows) => download("guilds.json", "application/json", JSON.stringify(rows, null, 2)));
  const broadcast = withRows((rows) => {
    sessionStorage.setItem("broadcast-ids", JSON.stringify(rows.map((g) => g.id)));
    location.assign("/admin/broadcast");
  });

  return (
    <div class="mb-3 flex flex-wrap items-center gap-1 rounded bg-indigo-500/10 px-3 py-1.5 text-sm">
      <span class="mr-1 font-medium">{count.toLocaleString()} selected</span>
      <button class="link mr-2" onClick={clearSelection}>Clear</button>
      {(Object.entries(ops) as [BatchOp, (typeof ops)[BatchOp]][]).map(([k, o]) => (
        <button
          class={`flex items-center gap-1.5 rounded px-2 py-1 hover:bg-neutral-800 disabled:pointer-events-none disabled:opacity-40 ${o.danger ? "text-red-400" : ""}`}
          disabled={running}
          title={running ? "A batch is running" : undefined}
          onClick={() => setOp(k)}
        >
          <span class={o.icon} />{o.label}
        </button>
      ))}
      <span class="mx-1 h-5 w-px bg-neutral-700" />
      <button class={tool} onClick={broadcast} title="Broadcast to selected" aria-label="Broadcast to selected"><span class="icon-[fa6-solid--bullhorn]" /></button>
      <button class={tool} onClick={copy} title="Copy IDs" aria-label="Copy IDs"><span class="icon-[fa6-solid--copy]" /></button>
      <button class={tool} onClick={exportCSV} title="Export CSV" aria-label="Export CSV"><span class="icon-[fa6-solid--file-csv]" /></button>
      <button class={tool} onClick={exportJSON} title="Export JSON" aria-label="Export JSON"><span class="icon-[fa6-solid--file-code]" /></button>
      {note && <span class="muted ml-2">{note}</span>}
      {op && <BatchDialog op={op} onClose={() => setOp(null)} />}
    </div>
  );
}

type Edits = Partial<Record<keyof Config, number | boolean>>;

function BatchDialog({ op, onClose }: { op: BatchOp; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  const [targets, setTargets] = useState<BatchTarget[]>();
  const [error, setError] = useState<string>();
  const [typed, setTyped] = useState("");
  const [edits, setEdits] = useState<Edits>({});
  const o = ops[op];
  useEffect(() => {
    ref.current!.showModal();
    batchTargets(op).then(setTargets, (e: Error) => setError(e.message));
  }, []);
  const ready = !!targets && (!o.danger || typed === String(targets.length)) && (op !== "config" || Object.keys(edits).length > 0);
  const run = (e: SubmitEvent) => {
    e.preventDefault();
    if (!ready) return;
    // clearSelection unmounts the bar before the dialog's close event fires, so onClose would never run
    onClose();
    ref.current!.close();
    clearSelection();
    runBatch(op, targets, op === "config" ? (edits as Record<string, number | boolean>) : undefined);
  };

  return (
    <dialog ref={ref} class="m-auto w-full max-w-lg rounded bg-neutral-900 text-neutral-200 backdrop:bg-black/70" onClose={onClose}>
      <form class="flex flex-col gap-3 p-4" onSubmit={run}>
        <h2 class={`mb-0 flex items-center gap-2 ${o.danger ? "text-red-400" : ""}`}><span class={o.icon} />{o.label}</h2>
        {error && <p class="banner mb-0">{error}</p>}
        {!targets && !error && <p class="muted flex items-center gap-2"><span class="icon-[fa6-solid--spinner] animate-spin" />Resolving guilds…</p>}
        {targets && (
          <>
            <p>{o.describe} <b>{targets.length.toLocaleString()}</b> guild{targets.length > 1 ? "s" : ""}:</p>
            <ul class="max-h-40 overflow-y-auto rounded bg-neutral-950/60 px-3 py-2 text-sm">
              {targets.slice(0, 100).map((t) => (
                <li class="flex gap-2"><span class="truncate">{t.name || "?"}</span><span class="muted ml-auto text-xs tabular-nums">{t.id}</span></li>
              ))}
              {targets.length > 100 && <li class="muted">…and {(targets.length - 100).toLocaleString()} more</li>}
            </ul>
          </>
        )}
        {op === "config" && targets && <ConfigEdits edits={edits} setEdits={setEdits} />}
        {o.danger && targets && (
          <label class="flex flex-col gap-1 text-sm">
            <span>Type <b>{targets.length}</b> to confirm. This can't be undone.</span>
            <input class="input" inputMode="numeric" autoFocus value={typed} onInput={(e) => setTyped(e.currentTarget.value.trim())} />
          </label>
        )}
        <div class="flex justify-end gap-2">
          <button type="button" class="btn bg-neutral-700 hover:bg-neutral-600" onClick={() => ref.current!.close()}>Cancel</button>
          <button class={`btn flex items-center gap-2 disabled:pointer-events-none disabled:opacity-50 ${o.danger ? "btn-danger" : ""}`} disabled={!ready}>
            <span class={o.icon} />{o.label}{targets ? ` (${targets.length.toLocaleString()} guilds)` : ""}
          </button>
        </div>
      </form>
    </dialog>
  );
}

// ConfigEdits picks which config fields to overwrite; unticked ones stay per guild.
function ConfigEdits({ edits, setEdits }: { edits: Edits; setEdits: (fn: (e: Edits) => Edits) => void }) {
  const toggle = (k: keyof Config, initial: number | boolean) =>
    setEdits((e) => {
      const { [k]: had, ...rest } = e;
      return had === undefined ? { ...e, [k]: initial } : rest;
    });
  const set = (k: keyof Config, v: number | boolean) => setEdits((e) => ({ ...e, [k]: v }));
  const row = (k: keyof Config) => `flex items-center gap-3 rounded border-l-2 py-0.5 pl-2 text-sm ${k in edits ? "border-indigo-400 bg-indigo-500/5" : "border-transparent"}`;
  return (
    <div class="flex flex-col gap-1">
      <p class="muted text-xs">Tick the fields to overwrite. N-gram size can't be batch edited (it rebuilds every chain)</p>
      {configBools.map(([k, label]) => (
        <div class={row(k)}>
          <input type="checkbox" checked={k in edits} onChange={() => toggle(k, false)} aria-label={`Change ${label}`} />
          <span class="flex-1">{label}</span>
          <select class="input w-24 py-0.5" disabled={!(k in edits)} value={String(edits[k] ?? false)} onChange={(e) => set(k, e.currentTarget.value === "true")}>
            <option value="true">on</option>
            <option value="false">off</option>
          </select>
        </div>
      ))}
      {configNumbers
        .filter(([k]) => k !== "n_gram_size")
        .map(([k, label, min, max]) => (
          <div class={row(k)}>
            <input type="checkbox" checked={k in edits} onChange={() => toggle(k, min)} aria-label={`Change ${label}`} />
            <span class="flex-1">{label}</span>
            <input
              class="input w-24 py-0.5 text-right"
              type="number"
              required
              min={min}
              max={max}
              disabled={!(k in edits)}
              value={(edits[k] as number | undefined) ?? ""}
              onInput={(e) => set(k, e.currentTarget.valueAsNumber)}
            />
          </div>
        ))}
    </div>
  );
}

export function BatchPanel() {
  const live = batchRun.value;
  const server = adminPage.value.data?.batch;
  const running = live ? !live.done : !!server;
  const [error, setError] = useState<string>();
  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => resource(adminPath.value, true), 5000);
    return () => clearInterval(t);
  }, [running]);
  if (!live && !server) return null;

  const s = live ?? { ...server!, errors: [] as NonNullable<typeof live>["errors"] };
  const label = ops[s.op as BatchOp]?.label ?? s.op;
  const pct = (n: number) => `${s.total ? (n / s.total) * 100 : 0}%`;
  const cancel = () => cancelBatch().catch((e: Error) => setError(e.message));

  return (
    <div class="mb-3 rounded border border-neutral-800 bg-neutral-950/60 p-3 text-sm">
      <div class="flex flex-wrap items-center gap-2">
        <span
          class={
            running
              ? "icon-[fa6-solid--spinner] animate-spin text-indigo-400"
              : s.failed || live?.error || live?.cancelled
                ? "icon-[fa6-solid--circle-xmark] text-red-400"
                : "icon-[fa6-solid--circle-check] text-emerald-400"
          }
        />
        <b>{label}</b>
        <span class="muted tabular-nums">
          {(s.ok + s.failed).toLocaleString()} / {s.total.toLocaleString()} · {s.ok.toLocaleString()} ok · {s.failed.toLocaleString()} failed
          {live?.cancelled && " · cancelled"}
        </span>
        {running ? (
          <button class="btn btn-danger ml-auto flex items-center gap-1.5 py-1" onClick={cancel}><span class="icon-[fa6-solid--stop]" />Cancel</button>
        ) : (
          live && <button class="link ml-auto" onClick={() => (batchRun.value = null)}>Dismiss</button>
        )}
      </div>
      <div class="mt-2 flex h-1.5 overflow-hidden rounded-full bg-neutral-800">
        <div class="bg-emerald-400 transition-all" style={{ width: pct(s.ok) }} />
        <div class="bg-red-400 transition-all" style={{ width: pct(s.failed) }} />
      </div>
      {!live && <p class="muted mt-2 text-xs">Running without a connected client: per-guild errors are in the bot logs.</p>}
      {(live?.error || error) && <p class="mt-2 text-red-300">{live?.error ?? error}</p>}
      {s.errors.length > 0 && (
        <details class="mt-2">
          <summary class="cursor-pointer text-red-300">{s.errors.length.toLocaleString()} failed</summary>
          <ul class="mt-1 max-h-48 overflow-y-auto">
            {s.errors.map((e) => (
              <li class="flex gap-2 py-0.5"><span class="max-w-48 truncate">{e.name || e.id}</span><span class="muted">{e.error}</span></li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}
