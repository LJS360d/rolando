import {
  APIActionRowComponent,
  APIMessageActionRowComponent,
  ActionRowBuilder,
  AnyComponentBuilder,
  JSONEncodable,
  RestOrArray
} from 'discord.js';

export class ActionRow {
	static actionRowData(...components: RestOrArray<AnyComponentBuilder>) {
		const actionRow = new ActionRowBuilder().addComponents(...components.flat());
		return actionRow as JSONEncodable<
			APIActionRowComponent<APIMessageActionRowComponent>
		>;
	}
}
