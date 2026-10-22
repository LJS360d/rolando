export default function Pager({ page, pages, onPage }: { page: number; pages: number; onPage: (p: number) => void }) {
  if (pages <= 1) return null;
  const btn = "link grid size-8 place-items-center rounded hover:bg-neutral-800 disabled:pointer-events-none disabled:opacity-30";
  const jump = (e: Event) => {
    const el = e.currentTarget as HTMLInputElement;
    const p = Math.min(pages, Math.max(1, Math.round(Number(el.value)) || page));
    el.value = String(p);
    if (p !== page) onPage(p);
  };
  return (
    <div class="flex items-center gap-1 text-sm">
      <button class={btn} disabled={page <= 1} onClick={() => onPage(1)} title="First page" aria-label="First page"><span class="icon-[fa6-solid--angles-left]" /></button>
      <button class={btn} disabled={page <= 1} onClick={() => onPage(page - 1)} title="Previous page" aria-label="Previous page"><span class="icon-[fa6-solid--chevron-left]" /></button>
      <span class="muted flex items-center gap-1 tabular-nums">
        <input
          key={page}
          class="input w-14 py-0.5 text-center"
          type="number"
          min={1}
          max={pages}
          value={page}
          aria-label="Page"
          onChange={jump}
          onKeyDown={(e) => e.key === "Enter" && jump(e)}
        />
        / {pages}
      </span>
      <button class={btn} disabled={page >= pages} onClick={() => onPage(page + 1)} title="Next page" aria-label="Next page"><span class="icon-[fa6-solid--chevron-right]" /></button>
      <button class={btn} disabled={page >= pages} onClick={() => onPage(pages)} title="Last page" aria-label="Last page"><span class="icon-[fa6-solid--angles-right]" /></button>
    </div>
  );
}
