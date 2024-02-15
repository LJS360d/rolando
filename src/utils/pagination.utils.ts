import {
	ChatInputCommandInteraction,
	EmbedBuilder,
	ActionRowBuilder,
	ButtonBuilder,
	ButtonStyle,
	MessageComponentInteraction,
} from 'discord.js';

export function chunkArray<T = unknown>(array: T[], chunkSize: number): T[][] {
	const chunks: T[][] = [];
	for (let i = 0; i < array.length; i += chunkSize) {
		chunks.push(array.slice(i, i + chunkSize));
	}
	return chunks;
}

export async function paginateInteraction(
	interaction: ChatInputCommandInteraction<'cached'>,
	embeds: EmbedBuilder[]
) {
	let page = 0;

	const updateButtons = (page: number, totalPages: number) => {
		return new ActionRowBuilder<ButtonBuilder>().addComponents(
			new ButtonBuilder()
				.setCustomId('first')
				.setLabel('⏮️First')
				.setStyle(ButtonStyle.Primary)
				.setDisabled(page === 0),
			new ButtonBuilder()
				.setCustomId('previous')
				.setLabel('⬅️Previous')
				.setStyle(ButtonStyle.Primary)
				.setDisabled(page === 0),
			new ButtonBuilder()
				.setCustomId('next')
				.setLabel('Next➡️')
				.setStyle(ButtonStyle.Primary)
				.setDisabled(page === totalPages - 1),
			new ButtonBuilder()
				.setCustomId('last')
				.setLabel('Last⏭️')
				.setStyle(ButtonStyle.Primary)
				.setDisabled(page === totalPages - 1)
		);
	};

	const totalPages = embeds.length;
	let row = updateButtons(page, totalPages);

	const addFooterToEmbed = (embed: EmbedBuilder, page: number, totalPages: number) => {
		return EmbedBuilder.from(embed).setFooter({ text: `Page ${page + 1} of ${totalPages}` });
	};

	await interaction.reply({
		embeds: [addFooterToEmbed(embeds[page], page, totalPages)],
		components: [row],
	});

	const filter = (i: MessageComponentInteraction) => i.user.id === interaction.user.id;
	if (!interaction.channel) return;
	const collector = interaction.channel.createMessageComponentCollector({ filter, time: 60000 });

	collector.on('collect', async (i: MessageComponentInteraction) => {
		switch (i.customId) {
			case 'first':
				page = 0;
				break;
			case 'previous':
				if (page > 0) page--;
				break;
			case 'next':
				if (page < totalPages - 1) page++;
				break;
			case 'last':
				page = totalPages - 1;
				break;
		}

		row = updateButtons(page, totalPages);
		await i.update({
			embeds: [addFooterToEmbed(embeds[page], page, totalPages)],
			components: [row],
		});
	});

	collector.on('end', () => {
		void interaction.editReply({ components: [] });
	});
}
