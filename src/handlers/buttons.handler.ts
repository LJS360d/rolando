import { ButtonInteraction } from 'discord.js';
import { ChainsService } from '../domain/services/chains.service';
import { DataFetchService } from '../domain/services/data.fetch.service';
import { FETCH_COMPLETE_MSG, FETCH_CONFIRM_MSG } from '../static/text';
import { Button, Handler, HandlerType } from 'fonzi2';

export class ButtonsHandler extends Handler {
	public readonly type = HandlerType.buttonInteraction;

	constructor(private chainsService: ChainsService) {
		super();
	}

	@Button('confirm-train')
	async onConfirmTrain(interaction: ButtonInteraction<'cached'>) {
		void interaction.deferUpdate();
		await interaction.channel?.send({
			content: FETCH_CONFIRM_MSG(interaction.user.id),
		});
		const startTime = Date.now();
		const dataFetchService = new DataFetchService(this.client!);
		const messages = await dataFetchService.fetchAllGuildMessages(interaction.guild);
		await interaction.channel?.send({
			content: FETCH_COMPLETE_MSG(
				interaction.user.id,
				messages.length,
				Date.now() - startTime
			),
		});
		this.chainsService.deleteChain(interaction.guild.id);
		const chain = await this.chainsService.getChain(
			interaction.guild.id,
			interaction.guild.name
		);
		this.chainsService.updateChain(chain, messages);
	}

	@Button('cancel-train')
	async onCancelTrain(interaction: ButtonInteraction<'cached'>) {
		void interaction.reply({
			content: 'The fetching process was canceled.',
			ephemeral: true,
		});
	}
}
