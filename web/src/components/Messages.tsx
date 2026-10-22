import { useSignal } from "@preact/signals";
import { guildId, resource, type MessagesPage } from "../lib/api";
import Pager from "./Pager";

export default function Messages() {
  const page = useSignal(1);
  const { data, error } = resource<MessagesPage>(`/guilds/${guildId}/messages?page=${page.value}`).value;
  return (
    <section class="card">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="mb-0 flex items-center gap-2"><span class="icon-[fa6-solid--message] text-indigo-400" />Learned messages</h2>
        {data && <Pager page={data.page} pages={data.pages} onPage={(p) => (page.value = p)} />}
      </div>
      {error && <p class="banner">{error}</p>}
      <ul class="mt-3 max-h-[32rem] overflow-y-auto rounded-lg bg-neutral-950/60 text-sm">
        {data?.messages.map((m) => <li class="border-b border-neutral-800/70 px-3 py-1.5 break-words whitespace-pre-wrap last:border-0">{m}</li>)}
        {data && !data.messages.length && <li class="muted px-3 py-6 text-center">Nothing learned yet.</li>}
      </ul>
    </section>
  );
}
