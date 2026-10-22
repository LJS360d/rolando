import { formatBytes, login, resource, session, type ChainState, type GuildRow } from "../lib/api";

export function GuildIcon({ g, class: cls }: { g: Pick<GuildRow, "icon" | "name">; class: string }) {
  if (g.icon) return <img class={`${cls} rounded-2xl object-cover`} src={g.icon} alt="" />;
  const initials = g.name.split(/\s+/).map((w) => w[0]).join("").slice(0, 3);
  return <span class={`${cls} grid place-items-center rounded-2xl bg-indigo-600 font-bold text-white`}>{initials}</span>;
}

const states: Record<ChainState, [string, string]> = {
  loaded: ["bg-emerald-400", "Chain is active"],
  unloaded: ["bg-neutral-600", "Chain is inactive"],
  fetching: ["animate-pulse bg-amber-400", "Train in progress"],
};

export function ChainStateBar({ state, class: cls = "h-8" }: { state: ChainState; class?: string }) {
  const [color, label] = states[state] ?? states.unloaded;
  return (
    <span class="-mx-1 flex shrink-0 cursor-help self-center px-1" title={label} aria-label={label} role="img">
      <span class={`w-1 rounded-full ${color} ${cls}`} />
    </span>
  );
}

export default function GuildLinks() {
  const sess = session().value;
  if (sess.data === null)
    return (
      <div class="card flex flex-col items-center gap-3 py-12 text-center">
        <span class="icon-[fa6-solid--lock] size-8 text-neutral-500" />
        <p>Log in with Discord to see the servers you manage.</p>
        <button class="btn flex items-center gap-2 bg-[#5865f2] hover:bg-[#4752c4]" onClick={login}>
          <span class="icon-[fa6-brands--discord]" /> Login
        </button>
      </div>
    );
  return <Grid />;
}

function Grid() {
  const { data, error } = resource<GuildRow[]>("/me/guilds").value;
  if (error) return <p class="banner">{error}</p>;
  if (!data)
    return (
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {[0, 1, 2, 3].map(() => <div class="card h-28 animate-pulse" />)}
      </div>
    );
  if (!data.length)
    return (
      <div class="card py-12 text-center">
        <span class="icon-[fa6-solid--ghost] mb-3 size-8 text-neutral-500" />
        <p class="muted">Rolando is not in any server you manage.</p>
      </div>
    );
  return (
    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {data.map((g) => (
        <a
          href={`/guild?id=${g.id}`}
          class="card mb-0 flex flex-col gap-3 text-neutral-200 ring-1 ring-transparent transition hover:no-underline hover:ring-indigo-500/60"
        >
          <div class="flex items-center gap-3">
            <ChainStateBar state={g.state} class="h-12" />
            <GuildIcon g={g} class="size-12 shrink-0" />
            <p class="min-w-0 truncate font-semibold" title={g.name}>{g.name}</p>
          </div>
          <div class="muted flex flex-wrap gap-x-4 gap-y-1 text-sm">
            <span class="flex items-center gap-1.5"><span class="icon-[fa6-solid--users]" />{g.members}</span>
            <span class="flex items-center gap-1.5"><span class="icon-[fa6-solid--message]" />{g.analytics.messages}</span>
            <span class="flex items-center gap-1.5"><span class="icon-[fa6-solid--database]" />{formatBytes(g.analytics.bytes)}</span>
          </div>
        </a>
      ))}
    </div>
  );
}
