import { formatNumber, formatTime, md } from '../utils/formatting.utils';

export const TRAIN_REPLY = `
  Are you sure you want to use **ALL SERVER MESSAGES** as training data for me?
  This will fetch data in all accessible channels and delete all previous training data for this server.
  If you wish to exclude specific channels, revoke my typing permissions in those channels.
`;

export const FETCH_CONFIRM_MSG = (id: string) => `
  <@${id}> Started Fetching messages.
  I will send a message when I'm done.
  Estimated Time: \`1 Minute per every 5000 Messages in the Server\`
  This might take a while..
`;

export const FETCH_DENY_MSG = (guild: string) => `
  This server: ${guild} has already performed this command.
`;

export const FETCH_COMPLETE_MSG = (id: string, amount: number, time: number) => `
  <@${id}> Finished Fetching messages.
  Messages fetched: \`${formatNumber(amount)}\`
  Time elapsed: \`${formatTime(time)}\`
`;

export const GUILD_CREATE_MSG = (name: string) => `
  Hello ${name},
  perform the command \`/train\` to use all the server's messages as training data
`;

export const ANALYTICS_DESCRIPTION = (maxSize: string) => `
${md.bold('Complexity Score')}: indicates how ${md.italic('smart')} the bot is.
A higher value means smarter
To increase this value you may want to type in long sentences,
with many different words,
thats the fastest way to make the bot learn.\n
${md.bold('Size')}: This server has a maximum storage size of ${md.bold(maxSize)}
`;

export const CHANNELS_DESCRIPTION = (hasAccess: string, noAccess: string) => `
channels the bot has access to are marked with: ${hasAccess}
while channels with no access are marked with: ${noAccess}

make a channel accessible by giving ${md.bold('ALL')} these permissions: 
${md.code('View Channel')} ${md.code('Send Messages')} ${md.code('Read Message History')} 
`;

export const REPO_URL = 'https://github.com/LJS360d/rolando2';
