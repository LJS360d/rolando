import {
	ApplicationCommandOptionType,
	ChannelType,
	type ChatInputCommandInteraction,
	EmbedBuilder,
	type PermissionsBitField,
} from 'discord.js';

import {
	ActionRow,
	Buttons,
	Command,
	DiscordHandler,
	HandlerType,
	paginateInteraction,
} from 'fonzi2';
import { MarkovChainAnalyzer } from '../domain/model/chain.analyzer';
import type { ChainsService } from '../domain/services/chains.service';
import { env } from '../env';
import {
	ANALYTICS_DESCRIPTION,
	CHANNELS_DESCRIPTION,
	REPO_URL,
	TRAIN_REPLY,
} from '../static/text';
import { md } from '../utils/formatting.utils';
import { chunkArray } from '../utils/pagination.utils';
import { hasChannelAccess } from '../utils/permission.utils';
import { getRandom } from '../utils/random.utils';

export class CommandsHandler extends DiscordHandler {
	public readonly type = HandlerType.commandInteraction;
	constructor(private chainsService: ChainsService) {
		super();
	}

	@Command({
		name: 'train',
		description:
			'Fetches all available messages in the server to be used as training data',
	})
	public async train(interaction: ChatInputCommandInteraction) {
		if (!(await this.checkAdmin(interaction))) return;
		const confirm = Buttons.confirm('confirm-train');
		const cancel = Buttons.cancel('cancel-train');

		void interaction.reply({
			content: TRAIN_REPLY,
			components: [ActionRow.actionRowData(cancel, confirm)],
			ephemeral: true,
		});
	}

	@Command({ name: 'gif', description: 'Returns a gif from the ones it knows' })
	public async gif(interaction: ChatInputCommandInteraction<'cached'>) {
		const chain = await this.chainsService.getChain(interaction.guild.id);
		void interaction.reply({
			content:
				(await chain.mediaStorage.getMedia('gif')) ?? 'no valid gif found',
		});
	}

	@Command({
		name: 'image',
		description: 'Returns a image from the ones it knows',
	})
	public async image(interaction: ChatInputCommandInteraction<'cached'>) {
		const chain = await this.chainsService.getChain(interaction.guild.id);
		void interaction.reply({
			content:
				(await chain.mediaStorage.getMedia('image')) ?? 'no valid image found',
		});
	}

	@Command({
		name: 'video',
		description: 'Returns a video from the ones it knows',
	})
	public async video(interaction: ChatInputCommandInteraction<'cached'>) {
		const chain = await this.chainsService.getChain(interaction.guild.id);
		void interaction.reply({
			content:
				(await chain.mediaStorage.getMedia('video')) ?? 'no valid video found',
		});
	}

	@Command({
		name: 'analytics',
		description: 'Returns the analytics of the bot',
	})
	public async analytics(interaction: ChatInputCommandInteraction<'cached'>) {
		const chain = await this.chainsService.getChain(interaction.guild.id);
		const analytics = new MarkovChainAnalyzer(chain).getAnalytics();
		const embed = new EmbedBuilder()
			.setTitle('Analytics')
			.setDescription(ANALYTICS_DESCRIPTION('25 MB'))
			.setColor('Gold')
			.addFields(
				{
					name: 'Complexity Score',
					value: md.code(analytics.complexityScore),
					inline: true,
				},
				{
					name: 'Vocabulary',
					value: md.code(`${analytics.words} words`),
					inline: true,
				},
				{ name: '\t', value: '\t' },
				{ name: 'Gifs', value: md.code(analytics.gifs), inline: true },
				{ name: 'Videos', value: md.code(analytics.videos), inline: true },
				{ name: 'Images', value: md.code(analytics.images), inline: true },
				{ name: '\t', value: '\t' },
				{
					name: 'Processed messages',
					value: md.code(analytics.messages),
					inline: true,
				},
				{
					name: 'Size',
					value: md.code(`${analytics.size} / 25.00 MB`),
					inline: true,
				}
			)
			.setFooter({
				iconURL: this.client.user.displayAvatarURL(),
				text: `Version: ${env.VERSION}`,
			});
		void interaction.reply({
			embeds: [embed],
		});
	}

	@Command({
		name: 'channels',
		description: 'View which channels are being used by the bot',
	})
	public async channels(interaction: ChatInputCommandInteraction<'cached'>) {
		const guild = interaction.guild;
		const channels = guild.channels.cache.filter(
			(ch) =>
				![ChannelType.GuildVoice, ChannelType.GuildCategory].includes(ch.type)
		);
		const accessEmote = (hasAccess: boolean) =>
			hasAccess ? ':green_circle:' : ':red_circle:';
		const channelsPermissionMap = channels.map((ch) => ({
			name: ch.name,
			access: accessEmote(hasChannelAccess(this.client.user, ch)),
		}));
		const channelFields = channelsPermissionMap.map((cp) => ({
			name: ' ',
			value: `${cp.access} #${cp.name}`,
			inline: true,
		}));
		const embeds = chunkArray(channelFields, 15).map((channelFields) =>
			new EmbedBuilder()
				.setTitle('Available channels')
				.setDescription(CHANNELS_DESCRIPTION(':green_circle:', ':red_circle:'))
				.setColor('Gold')
				.addFields({ name: '\t', value: '\t' })
				.addFields(channelFields)
		);
		await paginateInteraction(interaction, embeds);
	}

	@Command({
		name: 'replyrate',
		description: 'check or set the reply rate',
		options: [
			{
				name: 'rate',
				description: 'the rate to set',
				type: ApplicationCommandOptionType.Integer,
				minValue: 0,
				required: false,
			},
		],
	})
	public async replyrate(interaction: ChatInputCommandInteraction<'cached'>) {
		const rate = interaction.options.getInteger('rate');
		const chain = await this.chainsService.getChain(interaction.guild.id);
		if (rate !== null) {
			const msg = 'You are not authorized to change the reply rate.';
			if (!(await this.checkAdmin(interaction, msg))) return;
			chain.replyRate = rate;
			await this.chainsService.updateChainProps(chain.id, { replyRate: rate });
			void interaction.reply({ content: `Set reply rate to \`${rate}\`` });
			return;
		}
		await interaction.reply({
			content: `Current rate is \`${chain.replyRate}\``,
		});
	}

	@Command({
		name: 'opinion',
		description: 'Get a reply with a specific word as the seed',
		options: [
			{
				name: 'about',
				description: 'The seed of the message',
				type: ApplicationCommandOptionType.String,
				required: true,
			},
		],
	})
	public async opinion(interaction: ChatInputCommandInteraction<'cached'>) {
		const about = interaction.options
			.getString('about', true)
			.split(' ')
			.at(-1) as string;
		const chain = await this.chainsService.getChain(interaction.guild.id);
		const msg = chain.generateText(about, getRandom(8, 40));
		void interaction.reply({ content: msg });
		return;
	}

	@Command({
		name: 'wipe',
		description: 'deletes the given argument `data` from the training data',
		options: [
			{
				name: 'data',
				description: 'The message or link you want to be erased from memory',
				type: ApplicationCommandOptionType.String,
				required: true,
			},
		],
	})
	public async wipe(interaction: ChatInputCommandInteraction<'cached'>) {
		const data = interaction.options.getString('data', true);
		const chain = await this.chainsService.getChain(interaction.guild.id);
		chain.delete(data);
		void interaction.reply({ content: `Deleted \`${data}\`` });
		return;
	}

	@Command({
		name: 'src',
		description: 'Provides the URL to the repository with bot source code.',
	})
	public async info(interaction: ChatInputCommandInteraction<'cached'>) {
		void interaction.reply({ content: REPO_URL });
	}

	private async checkAdmin(
		interaction: ChatInputCommandInteraction,
		msg?: string
	) {
		if (env.OWNER_IDS.includes(interaction.user.id)) {
			return true;
		}
		const perms = interaction.member
			?.permissions as Readonly<PermissionsBitField>;
		if (perms.has('Administrator')) {
			return true;
		}
		await interaction.reply({
			content: msg || 'You are not authorized to use this command.',
			ephemeral: true,
		});
		return false;
	}
}
