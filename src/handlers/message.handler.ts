import type { Message } from 'discord.js';
import type { MarkovChain } from '../domain/model/markov.chain';
import type { ChainsService } from '../domain/services/chains.service';
import { getRandom } from '../utils/random.utils';
import { Handler, HandlerType, MessageEvent } from 'fonzi2';
import { mentionsUser } from '../utils/permission.utils';

export class MessageHandler extends Handler {
	public readonly type = HandlerType.messageEvent;
	constructor(private chainsService: ChainsService) {
		super();
	}

	@MessageEvent('GuildText')
	async onGuildMessage(message: Message<true>) {
		const { author, guild, content } = message;
		// * does not react to messages sent by itself
		if (author.id === this.client?.user?.id) return;

		const guildId = guild.id;
		const chain = await this.chainsService.getChain(guildId);
		if (!chain) {
			await this.chainsService.createChain(guildId, guild.name);
			return;
		}
		if (content.length > 3) {
			// * learning from message content
			chain.updateState(content);
			this.chainsService.updateChainState(guildId, content);
		}

		const isMentioned = mentionsUser(message, this.client.user);
		if (isMentioned) {
			// * always reply if mentioned
			await message.channel.sendTyping();
			void message.reply(await this.getMessage(chain));
			return;
		}
		const randomMessage =
			chain.replyRate === 1 ||
			(chain.replyRate > 1 && getRandom(1, chain.replyRate) === 1);
		if (randomMessage) {
			await message.channel.sendTyping();
			void message.channel.send(await this.getMessage(chain));
			return;
		}
	}

	private async getMessage(chain: MarkovChain) {
		const random = getRandom(4, 25);
		// Adjusted probabilities
		const reply =
			random <= 21
				? // ? 84% chance for text
					chain.talk(random)
				: random <= 23
					? // ? 10% chance for gif
						await chain.mediaStorage.getMedia('gif')
					: random <= 24
						? // ? 5% chance for image
							await chain.mediaStorage.getMedia('image')
						: // ? 1% chance for video
							await chain.mediaStorage.getMedia('video');
		if (reply === null) {
			return chain.talk(random);
		}
		return reply;
	}
}
