import { Message } from 'discord.js';
import { Handler, HandlersType } from '../../fonzi2/events/handlers/base.handler';
import { Logger } from '../../fonzi2/lib/logger';
import { MessageEvent } from '../../fonzi2/events/decorators/message.dec';
import { ChainsService } from '../domain/services/chains.service';
import { getRandom } from '../utils/random.utils';
import { MarkovChain } from '../domain/model/markov.chain';

export class MessageHandler extends Handler {
	public readonly type = HandlersType.messageEvent;
	constructor(private chainsService: ChainsService) {
		super();
	}

	@MessageEvent('GuildText')
	async onGuildMessage(message: Message<true>) {
		const { author, guild, content } = message;
    if (author === this.client?.user) return;
		const guildId = guild.id;
		const chain = await this.chainsService.getChain(guildId);
		if (!chain) {
			await this.chainsService.createChain(guildId);
			return;
		}
		// * Learning from message
		chain.updateState(content);
		this.chainsService.updateChain(chain);

		const mention = content.includes(`<@${this.client?.user?.id}>`);
		if (mention) {
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
		return reply;
	}
}
