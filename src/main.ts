import { Fonzi2Client, getRegisteredCommands, Logger } from 'fonzi2';
import mongoose from 'mongoose';
import { connectMongo } from './domain/repositories/mongo.connector';
import { ChainsService } from './domain/services/chains.service';
import { env } from './env';
import { ButtonsHandler } from './handlers/buttons.handler';
import { CommandsHandler } from './handlers/commands.handler';
import { EventsHandler } from './handlers/events.handler';
import { MessageHandler } from './handlers/message.handler';
import options from './options';
import { ChainsRepository } from './domain/repositories/chains.repository';
async function main() {
	const db = (await connectMongo(env.MONGODB_URI));

	const chainService = new ChainsService(new ChainsRepository());

	new Fonzi2Client(env.TOKEN, options, [
		new CommandsHandler(chainService),
		new ButtonsHandler(chainService),
		new MessageHandler(chainService),
		new EventsHandler(getRegisteredCommands(), chainService),
	]);

	process.on('uncaughtException', (err: any) => {
		if (err?.response?.status !== 429)
			Logger.error(`${err.name}: ${err.message}\n${err.stack}`);
	});

	process.on('unhandledRejection', (reason: any) => {
		if (reason?.status === 429) return;
		if (reason?.response?.status === 429) return;
	});

	['SIGINT', 'SIGTERM'].forEach((signal) => {
		process.on(signal, async () => {
			Logger.warn(`Received ${signal} signal, closing &u${db?.name}$ database connection`);
			await db?.close();
			process.exit(0);
		});
	});
}

void main();
