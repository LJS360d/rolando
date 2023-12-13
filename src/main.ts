import Fonzi2Client from './fonzi2/client/client';
import { options } from './fonzi2/client/options';
import { getRegisteredCommands } from './fonzi2/events/decorators/command.interaction.dec';
import { Logger } from './fonzi2/lib/logger';
import { firestore, storage } from './fonzi2/server/firebase/firebase';
import { ChainsService } from './rolando/domain/services/chains.service';
import { ButtonsHandler } from './rolando/handlers/buttons.handler';
import { CommandsHandler } from './rolando/handlers/commands/commands.handler';
import { EventsHandler } from './rolando/handlers/events.handler';
import { MessageHandler } from './rolando/handlers/message.handler';
const chainService = new ChainsService(firestore, storage);
new Fonzi2Client(options, [
	new CommandsHandler(chainService),
	new ButtonsHandler(chainService),
	new MessageHandler(chainService),
	new EventsHandler(getRegisteredCommands(), chainService),
]);

process.on('uncaughtException', (err) => {
	Logger.error(`${err.name}: ${err.message}\n${err.stack}`);
});

['SIGINT', 'SIGTERM'].forEach((signal) => {
	process.on(signal, () => {
		Logger.warn(`Received ${signal} signal`);
		process.exit(0);
	});
});
