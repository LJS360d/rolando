import { useSignal } from "@preact/signals";
import { useRef } from "preact/hooks";
import { api, forget, guildId, resource, type MessagesPage } from "../lib/api";
import Pager from "./Pager";

const tokenRe = /(<(?:@!?|@&|#)\d+>|<a?:\w+:\d+>|https?:\/\/\S+)/g;
const imageRe = /^[^?#]+\.(?:png|jpe?g|gif|webp)(?:[?#]|$)/i;
const row = "group flex items-start gap-2 px-3 pb-1.5";
const headRow = `${row} border-t border-neutral-800/70 pt-1.5 first:border-0`;
const timeFmt = new Intl.DateTimeFormat("en-GB", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });

function render(text: string, mentions: Record<string, string>) {
  return text.split(tokenRe).map((part, i) => {
    if (i % 2 === 0) return part;
    if (part.startsWith("http")) {
      return imageRe.test(part) ? (
        <a
          href={part}
          target="_blank"
          rel="noreferrer"
          class="my-1 block w-fit"
        >
          <img
            src={part}
            loading="lazy"
            alt={part}
            class="max-h-60 max-w-xs rounded"
          />
        </a>
      ) : (
        <a
          href={part}
          target="_blank"
          rel="noreferrer"
          class="text-indigo-400 hover:underline"
        >
          {part}
        </a>
      );
    }
    const emoji = part.match(/^<(a?):(\w+):(\d+)>$/);
    if (emoji) {
      return (
        <>
          <img
            src={`https://cdn.discordapp.com/emojis/${emoji[3]}.${emoji[1] ? "gif" : "webp"}`}
            alt={`:${emoji[2]}:`}
            title={`:${emoji[2]}:`}
            class="inline size-5 align-text-bottom"
          />
        </>
      );
    }
    return (
      <span class="rounded bg-indigo-500/20 px-0.5 text-indigo-300">
        {mentions[part] ?? (part[1] === "#" ? "#unknown" : "@unknown")}
      </span>
    );
  });
}

export default function Messages() {
  // newest first; next/prev go by id cursor (cheap keyset query); jumps go by page number
  const query = useSignal("page=1");
  const search = useSignal("");
  const base = `/guilds/${guildId}/messages?`;
  const path = `${base}${query.value}${search.value ? `&q=${encodeURIComponent(search.value)}` : ""}`;
  const r = resource<MessagesPage>(path).value;
  // keep the previous page (and its pager) on screen while the next one loads
  const last = useRef(r);
  if (r.data || r.error) last.current = r;
  const { data, error } = last.current;
  const loading = r !== last.current;
  const mentions = data?.mentions ?? {};
  let prevMin = -1;
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const onSearch = (v: string) => {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => ((search.value = v.trim()), (query.value = "page=1")), 300);
  };
  const remove = (m: string) => {
    if (!confirm(`Forget this message?\n\n${m.slice(0, 300)}`)) return;
    api("DELETE", `/guilds/${guildId}/messages`, { content: m }).then(() => {
      forget(base, path);
      resource(path, true);
      resource(`/guilds/${guildId}/summary`, true);
    }, (e: Error) => alert(e.message));
  };
  return (
    <section class="card">
      <div class="flex flex-wrap items-center gap-2">
        <h2 class="mb-0 mr-auto flex items-center gap-2">
          <span class="icon-[fa6-solid--message] text-indigo-400" />
          Learned messages
        </h2>
        <label class="relative">
          <span class="icon-[fa6-solid--magnifying-glass] muted absolute top-1/2 left-2.5 -translate-y-1/2" />
          <input class="input w-48 pl-8" placeholder="Search" onInput={(e) => onSearch(e.currentTarget.value)} />
        </label>
        {data && (
          <Pager
            page={data.page}
            pages={data.pages}
            onPage={(p) =>
              (query.value =
                p === data.page + 1 && data.last
                  ? `page=${p}&before=${data.last}`
                  : p === data.page - 1 && data.first
                    ? `page=${p}&after=${data.first}`
                    : `page=${p}`)
            }
          />
        )}
      </div>
      {error && <p class="banner">{error}</p>}
      <ul class={`mt-3 max-h-128 overflow-y-auto rounded-lg bg-neutral-950/60 text-sm ${loading ? "animate-pulse opacity-60" : ""}`}>
        {data?.messages.map((m, i) => {
          const t = data.times[i];
          const min = (t / 60) | 0;
          const head = min !== prevMin;
          prevMin = min;
          return (
            <li class={head ? headRow : row}>
              <div class="min-w-0 flex-1 wrap-break-word whitespace-pre-wrap">{render(m, mentions)}</div>
              {head && <time class="muted shrink-0 text-xs leading-5">{timeFmt.format(t * 1000)}</time>}
              <button
                class="grid size-6 shrink-0 place-items-center rounded text-red-400 opacity-0 group-hover:opacity-100 hover:bg-red-600/30 focus-visible:opacity-100"
                onClick={() => remove(m)}
                title="Forget this message"
                aria-label="Forget this message"
              >
                <span class="icon-[fa6-solid--trash-can] size-3" />
              </button>
            </li>
          );
        })}
        {data && !data.messages.length && (
          <li class="muted px-3 py-6 text-center">{search.value ? "No messages match." : "Nothing learned yet."}</li>
        )}
      </ul>
    </section>
  );
}
