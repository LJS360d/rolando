import { ChatInputCommandInteraction, PermissionsBitField } from 'discord.js';
import { Command } from '../../../fonzi2/events/decorators/command.interaction.dec';
import { Handler, HandlersType } from '../../../fonzi2/events/handlers/base.handler';
import { Buttons } from '../../../fonzi2/components/buttons';
import { ActionRow } from '../../../fonzi2/components/action-row';
import { env } from '../../../fonzi2/lib/env';
import { TRAIN_REPLY } from '../../static/text';
import { ChainsService } from '../../domain/services/chains.service';

export class CommandsHandler extends Handler {
	public readonly type = HandlersType.commandInteraction;
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
		const chain = await this.chainsService.getChain(
			interaction.guild.id,
			interaction.guild.name
		);
		void interaction.reply({ content: await chain.mediaStorage.getMedia('gif') });
	}

	@Command({ name: 'image', description: 'Returns a image from the ones it knows' })
	public async image(interaction: ChatInputCommandInteraction<'cached'>) {
		const chain = await this.chainsService.getChain(
			interaction.guild.id,
			interaction.guild.name
		);
		void interaction.reply({ content: await chain.mediaStorage.getMedia('image') });
	}

	@Command({ name: 'video', description: 'Returns a video from the ones it knows' })
	public async video(interaction: ChatInputCommandInteraction<'cached'>) {
		const chain = await this.chainsService.getChain(
			interaction.guild.id,
			interaction.guild.name
		);
		void interaction.reply({ content: await chain.mediaStorage.getMedia('video') });
	}

	private async checkAdmin(interaction: ChatInputCommandInteraction) {
		if (env.OWNER_IDS.includes(interaction.user.id)) {
			return true;
		}
		const perms = interaction.member?.permissions as Readonly<PermissionsBitField>;
		if (perms.has('Administrator')) {
			return true;
		}
		await interaction.reply({
			content: 'You are not authorized to use this command.',
			ephemeral: true,
		});
		return false;
	}
}
