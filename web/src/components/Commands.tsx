import autoAnimate from "@formkit/auto-animate";
import { useEffect, useRef, useState } from "preact/hooks";
import { resource, type Bot } from "../lib/api";

export default function Commands() {
  const bot = resource<Bot>("/bot").value.data;
  const [order, setOrder] = useState<Bot["commands"]>();
  const list = useRef<HTMLTableSectionElement>(null);
  useEffect(() => { if (list.current) autoAnimate(list.current); }, []);

  const commands = order ?? bot?.commands ?? [];
  const shuffle = () => {
    const c = [...commands];
    for (let i = c.length - 1; i > 0; i--) {
      const j = (Math.random() * (i + 1)) | 0;
      [c[i], c[j]] = [c[j], c[i]];
    }
    setOrder(c);
  };

  return (
    <>
      <div class="flex items-center justify-between">
        <h2>Commands</h2>
        <button class="text-amber-300 p-1 cursor-pointer hover:text-white" onClick={shuffle} title="Shuffle the commands, why not?" aria-label="Shuffle the commands, why not?">
          <span class="icon-[fa6-solid--shuffle] size-5" />
        </button>
      </div>
      <table class="w-full">
        <tbody ref={list}>
        {commands.map((c) => (
          <tr key={c.name}>
            <td class="whitespace-nowrap"><code>/{c.name}</code></td>
            <td>{c.description}</td>
          </tr>
        ))}
        </tbody>
      </table>
    </>
  );
}
