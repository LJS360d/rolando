import { useSignal } from "@preact/signals";
import { useRef } from "preact/hooks";
import { guildId, resource, type MessagesPage } from "../lib/api";
import Pager from "./Pager";

const tokenRe = /(<(?:@!?|@&|#)\d+>|<a?:\w+:\d+>|https?:\/\/\S+)/g;
const imageRe = /^[^?#]+\.(?:png|jpe?g|gif|webp)(?:[?#]|$)/i;

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
  // next/prev go by id cursor (cheap keyset query); jumps go by page number
  const query = useSignal("page=1");
  const r = resource<MessagesPage>(
    `/guilds/${guildId}/messages?${query.value}`,
  ).value;
  // keep the previous page (and its pager) on screen while the next one loads
  const last = useRef(r);
  if (r.data || r.error) last.current = r;
  const { data, error } = last.current;
  const loading = r !== last.current;
  return (
    <section class="card">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="mb-0 flex items-center gap-2">
          <span class="icon-[fa6-solid--message] text-indigo-400" />
          Learned messages
        </h2>
        {data && (
          <Pager
            page={data.page}
            pages={data.pages}
            onPage={(p) =>
              (query.value =
                p === data.page + 1 && data.last
                  ? `page=${p}&after=${data.last}`
                  : p === data.page - 1 && data.first
                    ? `page=${p}&before=${data.first}`
                    : `page=${p}`)
            }
          />
        )}
      </div>
      {error && <p class="banner">{error}</p>}
      <ul class={`mt-3 max-h-128 overflow-y-auto rounded-lg bg-neutral-950/60 text-sm ${loading ? "animate-pulse opacity-60" : ""}`}>
        {data?.messages.map((m) => (
          <li class="border-b border-neutral-800/70 px-3 py-1.5 wrap-break-word whitespace-pre-wrap last:border-0">
            {render(m, data.mentions ?? {})}
          </li>
        ))}
        {data && !data.messages.length && (
          <li class="muted px-3 py-6 text-center">Nothing learned yet.</li>
        )}
      </ul>
    </section>
  );
}
