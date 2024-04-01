import type { ApplicationCommandData, Guild } from 'discord.js';
import { ClientEvent, Handler, HandlerType, Logger } from 'fonzi2';
import type { ChainsService } from '../domain/services/chains.service';
import { RolandoServer } from '../server/rolando.server';
import { GUILD_CREATE_MSG } from '../static/text';

export class EventsHandler extends Handler {
	public readonly type = HandlerType.clientEvent;

	constructor(
		private commands: ApplicationCommandData[],
		private chainsService: ChainsService
	) {
		super();
	}

	@ClientEvent('ready')
	async onReady() {
		Logger.info(`Logged in as ${this.client?.user?.tag}!`);

		const load = Logger.loading('Started refreshing application (/) commands.');
		try {
			await this.client?.application?.commands.set(this.commands);
			load.success('Reloaded application (/) commands.');
			await this.chainsService.loadChains();
			new RolandoServer(this.client, this.chainsService).start();
		} catch (err: any) {
			load.fail('Failed to refresh application (/) commands.');
			Logger.error(err);
		}
	}

	@ClientEvent('guildUpdate')
	async onGuildUpdate(oldGuild: Guild, newGuild: Guild) {
		Logger.info(`Guild ${oldGuild.name} updated`);
		void this.chainsService.updateChainProps(oldGuild.id, {
			name: newGuild.name,
		});
	}

	@ClientEvent('guildCreate')
	async onGuildCreate(guild: Guild) {
		Logger.info(`Joined guild ${guild.name}`);
		void guild.systemChannel?.send(GUILD_CREATE_MSG(guild.name));
		void this.chainsService.createChain(guild.id, guild.name);
	}

	@ClientEvent('guildDelete')
	async onGuildDelete(guild: Guild) {
		Logger.info(`Left guild ${guild.name}`);
		void this.chainsService.deleteChain(guild.id);
	}
}
