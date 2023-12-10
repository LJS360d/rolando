import { ButtonBuilder, ButtonStyle } from 'discord.js';

export class Buttons {
	static confirm(id: string, label?: string) {
		return new ButtonBuilder()
			.setCustomId(id)
			.setLabel(label || 'Confirm')
			.setStyle(ButtonStyle.Success);
	}

	static cancel(id: string, label?: string) {
		return new ButtonBuilder()
			.setCustomId(id)
			.setLabel(label || 'Cancel')
			.setStyle(ButtonStyle.Secondary);
	}

	static delete(id: string, label?: string) {
		return new ButtonBuilder()
			.setCustomId(id)
			.setLabel(label || 'Delete')
			.setStyle(ButtonStyle.Danger);
	}
}
