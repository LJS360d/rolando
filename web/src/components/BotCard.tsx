import { resource, type Bot } from "../lib/api";

export default function BotCard() {
  const { data: bot, error } = resource<Bot>("/bot").value;
  if (error) return <p class="banner">{error}</p>;
  if (!bot) return <div class="mx-auto my-4 size-40 animate-pulse rounded-full bg-neutral-800" />;
  return (
    <>
      <img class="mx-auto my-4 size-40 rounded-full" src={bot.avatar} alt={bot.name} />
      <p>Currently part of <b>{bot.guilds}</b> servers</p>
      <div class="mt-4 flex justify-center gap-3">
        {bot.invite && <a class="btn" href={bot.invite}>Invite to your server</a>}
        {bot.support && <a class="btn bg-neutral-700 hover:bg-neutral-600" href={bot.support}>Support server</a>}
      </div>
    </>
  );
}
