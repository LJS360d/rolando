import { resource, type Bot } from "../lib/api";

// configured on the web container (INVITE_URL); falls back to the bot's own
const envInvite = () => document.querySelector<HTMLMetaElement>('meta[name="rolando:invite"]')?.content;

export function BotName() {
  const { data: bot, error } = resource<Bot>("/bot").value;
  if (bot) return <>{bot.name}</>;
  if (error) return <>Rolando</>;
  return <span class="inline-block h-8 w-40 animate-pulse rounded bg-neutral-800 align-middle" />;
}

export default function BotCard() {
  const { data: bot, error } = resource<Bot>("/bot").value;
  const invite = envInvite() || bot?.invite;
  const inviteBtn = invite && <a class="btn" href={invite}>Invite to your server</a>;
  if (error) return <><p class="banner">{error}</p>{inviteBtn}</>;
  if (!bot) return <div class="mx-auto my-4 size-40 animate-pulse rounded-full bg-neutral-800" />;
  return (
    <>
      <img class="mx-auto my-4 size-40 rounded-full" src={bot.avatar} alt={bot.name} />
      <p>Currently part of <b>{bot.guilds}</b> servers</p>
      <div class="mt-4 flex justify-center gap-3">
        {inviteBtn}
        {bot.support && <a class="btn bg-neutral-700 hover:bg-neutral-600" href={bot.support}>Support server</a>}
      </div>
    </>
  );
}
