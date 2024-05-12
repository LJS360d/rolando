import type {
	Client,
	Collection,
	Guild,
	GuildTextBasedChannel,
	Message,
} from 'discord.js';
import { Logger } from 'fonzi2';
import { hasChannelAccess } from '../../utils/permission.utils';
import { containsURL } from '../../utils/url.utils';
import type { ChainsService } from './chains.service';

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
				.map((channel) =>
					this.fetchChannelMessages(channel as GuildTextBasedChannel)
				);
		const results = await Promise.all(fetchPromises);
		const messages = results.flat();
		Logger.info(
			`Fetched #green${messages.length}$ messages in guild: ${guild.name}`
		);
		return messages;
	}

	private async fetchChannelMessages(
		channel: GuildTextBasedChannel
	): Promise<string[]> {
		const load = Logger.loading(`Fetching messages in #${channel.name}...`);
		const messages: string[] = [];
		let lastMessageId: string | undefined;
		let errorCount = 0;

		while (messages.length < this.MSG_LIMIT) {
			try {
				const messageBatch = await this.getMessageBatch(channel, lastMessageId);
				if (messageBatch.length === 0) break;

				const messagesContent = messageBatch.map((msg) => msg.content);
				messages.push(...messagesContent);

				void this.chainService.updateChainState(
					channel.guildId,
					messagesContent
				);
				lastMessageId = messageBatch.at(-1)?.id;

				load.update(
					`Fetched #green${messages.length}$ messages in #${channel.name}`
				);
			} catch (error) {
				Logger.warn(
					`Message fetching error in ${channel.name} at #green${messages.length}$ messages, current error count: ${errorCount}`
				);
				if (++errorCount > this.MSG_FETCH_MAXERRORS) {
					Logger.warn(
						`Fetching error limit reached in ${channel.name} at #green${messages.length}$ messages, Error ${error}`
					);
					break;
				}
			}
		}
		load.success(
			`Fetched #green${messages.length}$ messages in #${channel.name}`
		);
		return messages;
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
