import { formatTime } from '../utils/formatting.utils';

export const TRAIN_REPLY = `
  Are you sure you want to use **ALL SERVER MESSAGES** as training data for me?
  This will fetch data in all accessible channels.
  If you wish to exclude specific channels, revoke my typing permissions in those channels.
`;

export const FETCH_CONFIRM_MSG = (id: string) => `
  <@${id}> Started Fetching messages.
  I will send a message when I'm done.
  Estimated Time: \`1 Minute per every 5000 Messages in the Server\`
  This might take a while..
`;

export const FETCH_COMPLETE_MSG = (id: string, amount: number, time: number) => `
  <@${id}> Finished Fetching messages.
  Messages fetched: \`${amount}\`
  Time elapsed: \`${formatTime(time)}\`
`;
