import { ButtonInteraction } from 'discord.js';
import { Button } from '../../fonzi2/events/decorators/button.interaction.dec';
import { Handler, HandlersType } from '../../fonzi2/events/handlers/base.handler';
import { ChainsService } from '../domain/services/chains.service';
import { DataFetchService } from '../domain/services/data.fetch.service';
import { FETCH_COMPLETE_MSG, FETCH_CONFIRM_MSG } from '../static/text';

export class ButtonsHandler extends Handler {
	public readonly type = HandlersType.buttonInteraction;

	constructor(private chainsService: ChainsService) {
		super();
	}

	@Button('confirm-train')
	async onConfirmTrain(interaction: ButtonInteraction<'cached'>) {
		await interaction.channel?.send({
			content: FETCH_CONFIRM_MSG(interaction.user.id),
		});
		const startTime = Date.now();
		const dataFetchService = new DataFetchService(this.client!);
		const messages = await dataFetchService.fetchAllGuildMessages(interaction.guild);
		const chain = await this.chainsService.getChain(
			interaction.guild.id,
			interaction.guild.name
		);
		await interaction.channel?.send({
			content: FETCH_COMPLETE_MSG(
				interaction.user.id,
				messages.length,
				Date.now() - startTime
			),
		});
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
