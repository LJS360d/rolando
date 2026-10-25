import { useEffect, useState } from "preact/hooks";
import { api, guildId, resource, session, type Config, type GuildRow } from "../lib/api";

export const configNumbers: [keyof Config, string, number, number?][] = [
  ["reply_rate", "Reply rate", 0],
  ["reaction_rate", "Reaction rate", 0],
  ["vc_join_rate", "VC join rate", 0],
  ["n_gram_size", "N-gram size", 2, 8],
  ["max_size_mb", "Max size (MB)", 1],
  ["markov_max_branches", "Max transitions per prefix (0 = unlimited)", 0],
];

export const configBools: [keyof Config, string][] = [
  ["pings", "Pings"],
  ["premium", "Premium"],
];

const row = "flex items-center justify-between gap-3 rounded border-l-2 py-0.5 pl-2 text-sm";

export default function ConfigForm() {
  const summary = resource<GuildRow>(`/guilds/${guildId}/summary`);
  const g = summary.value.data;
  const [edits, setEdits] = useState<Partial<Config>>({});
  const [status, setStatus] = useState<"saving" | "saved" | { error: string }>();
  const config = g?.config;
  const changed = config ? (Object.keys(edits) as (keyof Config)[]).filter((k) => edits[k] !== config[k]) : [];

  useEffect(() => {
    if (!changed.length) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    addEventListener("beforeunload", warn);
    return () => removeEventListener("beforeunload", warn);
  }, [changed.length > 0]);

  if (!session().value.data?.o || !g || !config) return null;

  const value = <K extends keyof Config>(k: K) => (edits[k] ?? config[k]) as Config[K];
  const set = (k: keyof Config, v: number | boolean) => {
    setEdits((e) => ({ ...e, [k]: v }));
    if (status !== "saving") setStatus(undefined);
  };
  const save = (e: SubmitEvent) => {
    e.preventDefault();
    if (!changed.length) return;
    setStatus("saving");
    api<Config>("PUT", `/guilds/${guildId}/config`, { ...config, ...edits }).then(
      (c) => {
        summary.value = { data: { ...g, config: c } };
        setEdits({});
        setStatus("saved");
        setTimeout(() => setStatus((s) => (s === "saved" ? undefined : s)), 2000);
      },
      (err: Error) => setStatus({ error: err.message }),
    );
  };
  const leave = () =>
    confirm("Leave this server and delete its data?") &&
    api("POST", `/guilds/${guildId}/leave`).then(() => location.assign("/admin"), (err: Error) => setStatus({ error: err.message }));

  const mark = (k: keyof Config) => (changed.includes(k) ? "border-indigo-400 bg-indigo-500/5" : "border-transparent");
  const saving = status === "saving";

  return (
    <section class="card">
      <h2 class="flex items-center gap-2">
        <span class="icon-[fa6-solid--sliders] text-indigo-400" />Config
        {changed.length > 0 && <span class="ml-auto rounded-full bg-indigo-500/20 px-2 py-0.5 text-xs font-medium text-indigo-300">{changed.length} unsaved</span>}
      </h2>
      <form class="flex flex-col gap-2" onSubmit={save}>
        {configBools.map(([k, label]) => (
          <label class={`${row} ${mark(k)}`}>
            {label}
            <input class="mr-1 size-4" type="checkbox" checked={value(k) as boolean} onChange={(e) => set(k, e.currentTarget.checked)} />
          </label>
        ))}
        {configNumbers.map(([k, label, min, max]) => (
          <label class={`${row} ${mark(k)}`}>
            {label}
            <input
              class="input w-24 text-right"
              type="number"
              required
              min={min}
              max={max}
              value={value(k) as number}
              onInput={(e) => set(k, e.currentTarget.valueAsNumber)}
            />
          </label>
        ))}
        {changed.includes("n_gram_size") && (
          <p class="flex items-start gap-2 rounded bg-amber-900/40 px-3 py-2 text-xs text-amber-200">
            <span class="icon-[fa6-solid--triangle-exclamation] mt-0.5 shrink-0" />Changing the n-gram size rebuilds the whole chain in the background.
          </p>
        )}
        {typeof status === "object" && (
          <p class="flex items-start gap-2 rounded bg-red-900/40 px-3 py-2 text-xs text-red-200">
            <span class="icon-[fa6-solid--circle-exclamation] mt-0.5 shrink-0" />{status.error}
          </p>
        )}
        <div class="mt-1 flex gap-2">
          <button
            class={`btn flex flex-1 items-center justify-center gap-2 disabled:cursor-default disabled:opacity-50 ${status === "saved" ? "bg-emerald-600 hover:bg-emerald-600" : ""}`}
            disabled={saving || (!changed.length && status !== "saved")}
          >
            <span class={saving ? "icon-[fa6-solid--spinner] animate-spin" : status === "saved" ? "icon-[fa6-solid--check]" : "icon-[fa6-solid--floppy-disk]"} />
            {saving ? "Saving…" : status === "saved" ? "Saved" : changed.length ? `Save ${changed.length} change${changed.length > 1 ? "s" : ""}` : "No changes"}
          </button>
          {changed.length > 0 && !saving && (
            <button type="button" class="btn bg-neutral-700 hover:bg-neutral-600" onClick={() => (setEdits({}), setStatus(undefined))} title="Discard changes" aria-label="Discard changes">
              <span class="icon-[fa6-solid--rotate-left]" />
            </button>
          )}
        </div>
      </form>
      <div class="mt-4 border-t border-neutral-800 pt-4">
        <button class="btn btn-danger flex w-full items-center justify-center gap-2" onClick={leave}><span class="icon-[fa6-solid--right-from-bracket]" />Leave</button>
      </div>
    </section>
  );
}
