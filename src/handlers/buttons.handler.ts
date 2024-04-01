import type { ButtonInteraction } from 'discord.js';
import type { ChainsService } from '../domain/services/chains.service';
import { DataFetchService } from '../domain/services/data.fetch.service';
import {
	FETCH_COMPLETE_MSG,
	FETCH_CONFIRM_MSG,
	FETCH_DENY_MSG,
} from '../static/text';
import { Button, Handler, HandlerType } from 'fonzi2';

export class ButtonsHandler extends Handler {
	public readonly type = HandlerType.buttonInteraction;

	constructor(private chainsService: ChainsService) {
		super();
	}

	@Button('confirm-train')
	async onConfirmTrain(interaction: ButtonInteraction<'cached'>) {
		void interaction.deferUpdate();
		if (
			(await this.chainsService.getChainDocument(interaction.guildId))?.trained
		) {
			await interaction.channel?.send({
				content: FETCH_DENY_MSG(interaction.guild.name),
			});
			return;
		}
		await interaction.channel?.send({
			content: FETCH_CONFIRM_MSG(interaction.user.id),
		});
		const startTime = Date.now();
		const dataFetchService = new DataFetchService(
			this.client,
			this.chainsService
		);
		this.chainsService.updateChainProps(interaction.guild.id, {
			trained: true,
		});
		const messages = await dataFetchService.fetchAllGuildMessages(
			interaction.guild
		);
		await interaction.channel?.send({
			content: FETCH_COMPLETE_MSG(
				interaction.user.id,
				messages.length,
				Date.now() - startTime
			),
		});
		// ? chain trained during fetch process
		// this.chainsService.updateChainState(interaction.guild.id, messages);
	}

	@Button('cancel-train')
	async onCancelTrain(interaction: ButtonInteraction<'cached'>) {
		void interaction.reply({
			content: 'The fetching process was canceled.',
			ephemeral: true,
		});
	}
}
