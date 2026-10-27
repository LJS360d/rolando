import { useSignal } from "@preact/signals";
import { useRef, useState } from "preact/hooks";
import { api, forget, guildId, resource, type GuildRow, type MediaPage } from "../lib/api";
import Pager from "./Pager";

type Kind = "gif" | "image" | "video";

const kinds: [Kind, string, string, keyof GuildRow["analytics"]][] = [
  ["gif", "Gifs", "icon-[fa6-solid--film]", "gifs"],
  ["image", "Images", "icon-[fa6-solid--image]", "images"],
  ["video", "Videos", "icon-[fa6-solid--video]", "videos"],
];

const imageRe = /\.(?:png|jpe?g|gif|webp|avif)$/i;
const videoRe = /\.(?:mp4|webm|mov)$/i;

const parse = (url: string) => {
  try {
    return new URL(url);
  } catch {
    return null;
  }
};

// embed maps page links of known hosts to something the browser can show:
// tenor's embed page, giphy's gif file, youtube's thumbnail.
function embed(u: URL): { img?: string; frame?: string } {
  const host = u.hostname.replace(/^(www|m)\./, "");
  let id: string | undefined;
  if (host === "tenor.com" && (id = u.pathname.match(/^\/view\/.*?(\d+)$/)?.[1])) return { frame: `https://tenor.com/embed/${id}` };
  if (host === "giphy.com" && (id = u.pathname.match(/^\/gifs\/(?:.*-)?(\w+)$/)?.[1])) return { img: `https://media.giphy.com/media/${id}/giphy.gif` };
  if (host === "youtube.com") id = u.searchParams.get("v") ?? u.pathname.match(/^\/shorts\/([\w-]+)/)?.[1];
  if (host === "youtu.be") id = u.pathname.slice(1);
  if (id) return { img: `https://i.ytimg.com/vi/${id}/hqdefault.jpg` };
  if (imageRe.test(u.pathname)) return { img: u.href };
  return {};
}

// Preview shows direct files and known embeds; other pages and expired links show their host.
function Preview({ url }: { url: string }) {
  const [broken, setBroken] = useState(false);
  const u = parse(url);
  const e = u ? embed(u) : {};
  const cls = "size-full object-cover";
  if (!broken && e.frame) return <iframe src={e.frame} loading="lazy" title="" class="pointer-events-none size-full border-0" />;
  if (!broken && e.img) return <img src={e.img} loading="lazy" alt="" class={cls} onError={() => setBroken(true)} />;
  if (!broken && u && videoRe.test(u.pathname)) return <video src={url} preload="metadata" muted class={cls} onError={() => setBroken(true)} />;
  return (
    <span class="muted flex size-full flex-col items-center justify-center gap-1 p-2 text-center text-xs">
      <span class="icon-[fa6-solid--link] size-4" />
      <span class="w-full truncate">{u?.hostname.replace(/^www\./, "") ?? url}</span>
    </span>
  );
}

export default function Media() {
  const summary = resource<GuildRow>(`/guilds/${guildId}/summary`).value.data;
  const kind = useSignal<Kind>("gif");
  const page = useSignal(1);
  const base = `/guilds/${guildId}/media?`;
  const path = `${base}kind=${kind.value}&page=${page.value}`;
  const r = resource<MediaPage>(path).value;
  // keep the previous page (and its pager) on screen while the next one loads
  const last = useRef(r);
  if (r.data || r.error) last.current = r;
  const { data, error } = last.current;
  const loading = r !== last.current;
  if (!summary || summary.state === "empty") return null;

  const remove = (url: string) => {
    if (!confirm(`Forget this ${kind.value}? Rolando won't post it again.\n\n${url}`)) return;
    api("DELETE", `/guilds/${guildId}/media`, { url }).then(() => {
      forget(base, path);
      resource(path, true);
      resource(`/guilds/${guildId}/summary`, true);
    }, (e: Error) => alert(e.message));
  };

  return (
    <section class="card">
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <h2 class="mb-0 mr-auto flex items-center gap-2"><span class="icon-[fa6-solid--photo-film] text-indigo-400" />Media</h2>
        <div class="flex gap-1 text-sm" role="tablist">
          {kinds.map(([k, label, icon, count]) => (
            <button
              role="tab"
              aria-selected={kind.value === k}
              class={`flex items-center gap-1.5 rounded px-2 py-1 ${kind.value === k ? "bg-indigo-500/20 text-indigo-300" : "hover:bg-neutral-800"}`}
              onClick={() => ((kind.value = k), (page.value = 1))}
            >
              <span class={icon} />{label}
              <span class="muted tabular-nums">{summary.analytics[count].toLocaleString()}</span>
            </button>
          ))}
        </div>
        {data && <Pager page={data.page} pages={data.pages} onPage={(p) => (page.value = p)} />}
      </div>
      <p class="muted mb-3 flex items-start gap-2 text-xs">
        <span class="icon-[fa6-solid--circle-info] mt-0.5 shrink-0" />
        Some media can't be previewed here, like posts on X or TikTok, Imgur pages and expired Discord attachments: they show as a link. Discord still embeds them when Rolando posts them.
      </p>
      {error && <p class="banner">{error}</p>}
      <ul class={`grid grid-cols-3 gap-2 sm:grid-cols-4 lg:grid-cols-6 ${loading ? "animate-pulse opacity-60" : ""}`}>
        {!data && !error && [0, 1, 2, 3, 4, 5].map(() => <li class="aspect-square animate-pulse rounded bg-neutral-800" />)}
        {data?.urls.map((u) => (
          <li class="group relative aspect-square overflow-hidden rounded bg-neutral-950/60" key={u}>
            <a href={u} target="_blank" rel="noreferrer" title={u} class="block size-full hover:no-underline">
              <Preview url={u} />
            </a>
            <button
              class="absolute top-1 right-1 grid size-7 place-items-center rounded bg-neutral-900/80 text-red-400 opacity-0 group-hover:opacity-100 hover:bg-red-600/60 hover:text-white focus-visible:opacity-100"
              onClick={() => remove(u)}
              title={`Forget this ${kind.value}`}
              aria-label={`Forget this ${kind.value}`}
            >
              <span class="icon-[fa6-solid--trash-can] size-3" />
            </button>
          </li>
        ))}
      </ul>
      {data && !data.urls.length && <p class="muted py-6 text-center text-sm">No {kind.value}s learned yet.</p>}
    </section>
  );
}
