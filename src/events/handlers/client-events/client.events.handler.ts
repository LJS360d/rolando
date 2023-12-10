import { ApplicationCommandData, Guild } from 'discord.js';
import { Logger } from '../../../lib/logger';
import { Fonzi2Server } from '../../../server/server';
import { ClientEvent } from '../../decorators/client.event.dec';
import { Handler, HandlersType } from '../base.handler';

export class ClientEventsHandler extends Handler {
	public type = HandlersType.clientEvent;
	constructor(private commands: ApplicationCommandData[]) {
		super();
	}
	@ClientEvent('ready')
	async onReady() {
		// * Successful login
		Logger.info(`Logged in as ${this.client?.user?.tag}!`);

		try {
			const load = Logger.loading();
			load('Started refreshing application (/) commands.');
			await this.client?.application?.commands.set(this.commands);
			load('Successfully reloaded application (/) commands.', true);
			new Fonzi2Server(this.client!).start();
		} catch (err: any) {
			Logger.error(err);
		}
	}

	@ClientEvent('guildCreate')
	async onGuildCreate(guild: Guild) {
		Logger.info(`Joined guild ${guild.name}`);
		void guild.systemChannel?.send(`Hello ${guild.name}`);
	}

	@ClientEvent('guildDelete')
	async onGuildDelete(guild: Guild) {
		Logger.info(`Left guild ${guild.name}`);
	}
}
