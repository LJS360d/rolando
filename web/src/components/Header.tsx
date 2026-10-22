import { api, login, session } from "../lib/api";

const link = "flex items-center gap-2 rounded px-3 py-2 text-neutral-100 hover:bg-white/10 hover:no-underline";

export default function Header() {
  const { data } = session().value;
  const logout = () => api("POST", "/auth/logout").then(() => location.assign("/"));
  const color = data?.c ? `#${data.c.toString(16).padStart(6, "0")}` : undefined;
  return (
    <header class="sticky top-0 z-10 bg-neutral-900 shadow-lg">
      <nav class="mx-auto flex max-w-7xl items-center gap-1 px-4 py-2">
        <a href="/" class="mr-auto flex items-center gap-2 text-lg font-bold text-neutral-100 hover:no-underline">
          <img src="/favicon-32x32.png" class="size-7 rounded-full" alt="" /> Rolando
        </a>
        {data?.o && (
          <>
            <a class={link} href="/admin" title="Admin"><span class="icon-[fa6-solid--shield-halved]" /><span class="hidden sm:inline">Admin</span></a>
            <a class={link} href="/admin/broadcast" title="Broadcast"><span class="icon-[fa6-solid--bullhorn]" /><span class="hidden sm:inline">Broadcast</span></a>
          </>
        )}
        {data && <a class={link} href="/me" title="My servers"><span class="icon-[fa6-solid--server]" /><span class="hidden sm:inline">My servers</span></a>}
        {data === null && (
          <button class="btn flex items-center gap-2 bg-[#5865f2] hover:bg-[#4752c4]" onClick={login}>
            <span class="icon-[fa6-brands--discord]" /> Login
          </button>
        )}
        {data && (
          <div class="ml-2 flex items-center gap-2">
            <span class="flex items-center gap-2 rounded-full bg-neutral-700 py-1 pr-1 pl-1 text-neutral-100 md:pl-3" style={color && { backgroundColor: color }}>
              <span class="hidden font-semibold [text-shadow:0_1px_2px_rgb(0_0_0/.6)] md:inline">{data.n}</span>
              {data.a && <img class="size-8 rounded-full" src={data.a} alt={data.n} />}
            </span>
            <button class="btn flex items-center gap-2 bg-neutral-700 hover:bg-neutral-600" onClick={logout} title="Logout">
              <span class="icon-[fa6-solid--right-from-bracket]" /><span class="hidden sm:inline">Logout</span>
            </button>
          </div>
        )}
      </nav>
    </header>
  );
}
