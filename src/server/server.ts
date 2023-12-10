import axios, { AxiosResponse } from 'axios';
import session from 'cookie-session';
import crypto from 'crypto';
import { Client } from 'discord.js';
import express, { NextFunction, type Request, type Response } from 'express';
import http from 'http';
import { env } from '../lib/env';
import { CC, Logger } from '../lib/logger';
import { DiscordUserInfo } from '../types/discord.user.info';
export class Fonzi2Server {
	private readonly startTime = Date.now();
	private port: number = env.PORT;
	private app: express.Application;
	private httpServer: http.Server;
	constructor(private client: Client) {
		this.app = express();
		this.httpServer = http.createServer(this.app);
		this.app.use(express.static('public'));
		this.app.use(express.json());
		const secret = crypto.createHash('sha3-256').update(JSON.stringify(env)).digest('hex');
		this.app.use(session({ secret }));
		this.app.set('view engine', 'ejs');
	}

	start() {
		this.httpServer.listen(this.port, () => {
			if (env.NODE_ENV === 'development') {
				Logger.info(
					`Server listening on ${CC.doubleUnderline}http://localhost:${env.PORT}`
				);
			} else Logger.info(`Server open on port ${env.PORT}`);
		});
		this.app.get('/', this.authorize.bind(this));
		this.app.get('/unauthorized', this.unauthorized.bind(this));
		this.app.get('/notfound', this.notFound.bind(this));
		this.app.get('/login', this.login.bind(this));
		this.app.post('/login', this.loginPost.bind(this));
		this.app.get('/dashboard', this.dashboard.bind(this));

		this.app.use(this.notFoundMiddleware.bind(this));

		process.on('SIGTERM', () => {
			this.stop();
		});
	}

	stop() {
		this.httpServer.close();
	}

	protected dashboard(req: Request, res: Response) {
		const userInfo = req.session['userInfo'];
		if (!userInfo) {
			res.redirect('/unauthorized');
			return;
		}
		const props = {
			client: this.client,
			guilds: this.client.guilds.cache,
			startTime: this.startTime,
			version: env.VERSION,
      inviteLink: env.INVITE_LINK,
			userInfo,
		};
		res.render('default/dashboard', props);
	}

	protected authorize(req: Request, res: Response) {
		const userInfo = req.session['userInfo'];
		if (!userInfo) {
			res.redirect(env.OAUTH2_URL);
			return;
		}
		res.redirect('/dashboard');
	}

	protected login(req: Request, res: Response) {
		res.render('default/login');
	}

	protected unauthorized(req: Request, res: Response) {
		res.render('default/unauthorized');
	}

	protected notFound(req: Request, res: Response) {
		res.render('default/notfound');
	}

	protected notFoundMiddleware(req: Request, res: Response, next: NextFunction) {
		res.redirect('/notfound');
		next();
	}

	private async loginPost(req: Request, res: Response) {
		const { accessToken } = req.body;
		try {
			const userInfo = await this.getDiscordAuthUserInfo(accessToken);
			const skipAuth = env.OWNER_IDS.length === 0;
			if (skipAuth || env.OWNER_IDS.includes(userInfo.id)) {
				req.session['userInfo'] = userInfo;
				res.status(302).json({ route: '/dashboard' });
			} else res.status(401).json({ route: '/unauthorized' });
		} catch (error: any) {
			res.status(502).json({ msg: error.message });
		}
	}

	private async getDiscordAuthUserInfo(accessToken: string): Promise<DiscordUserInfo> {
		try {
			const authResponse: AxiosResponse<DiscordUserInfo, any> = await axios.get(
				'https://discord.com/api/v10/users/@me',
				{
					headers: {
						Authorization: `Bearer ${accessToken}`,
					},
				}
			);
			const userId = authResponse.data.id;
			const userAvatarHash = authResponse.data.avatar;
			authResponse.data.avatar = `https://cdn.discordapp.com/avatars/${userId}/${userAvatarHash}.png`;
			return authResponse.data;
		} catch (error: any) {
			throw error;
		}
	}
}
