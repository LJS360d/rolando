import { Fonzi2Client, getRegisteredCommands, Logger } from 'fonzi2';
import { ChainsService } from './domain/services/chains.service';
import { env } from './env';
import { firestore, storage } from './firebase/firebase';
import { ButtonsHandler } from './handlers/buttons.handler';
import { CommandsHandler } from './handlers/commands.handler';
import { EventsHandler } from './handlers/events.handler';
import { MessageHandler } from './handlers/message.handler';
import options from './options';
const chainService = new ChainsService(firestore, storage);
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
});


['SIGINT', 'SIGTERM'].forEach((signal) => {
	process.on(signal, () => {
		Logger.warn(`Received ${signal} signal`);
		process.exit(0);
	});
});
