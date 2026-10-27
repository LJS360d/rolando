import { useState } from "preact/hooks";
import { api, formatBytes, guildId, resource, type GuildRow } from "../lib/api";
import { ChainStateBar, GuildIcon } from "./GuildLinks";

export default function GuildHeader() {
  const { data: g, error } = resource<GuildRow>(`/guilds/${guildId}/summary`).value;
  const [loading, setLoading] = useState(false);
  const load = () => {
    setLoading(true);
    api("GET", `/guilds/${guildId}/graph`)
      .then(() => resource(`/guilds/${guildId}/summary`, true), (e: Error) => alert(e.message))
      .finally(() => setLoading(false));
  };
  if (error) return <p class="banner">{error}</p>;
  if (!g) return <div class="card h-40 animate-pulse" />;
  const a = g.analytics;
  const max = g.config.max_size_mb * 2 ** 20;
  const stats: [string, string, number][] = [
    ["icon-[fa6-solid--message]", "Messages", a.messages],
    ["icon-[fa6-solid--font]", "Words", a.words],
    ["icon-[fa6-solid--diagram-project]", "Complexity", a.complexity_score],
    ["icon-[fa6-solid--film]", "Gifs", a.gifs],
    ["icon-[fa6-solid--image]", "Images", a.images],
    ["icon-[fa6-solid--video]", "Videos", a.videos],
  ];
  return (
    <section class="card">
      <div class="flex flex-wrap items-center gap-4">
        <ChainStateBar state={g.state} class="h-16" />
        <GuildIcon g={g} class="size-16 shrink-0 text-xl" />
        <div class="min-w-0 flex-1">
          <h1 class="mb-1 truncate">{g.name}</h1>
          <div class="muted flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
            <span class="flex items-center gap-1.5"><span class="icon-[fa6-solid--users]" />{g.members} members</span>
            {g.state === "unloaded" && (
              <button class="btn flex items-center gap-2 px-2 py-1 text-xs" disabled={loading} onClick={load}>
                <span class={loading ? "icon-[fa6-solid--spinner] animate-spin" : "icon-[fa6-solid--upload]"} /> Load chain
              </button>
            )}
          </div>
        </div>
        <div class="w-full sm:w-64">
          <div class="muted mb-1 flex justify-between text-xs">
            <span class="flex items-center gap-1.5"><span class="icon-[fa6-solid--database]" />Storage</span>
            <span>{formatBytes(a.bytes)} / {formatBytes(max)}</span>
          </div>
          <div class="h-2 overflow-hidden rounded-full bg-neutral-800">
            <div class="h-full rounded-full bg-indigo-500" style={{ width: `${Math.min(100, (a.bytes / max) * 100)}%` }} />
          </div>
        </div>
      </div>
      <div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
        {stats.map(([icon, label, v]) => (
          <div class="rounded-lg bg-neutral-800/60 p-3">
            <div class="muted flex items-center gap-1.5 text-xs"><span class={icon} />{label}</div>
            <div class="mt-1 text-xl font-semibold tabular-nums">{v.toLocaleString()}</div>
          </div>
        ))}
      </div>
    </section>
  );
}
