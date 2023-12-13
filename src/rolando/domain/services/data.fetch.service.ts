import {
	Client,
	Guild,
	GuildBasedChannel,
	GuildTextBasedChannel,
	Message,
	PermissionFlagsBits,
	PermissionsBitField,
} from 'discord.js';
import { Logger } from '../../../fonzi2/lib/logger';
import { containsURL } from '../../utils/url.utils';

export class DataFetchService {
	private readonly MSG_LIMIT = 500000;
	constructor(private client: Client) {}

	async fetchAllGuildMessages(guild: Guild): Promise<string[]> {
		const load = Logger.loading();
		const fetchPromises: Promise<string[]>[] = [];
		load(`Fetching messages in guild: ${guild.name}`);
		guild.channels.cache.forEach((channel) => {
			const channelPerms = channel.permissionsFor(this.client.user!)!;
			const channelAccess = this.hasChannelAccess(channelPerms, channel);
			if (channel.isTextBased() && channelAccess) {
				fetchPromises.push(this.fetchChannelMessages(channel));
			}
		});
		const results = await Promise.all(fetchPromises);
		const messages = results.flat();
		load(`Fetched ${messages.length} messages in guild: ${guild.name}`);
		return messages;
	}

	async fetchChannelMessages(channel: GuildTextBasedChannel): Promise<string[]> {
		return new Promise(async (resolve, reject) => {
			try {
				const messages: string[] = [];
				let lastMessageID: string | undefined = undefined;
				let remaining = true;
				let firstFetch = true;
				while (remaining && messages.length < this.MSG_LIMIT) {
					// Fetch a batch of messages
					const messageBatch = await channel.messages.fetch({
						limit: 100,
						before: lastMessageID,
					});

					if (lastMessageID === undefined && !firstFetch) {
						// No more messages remaining
						remaining = false;
						continue;
					}

					messageBatch.forEach((msg: Message) => {
						if (msg.content && msg.author !== this.client.user) {
							const message: string = msg.content;
							if (containsURL(message) || message.split(' ').length > 1) {
								messages.push(message);
							}
						}
					});

					// Update the last message ID for the next batch
					lastMessageID = messageBatch.at(-1)?.id;
					if (firstFetch) {
						firstFetch = false;
					}
				}
				resolve(messages);
			} catch (error) {
				Logger.error(`Error fetching messages in ${channel.name}`);
				reject([]);
			}
		});
	}

	private hasChannelAccess(
		perms: Readonly<PermissionsBitField>,
		channel: GuildBasedChannel
	): boolean {
		const canReadChannel = perms.has(PermissionFlagsBits.ReadMessageHistory);
		const canAccessChannel = perms.has(PermissionFlagsBits.SendMessages);
		const canViewChannel = perms.has(PermissionFlagsBits.ViewChannel);
		return (
			channel.isTextBased() &&
			!channel.isVoiceBased() &&
			canReadChannel &&
			canAccessChannel &&
			canViewChannel
		);
	}
}
