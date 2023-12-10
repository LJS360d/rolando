import 'dotenv/config';
import { Logger } from './logger';
export const env = {
	// ! [REQUIRED] the discord bot's token
	TOKEN: process.env['TOKEN']!,
	// ? [Recommended] a webhook for logs
	LOG_WEBHOOK: process.env['LOG_WEBHOOK'],
	// ! [REQUIRED] OAuth2 credentials
	OAUTH2_URL: process.env['OAUTH2_URL']!,
	// ? [Recommended] a comma separated list of discord user IDs that can access the admin dashboard, leave empty for no auth
	OWNER_IDS: process.env['OWNER_IDS'] ? process.env['OWNER_IDS'].split(',') : [],
	// ? [Recommended] the bot Invite link
	INVITE_LINK: process.env['INVITE_LINK'],
	// * npm package version
	VERSION: process.env['npm_package_version']!,
	// * the server's port (default: 8080)
	PORT: Number(process.env['PORT']) || 8080,
	// * the current environment (default: development)
	NODE_ENV: (process.env['NODE_ENV'] || 'development') as
		| 'development'
		| 'staging'
		| 'test'
		| 'production',
	// ! Firebase configuration
	FIREBASE_ENABLED: process.env['FIREBASE_ENABLED'] || false,
	FIREBASE_CONFIG: {
		apiKey: process.env['apiKey'],
		authDomain: process.env['authDomain'],
		projectId: process.env['projectId'],
		storageBucket: process.env['storageBucket'],
		messagingSenderId: process.env['messagingSenderId'],
		appId: process.env['appId'],
		measurementId: process.env['measurementId'],
	},
} as const;

export function validateEnv(warn = true, Env = env) {
	let invalidEnv = false;
	for (const prop of ['TOKEN', 'OAUTH2_URL', 'VERSION'] as (keyof typeof Env)[]) {
		const invalidString = typeof Env[prop] === 'string' && !Env[prop];
		if (invalidString) {
			Logger.error(`Required property "${prop}" is missing in the env configuration.`);
			invalidEnv = true;
		}
	}

	if (warn) {
		for (const prop of [
			'INVITE_LINK',
			'LOG_WEBHOOK',
			'OWNER_IDS',
		] as (keyof typeof Env)[]) {
			const invalidString = typeof Env[prop] === 'string' && !Env[prop];
			const invalidArray =
				Array.isArray(Env[prop]) && !(Env[prop] as Array<string>).length;
			if (invalidString) {
				Logger.warn(
					`Recommended property "${prop}" is missing a value in the env configuration.`
				);
			}
			if (invalidArray) {
				Logger.warn(
					`Recommended property "${prop}" is missing values in the env configuration.`
				);
			}
		}
	}

	if (invalidEnv) process.exit(0);
}
