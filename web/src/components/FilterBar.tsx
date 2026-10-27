import { useEffect, useRef, useState } from "preact/hooks";

export type FieldType = "number" | "bool" | "enum" | "string";
// scale converts the shown value to the stored one (e.g. MB shown, bytes stored).
export type Field = { key: string; label: string; type: FieldType; values?: string[]; scale?: number; unit?: string };
export type Filter = { key: string; op: string; value: string };

const ops: Record<FieldType, [string, string][]> = {
  number: [["gt", ">"], ["gte", "≥"], ["lt", "<"], ["lte", "≤"], ["eq", "="], ["ne", "≠"]],
  bool: [["eq", "is"]],
  enum: [["in", "is any of"], ["nin", "is none of"]],
  string: [["contains", "contains"], ["eq", "is"], ["ne", "is not"]],
};

const shown = (f: Field | undefined, v: string) => {
  if (f?.type === "number" && f.scale && v !== "") return String(Number(v) / f.scale);
  if (f?.type === "bool") return v === "true" ? "yes" : "no";
  if (f?.type === "enum") return v.split(",").join(", ");
  return v;
};

const byKey = (fields: Field[], key: string) => fields.find((f) => f.key === key);

const blank = (f: Field): Filter => ({ key: f.key, op: ops[f.type][0][0], value: f.type === "bool" ? "true" : "" });

export default function FilterBar({ fields, filters, onChange }: { fields: Field[]; filters: Filter[]; onChange: (fs: Filter[]) => void }) {
  const [editing, setEditing] = useState<number | null>(null);
  const save = (f: Filter) => {
    onChange(editing === -1 ? [...filters, f] : filters.map((x, i) => (i === editing ? f : x)));
    setEditing(null);
  };
  return (
    <div class="relative flex flex-wrap items-center gap-2">
      {filters.map((f, i) => {
        const fd = byKey(fields, f.key);
        const op = fd && ops[fd.type].find(([k]) => k === f.op)?.[1];
        return (
          <span class={`flex items-center rounded-full border text-sm ${editing === i ? "border-indigo-400 bg-indigo-500/20" : "border-indigo-500/40 bg-indigo-500/10"}`}>
            <button class="py-0.5 pr-1 pl-3 hover:text-white" onClick={() => setEditing(i)}>
              <span class="muted">{fd?.label ?? f.key}</span> {op ?? f.op} <span class="font-medium">{shown(fd, f.value)}{fd?.unit && ` ${fd.unit}`}</span>
            </button>
            <button class="grid size-6 place-items-center rounded-full hover:bg-indigo-500/30" onClick={() => onChange(filters.filter((_, j) => j !== i))} aria-label="Remove filter">
              <span class="icon-[fa6-solid--xmark] size-3" />
            </button>
          </span>
        );
      })}
      <button class="link flex items-center gap-1 text-sm" onClick={() => setEditing(-1)}><span class="icon-[fa6-solid--plus]" />Filter</button>
      {filters.length > 1 && <button class="muted text-sm hover:text-neutral-200" onClick={() => onChange([])}>Clear all</button>}
      {editing !== null && (
        <Editor key={editing} fields={fields} initial={editing === -1 ? blank(fields[0]) : filters[editing]} onSave={save} onClose={() => setEditing(null)} />
      )}
    </div>
  );
}

function Editor({ fields, initial, onSave, onClose }: { fields: Field[]; initial: Filter; onSave: (f: Filter) => void; onClose: () => void }) {
  // number values are edited as shown (unscaled) and scaled back on save
  const [f, setF] = useState(() => {
    const fd = byKey(fields, initial.key);
    return fd?.type === "number" ? { ...initial, value: shown(fd, initial.value) } : initial;
  });
  const ref = useRef<HTMLFormElement>(null);
  const fd = byKey(fields, f.key) ?? fields[0];
  useEffect(() => {
    const out = (e: MouseEvent) => ref.current && !ref.current.contains(e.target as Node) && onClose();
    const esc = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("mousedown", out);
    document.addEventListener("keydown", esc);
    return () => (document.removeEventListener("mousedown", out), document.removeEventListener("keydown", esc));
  }, []);
  const picked = f.value ? f.value.split(",") : [];
  const toggle = (v: string) => setF({ ...f, value: (picked.includes(v) ? picked.filter((x) => x !== v) : [...picked, v]).join(",") });
  const submit = (e: SubmitEvent) => {
    e.preventDefault();
    if (f.value !== "") onSave(fd.type === "number" && fd.scale ? { ...f, value: String(Number(f.value) * fd.scale) } : f);
  };

  return (
    <form ref={ref} class="absolute top-full left-0 z-10 mt-2 flex w-72 flex-col gap-2 rounded border border-neutral-700 bg-neutral-900 p-3 shadow-xl" onSubmit={submit}>
      <select class="input" value={f.key} onChange={(e) => setF(blank(byKey(fields, e.currentTarget.value)!))}>
        {fields.map((x) => <option value={x.key}>{x.label}</option>)}
      </select>
      {ops[fd.type].length > 1 && (
        <select class="input" value={f.op} onChange={(e) => setF({ ...f, op: e.currentTarget.value })}>
          {ops[fd.type].map(([k, label]) => <option value={k}>{label}</option>)}
        </select>
      )}
      {fd.type === "number" && (
        <label class="flex items-center gap-2">
          <input
            class="input min-w-0 flex-1"
            type="number"
            step="any"
            required
            autoFocus
            value={f.value}
            onInput={(e) => setF({ ...f, value: e.currentTarget.value })}
          />
          {fd.unit && <span class="muted text-sm">{fd.unit}</span>}
        </label>
      )}
      {fd.type === "string" && <input class="input" required autoFocus value={f.value} onInput={(e) => setF({ ...f, value: e.currentTarget.value })} />}
      {fd.type === "bool" && (
        <select class="input" value={f.value} onChange={(e) => setF({ ...f, value: e.currentTarget.value })}>
          <option value="true">yes</option>
          <option value="false">no</option>
        </select>
      )}
      {fd.type === "enum" &&
        fd.values!.map((v) => (
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={picked.includes(v)} onChange={() => toggle(v)} />{v}
          </label>
        ))}
      <div class="flex justify-end gap-2">
        <button type="button" class="btn bg-neutral-700 hover:bg-neutral-600" onClick={onClose}>Cancel</button>
        <button class="btn disabled:opacity-50" disabled={f.value === ""}>Apply</button>
      </div>
    </form>
  );
}
