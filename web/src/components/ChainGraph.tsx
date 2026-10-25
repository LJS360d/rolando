import { useEffect, useState } from "preact/hooks";
import { api, guildId, resource, type Graph, type GuildRow, type Link } from "../lib/api";

const W = 760, ROW = 30, CX = W / 2, LX = 190, RX = W - 190;
const clip = (w: string) => (w.length > 18 ? w.slice(0, 17) + "…" : w);

export default function ChainGraph() {
  const summary = resource<GuildRow>(`/guilds/${guildId}/summary`);
  const state = summary.value.data?.state;
  const loaded = state === "loaded" || state === "fetching";
  const [graph, setGraph] = useState<Graph>();
  const [trail, setTrail] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  // keep: index of the breadcrumb clicked; "reset" starts a new trail.
  const go = (word: string, keep?: number | "reset") => {
    setBusy(true);
    setError(undefined);
    api<Graph>("GET", `/guilds/${guildId}/graph?word=${encodeURIComponent(word)}`)
      .then((g) => {
        if (!g.word) return setError(word ? `“${word}” is not in the chain` : "The chain is empty");
        setGraph(g);
        setTrail((t) => (keep === "reset" ? [g.word] : keep !== undefined ? t.slice(0, keep + 1) : [...t.slice(-7), g.word]));
        if (!loaded) resource(`/guilds/${guildId}/summary`, true);
      }, (e: Error) => setError(e.message))
      .finally(() => setBusy(false));
  };
  useEffect(() => { if (loaded && !graph) go("", "reset"); }, [loaded]);

  const search = (e: SubmitEvent) => {
    e.preventDefault();
    const w = new FormData(e.currentTarget as HTMLFormElement).get("word") as string;
    if (w.trim()) go(w.trim());
  };

  return (
    <section class="card">
      <div class="mb-3 flex flex-wrap items-center gap-3">
        <h2 class="mb-0 mr-auto flex items-center gap-2"><span class="icon-[fa6-solid--diagram-project] text-indigo-400" />Chain graph</h2>
        {graph && (
          <form class="flex gap-2" onSubmit={search}>
            <input class="input w-40" name="word" placeholder="Find a word" />
            <button class="btn" title="Search" aria-label="Search"><span class="icon-[fa6-solid--magnifying-glass]" /></button>
            <button type="button" class="btn bg-neutral-700 hover:bg-neutral-600" onClick={() => go("", "reset")} title="Start over from a random word" aria-label="Start over from a random word">
              <span class="icon-[fa6-solid--dice]" />
            </button>
          </form>
        )}
      </div>
      {error && <p class="banner">{error}</p>}
      {!graph ? (
        <div class="flex flex-col items-center gap-3 py-12 text-center">
          <span class="icon-[fa6-solid--circle-nodes] size-10 text-neutral-600" />
          {state === "unloaded" ? (
            <p class="muted max-w-sm">The chain is unloaded. Load it to explore it.</p>
          ) : (
            <p class="muted">{busy ? "Loading…" : ""}</p>
          )}
        </div>
      ) : (
        <>
          <nav class="muted mb-2 flex flex-wrap items-center gap-1 text-sm">
            {trail.map((w, i) => (
              <>
                {i > 0 && <span class="icon-[fa6-solid--chevron-right] size-2.5" />}
                {i === trail.length - 1 ? <b class="text-neutral-200">{clip(w)}</b> : <button class="link" onClick={() => go(w, i)}>{clip(w)}</button>}
              </>
            ))}
          </nav>
          <Ego graph={graph} onPick={go} busy={busy} />
          <p class="muted mt-2 flex justify-between text-xs">
            <span><span class="icon-[fa6-solid--arrow-left]" /> words that come before</span>
            <span>words that come after <span class="icon-[fa6-solid--arrow-right]" /></span>
          </p>
        </>
      )}
    </section>
  );
}

function Ego({ graph, onPick, busy }: { graph: Graph; onPick: (w: string) => void; busy: boolean }) {
  const ins = graph.in ?? [], outs = graph.out ?? [];
  const inMore = graph.in_total - ins.length, outMore = graph.out_total - outs.length;
  const H = Math.max(ins.length + (inMore > 0 ? 1 : 0), outs.length + (outMore > 0 ? 1 : 0), 3) * ROW + 20;
  const cy = H / 2;
  const max = Math.max(1, ...ins.map((l) => l.c), ...outs.map((l) => l.c));
  const stroke = (c: number) => 1 + (5 * Math.log1p(c)) / Math.log1p(max);
  const y = (i: number, n: number) => cy + (i - (n - 1) / 2) * ROW;
  const half = Math.min(clip(graph.word).length * 6 + 20, 130);

  const more = (n: number, shown: number, left: boolean) =>
    n > 0 && (
      <text
        x={left ? LX - 10 : RX + 10}
        y={y(shown, shown + 1)}
        dominant-baseline="middle"
        text-anchor={left ? "end" : "start"}
        class="fill-neutral-500 text-[12px] italic"
      >
        <title>{`Showing the top ${shown} of ${n + shown}`}</title>+ {n.toLocaleString()} more…
      </text>
    );

  const side = (links: Link[], left: boolean, extra: number) =>
    links.map((l, i) => {
      const ny = y(i, links.length + extra);
      const nx = left ? LX : RX;
      const cx = left ? CX - half : CX + half;
      return (
        <g class="group cursor-pointer" onClick={() => onPick(l.w)}>
          <title>{`${l.w} · ${l.c}×`}</title>
          <path
            d={`M${nx} ${ny} C${(nx + cx) / 2} ${ny} ${(nx + cx) / 2} ${cy} ${cx} ${cy}`}
            fill="none"
            class={`${left ? "stroke-sky-500/40" : "stroke-indigo-500/50"} transition group-hover:stroke-amber-400`}
            stroke-width={stroke(l.c)}
          />
          <circle cx={nx} cy={ny} r={4} class={`${left ? "fill-sky-400" : "fill-indigo-400"} group-hover:fill-amber-400`} />
          <text
            x={left ? nx - 10 : nx + 10}
            y={ny}
            dominant-baseline="middle"
            text-anchor={left ? "end" : "start"}
            class="fill-neutral-200 text-[13px] group-hover:fill-amber-300"
          >
            {left && <tspan class="fill-neutral-500 text-[11px]">{l.c} </tspan>}
            {clip(l.w)}
            {!left && <tspan class="fill-neutral-500 text-[11px]"> {l.c}</tspan>}
          </text>
        </g>
      );
    });

  return (
    <svg viewBox={`0 0 ${W} ${H}`} class={`w-full rounded-lg bg-neutral-950/60 transition-opacity ${busy ? "opacity-50" : ""}`}>
      {side(ins, true, inMore > 0 ? 1 : 0)}
      {side(outs, false, outMore > 0 ? 1 : 0)}
      {more(inMore, ins.length, true)}
      {more(outMore, outs.length, false)}
      <rect x={CX - half} y={cy - 18} width={half * 2} height={36} rx={18} class="fill-indigo-600" />
      <text x={CX} y={cy} dominant-baseline="middle" text-anchor="middle" class="fill-white text-[16px] font-semibold">{clip(graph.word)}</text>
      {!ins.length && !outs.length && <text x={CX} y={cy + 40} text-anchor="middle" class="fill-neutral-500 text-[12px]">no links</text>}
    </svg>
  );
}
