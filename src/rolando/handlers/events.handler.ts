import { ClientEvent } from '../../fonzi2/events/decorators/client.event.dec';
import { Handler, HandlersType } from '../../fonzi2/events/handlers/base.handler';
import { Logger } from '../../fonzi2/lib/logger';
import { Fonzi2Server } from '../../fonzi2/server/server';
import { ApplicationCommandData, Guild } from 'discord.js';
import { ChainsService } from '../domain/services/chains.service';

export class EventsHandler extends Handler {
	public readonly type = HandlersType.clientEvent;

	constructor(
		private commands: ApplicationCommandData[],
		private chainsService: ChainsService
	) {
		super();
	}

	@ClientEvent('ready')
	async onReady() {
		Logger.info(`Logged in as ${this.client?.user?.tag}!`);

		try {
			const load = Logger.loading();
			load('Started refreshing application (/) commands.');
			await this.client?.application?.commands.set(this.commands);
			load('Successfully reloaded application (/) commands.', true);
			this.chainsService.loadChains();
			new Fonzi2Server(this.client!).start();
		} catch (err: any) {
			Logger.error(err);
		}
	}

	@ClientEvent('guildCreate')
	async onGuildCreate(guild: Guild) {
		Logger.info(`Joined guild ${guild.name}`);
		void guild.systemChannel?.send(`Hello ${guild.name}`);
		void this.chainsService.createChain(guild.id, guild.name);
	}

	@ClientEvent('guildDelete')
	async onGuildDelete(guild: Guild) {
		Logger.info(`Left guild ${guild.name}`);
		void this.chainsService.deleteChain(guild.id);
	}
}
