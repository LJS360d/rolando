import {
	Client,
	Guild,
	GuildBasedChannel,
	GuildTextBasedChannel,
	Message,
	PermissionFlagsBits,
} from 'discord.js';
import { Logger } from 'fonzi2';
import { containsURL } from '../../utils/url.utils';

export class DataFetchService {
	private readonly MSG_LIMIT = 500000;
	private readonly MSG_FETCH_MAXERRORS = 5;
	constructor(private client: Client) {}

	async fetchAllGuildMessages(guild: Guild): Promise<string[]> {
		Logger.info(`Fetching messages in guild: ${guild.name}`);
		const fetchPromises: Promise<string[]>[] = [];
		Array.from(guild.channels.cache.values())
			.filter((channel) => this.hasChannelAccess(channel))
			.forEach((channel: GuildTextBasedChannel) => {
				fetchPromises.push(this.fetchChannelMessages(channel));
			});
		const results = await Promise.all(fetchPromises);
		const messages = results.flat();
		Logger.info(`Fetched #green${messages.length}$ messages in guild: ${guild.name}`);
		return messages;
	}

	async fetchChannelMessages(channel: GuildTextBasedChannel): Promise<string[]> {
		return new Promise(async (resolve) => {
			const load = Logger.loading(`Fetching messages in #${channel.name}...`);
			const messages: string[] = [];
			let lastMessageID: string | undefined = undefined;
			let remaining = true;
			let firstFetch = true;
			let errorCount = 0;
			while (remaining && messages.length < this.MSG_LIMIT) {
				try {
					const messageBatch = await channel.messages.fetch({
						limit: 100,
						before: lastMessageID,
					});

					if (lastMessageID === undefined && !firstFetch) {
						remaining = false;
						continue;
					}

					messageBatch.forEach((msg: Message) => {
						if (msg.content && msg.author !== this.client.user) {
							const message: string = msg.content;
							if (containsURL(message) || message.split(' ').length > 1) {
								messages.push(message);
								// TODO immediatly append messages to text storage
							}
						}
					});
					load.update(`Fetched #green${messages.length}$ messages in #${channel.name}`);

					lastMessageID = messageBatch.at(-1)?.id;
					if (firstFetch) {
						firstFetch = false;
					}
				} catch (error) {
					Logger.warn(
						`Message fetching error in ${channel.name} at #green${messages.length}$ messages, current error count: ${errorCount}`
					);
					if (errorCount > this.MSG_FETCH_MAXERRORS) {
						load.fail(
							`Fetching error limit reached in ${channel.name} at #green${messages.length}$ messages, Error ${error}`
						);
						resolve(messages);
					}
				}
			}
			load.success(`Fetched #green${messages.length}$ messages in #${channel.name}`);
			resolve(messages);
		});
	}

	private hasChannelAccess(channel: GuildBasedChannel): boolean {
		const perms = channel.permissionsFor(this.client.user!)!;
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
