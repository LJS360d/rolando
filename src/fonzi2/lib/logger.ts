import axios from 'axios';
import { EmbedBuilder } from 'discord.js';
import { env } from './env';
// ConsoleColors
export enum CC {
	stop = '\x1b[0m',
	bold = '\x1b[1m',
	italic = '\x1b[3m',
	underline = '\x1b[4m',
	highlight = '\x1b[7m',
	hidden = '\x1b[8m',
	strikethrough = '\x1b[8m',
	doubleUnderline = '\x1b[21m',
	black = '\x1b[30m',
	gray = '\x1b[37m',
	red = '\x1b[31m',
	green = '\x1b[32m',
	yellow = '\x1b[33m',
	blue = '\x1b[34m',
	magenta = '\x1b[35m',
	cyan = '\x1b[36m',
	white = '\x1b[38m',
	blackbg = '\x1b[40m',
	redbg = '\x1b[41m',
	greenbg = '\x1b[42m',
	yellowbg = '\x1b[43m',
	bluebg = '\x1b[44m',
	magentabg = '\x1b[45m',
	cyanbg = '\x1b[46m',
	whitebg = '\x1b[47m',
}
// DiscordColors
export enum DC {
	black = 2303786,
	gray = 9807270,
	red = 15548997,
	green = 5763719,
	yellow = 16776960,
	blue = 3447003,
	magenta = 15418782,
	cyan = 1752220,
	white = 16777215,
}

export class Logger {
	protected static readonly pattern = `${CC.gray}[%time]$ %color%level$ ${CC.white}%msg$`;
	static remoteEnabled = true;

	public static info(msg: string | object): void {
		this.log('INFO', 'green', msg);
	}

	public static trace(msg: string | object): void {
		this.log('TRACE', 'cyan', msg);
	}

	public static debug(msg: string | object): void {
		this.log('DEBUG', 'magenta', msg);
	}

	public static warn(msg: string | object): void {
		this.log('WARN', 'yellow', msg);
	}

	public static error(msg: string | object): void {
		this.log('ERROR', 'red', msg);
	}

	public static loading() {
		const frames = ['|', '/', '-', '\\'];
		const level = 'LOAD';
		let i = 0;
		let loader: NodeJS.Timeout;
		let isLoading = false;

		return (msg: string, success = true) => {
			const updatePattern = (end?: boolean) => {
				const frame = frames[i++];
				const nextMsg = end
					? `${success ? `${CC.green}✓$` : `${CC.red}✗$`} ${msg}`
					: `${frame} ${msg}`;
				i %= frames.length;
				return this.$(
					this.pattern
						.replace('%time', this.now())
						.replace('%level', level)
						.replace('%msg', nextMsg)
						.replace('%color', CC.magenta)
				);
			};

			if (!isLoading) {
				// Start the loading animation
				isLoading = true;
				loader = setInterval(() => process.stdout.write(`${updatePattern()}\r`), 100);
			} else {
				// Stop the loading animation
				clearInterval(loader);
				process.stdout.write(`${updatePattern(true)}\n`);
				if (this.remoteEnabled)
					this.remoteLog(level, 'magenta', `${success ? `✓` : `✗`} ${msg}`);
				isLoading = false;
			}
		};
	}

	private static log(level: string, color: keyof typeof CC, msg: string | object): void {
		if (typeof msg !== 'string') {
			msg = JSON.stringify(msg, null, 2);
		}
		const pattern = this.pattern
			.replace('%time', this.now())
			.replace('%level', level)
			.replace('%msg', msg)
			.replace('%color', CC[color] ?? CC.white);

		console.log(this.$(pattern));    
		if (this.remoteEnabled && !['DEBUG', 'TRACE'].includes(level)) this.remoteLog(level, color as keyof typeof DC, msg);
	}

	private static remoteLog(level: string, color: keyof typeof DC, msg: string) {
		const embed = new EmbedBuilder()
			.setColor(DC[color] ?? DC.white)
			.setDescription(this.now())
			.addFields({
				name: `${env.NODE_ENV.charAt(0).toUpperCase() + env.NODE_ENV.slice(1)} ${level}`,
				value: msg,
				inline: true,
			});
		const body = JSON.stringify({ embeds: [embed] });

		try {
			if (env.LOG_WEBHOOK)
				axios.post(env.LOG_WEBHOOK, body, {
					headers: {
						'Content-Type': 'application/json',
					},
				});
		} catch (error: any) {
			if (error.response && error.response.status === 429) {
				// ignore 429 error response
			}
		}
	}

	private static $(str: string) {
		return str.replace(/\$/g, CC.stop);
	}

	private static now() {
		return new Date().toLocaleTimeString('en-GB', {
			day: '2-digit',
			month: '2-digit',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
		});
	}
}
