import { useEffect, useState } from "preact/hooks";
import { adminPage } from "../lib/admin";

const uptime = (s: number) => {
  const d = Math.floor(s / 86400), h = Math.floor(s / 3600) % 24, m = Math.floor(s / 60) % 60;
  return `${d ? `${d}d ` : ""}${d || h ? `${h}h ` : ""}${m}m ${s % 60}s`;
};

function Uptime({ started }: { started: number }) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);
  return <>{uptime(Math.max(0, Math.floor(now / 1000) - started))}</>;
}

export default function AdminStats() {
  const { data, error } = adminPage.value;
  if (error) return <p class="banner">{error}</p>;
  const nodeErrors = Object.entries(data?.errors ?? {});
  const tiles: [string, preact.ComponentChildren][] = [
    ["Uptime", data && <Uptime started={data.started} />],
    ["Guilds", data?.total],
    ["Chains loaded on this node", data?.loaded],
    ["Unreachable nodes", data && nodeErrors.length],
  ];
  return (
    <>
      <div class="mb-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
        {tiles.map(([label, v]) => (
          <div class="card mb-0">
            <div class="muted flex items-center gap-2 text-sm">{label}</div>
            <div class={`mt-1 text-2xl font-semibold tabular-nums ${v === undefined ? "h-8 w-20 animate-pulse rounded bg-neutral-800" : ""}`}>{v}</div>
          </div>
        ))}
      </div>
      {nodeErrors.map(([node, err]) => <p class="banner">Node {node} unreachable: {err}</p>)}
    </>
  );
}
