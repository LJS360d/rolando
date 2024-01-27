import { Fonzi2Client, getRegisteredCommands, Logger } from 'fonzi2';
import { ChainsRepository } from './domain/repositories/chains/chains.repository';
import { ChainsService } from './domain/services/chains.service';
import { env } from './env';
import { ButtonsHandler } from './handlers/buttons.handler';
import { CommandsHandler } from './handlers/commands.handler';
import { EventsHandler } from './handlers/events.handler';
import { MessageHandler } from './handlers/message.handler';
import options from './options';
import { connectMongo } from './domain/repositories/common/mongo.connector';
import { TextDataRepository } from './domain/repositories/fs-storage/text-data.repository';
import { Container } from 'typedi';
import { Client } from 'discord.js';

async function main() {
	const db = await connectMongo(env.MONGODB_URI, 'rolando');

	const textDataRepository = new TextDataRepository();
	const chainsRepository = new ChainsRepository(textDataRepository);
	const chainService = new ChainsService(chainsRepository);

	const client = new Fonzi2Client(env.TOKEN, options, [
		new CommandsHandler(chainService),
		new ButtonsHandler(chainService),
		new MessageHandler(chainService),
		new EventsHandler(getRegisteredCommands(), chainService),
	]);
	Container.set(Client, client);

	process.on('uncaughtException', (err: any) => {
		if (err?.response?.status !== 429)
			Logger.error(`${err.name}: ${err.message}\n${err.stack}`);
	});

	process.on('unhandledRejection', (reason: any) => {
		if (reason?.status === 429) return;
		if (reason?.response?.status === 429) return;
		Logger.error(`Unhandled Promise Rejection: ${JSON.stringify(reason, null, 2)}`);
	});

	['SIGINT', 'SIGTERM'].forEach((signal) => {
		process.on(signal, async () => {
			Logger.warn(
				`Received ${signal} signal, closing &u${db?.name}$ database connection`
			);
			await db?.close();
			process.exit(0);
		});
	});
}

void main();
