import { ChannelType, Client, TextChannel } from 'discord.js';
import express, { Request, Response } from 'express';
import { Fonzi2Server } from 'fonzi2';
import { resolve } from 'path';
import { MarkovChainAnalyzer } from '../domain/model/chain.analyzer';
import { ChainsService } from '../domain/services/chains.service';
import { env } from '../env';
import { render } from './render';

export class RolandoServer extends Fonzi2Server {
	constructor(
		client: Client<true>,
		private chainsService: ChainsService
	) {
		super(client);
		this.app.use(express.static(resolve('public')));
		this.app.set('views', [this.app.get('views'), resolve('views')]);
	}

	override async start() {
		this.app.get('/chain', this.guildChain.bind(this));
		this.app.get('/invite', this.getGuildInvite.bind(this));
		super.start();
	}

	override async dashboard(req: Request, res: Response) {
		const userInfo = this.getSessionUserInfo(req);
		if (!userInfo) {
			res.redirect('/unauthorized');
			return;
		}

		const props = {
			client: this.client,
			guilds: this.client.guilds.cache,
			startTime: this.startTime,
			inviteLink: env.INVITE_LINK,
			//? Rolando specific
			chains: this.chainsService.chains,
			getGuildInvite: this.getGuildInvite.bind(this),
			MarkovChainAnalyzer,
		};

		const options = {
			version: env.VERSION,
			userInfo,
		};
		render(res, 'pages/dashboard', props, options);
		return;
	}

	private async getGuildInvite(
		req: Request<any, any, any, { guildId: string }>,
		res: Response
	) {
		const guild = this.client.guilds.cache.get(req.query.guildId)!;
		const channel = guild.channels.cache.find(
			(channel) => channel && channel.type === ChannelType.GuildText
		) as TextChannel | undefined;
		if (!channel) {
			res.send('<a><i style="color: red;" class="fa-solid fa-door-closed"></i></a>');
			return;
		}
		try {
			const invite = await channel.createInvite();
			res.send(
				`<a href="https://discord.gg/${invite.code}"><i class="fa-solid fa-door-open"></i></a>`
			);
		} catch (error) {
			res.send('<a><i style="color: red;" class="fa-solid fa-door-closed"></i></a>');
			return;
		}
	}

	private async guildChain(
		req: Request<any, any, any, { guildId: string }>,
		res: Response
	) {
		if (!req.session!['userInfo']) {
			res.redirect('/unauthorized');
			return;
		}
		const { guildId } = req.query;
		const chain = this.chainsService.chains.get(guildId) ?? {
			code: 404,
			message: `chain ${guildId} not found`,
		};
		res.status(!!chain ? 200 : 404).json(chain);
	}
}
