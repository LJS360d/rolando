import { Client, Collection, Guild, GuildTextBasedChannel, Message } from 'discord.js';
import { Logger } from 'fonzi2';
import { hasChannelAccess } from '../../utils/permission.utils';
import { containsURL } from '../../utils/url.utils';
import { ChainsService } from './chains.service';

export class DataFetchService {
	private readonly MSG_LIMIT = 750000;
	private readonly MSG_FETCH_MAXERRORS = 5;
	constructor(
		private client: Client<true>,
		private chainService: ChainsService
	) {}

	async fetchAllGuildMessages(guild: Guild): Promise<string[]> {
		Logger.info(`Fetching messages in guild: ${guild.name}`);
		const fetchPromises: Promise<string[]>[] =
			// ? Get all guild channels
			Array.from(guild.channels.cache.values())
				// ? Filter to only text channels with read/write access
				.filter((channel) => hasChannelAccess(this.client.user, channel))
				// ? Start fetching in each channel
				.map((channel) => this.fetchChannelMessages(channel as GuildTextBasedChannel));
		const results = await Promise.all(fetchPromises);
		const messages = results.flat();
		Logger.info(`Fetched #green${messages.length}$ messages in guild: ${guild.name}`);
		return messages;
	}

	private async fetchChannelMessages(channel: GuildTextBasedChannel): Promise<string[]> {
		// biome-ignore lint/suspicious/noAsyncPromiseExecutor: Old code, works, but should be refactored
		return new Promise(async (resolve) => {
			const load = Logger.loading(`Fetching messages in #${channel.name}...`);
			const messages: string[] = [];
			let lastMessageId: string | undefined = undefined;
			let remaining = true;
			let firstFetch = true;
			let errorCount = 0;
			while (remaining && messages.length < this.MSG_LIMIT) {
				try {
					const messageBatch = await this.getMessageBatch(channel, lastMessageId);
					if (lastMessageId === undefined && !firstFetch) {
						remaining = false;
						continue;
					}
					lastMessageId = messageBatch.at(-1)?.id;
					if (firstFetch) firstFetch = false;
					const textMessages = messageBatch.map((msg) => msg.content);
					messages.push.apply(messages, textMessages);
					void this.chainService.updateChainState(channel.guildId, textMessages);
					load.update(`Fetched #green${messages.length}$ messages in #${channel.name}`);
				} catch (error) {
					errorCount++;
					Logger.warn(
						`Message fetching error in ${channel.name} at #green${messages.length}$ messages, current error count: ${errorCount}`
					);
					if (errorCount > this.MSG_FETCH_MAXERRORS) {
						load.fail(
							`Fetching error limit reached in ${channel.name} at #green${messages.length}$ messages, Error ${error}`
						);
						resolve(messages);
						return;
					}
				}
			}
			load.success(`Fetched #green${messages.length}$ messages in #${channel.name}`);
			resolve(messages);
		});
	}

	private async getMessageBatch(
		channel: GuildTextBasedChannel,
		lastMessageId?: string
	): Promise<Message<true>[]> {
		const messageBatch = (await channel.messages.fetch({
			limit: 100,
			before: lastMessageId,
		})) as Collection<string, Message<true>>;
		const cleanMessages = Array.from(messageBatch.values()).filter(
			(msg) => msg.content.split(' ').length > 1 || containsURL(msg.content)
		);
		return cleanMessages;
	}
}
