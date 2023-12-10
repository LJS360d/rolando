import Fonzi2Client from './fonzi2/client/client';
import { options } from './fonzi2/client/options';
import {
  getRegisteredCommands
} from './fonzi2/events/decorators/command.interaction.dec';
import { ButtonInteractionHandler } from './fonzi2/events/handlers/buttons/buttons.handler';
import { CommandInteractionsHandler } from './fonzi2/events/handlers/commands/commands.handler';
import { ClientEventsHandler } from './fonzi2/events/handlers/client-events/client.events.handler';
import { Logger } from './fonzi2/lib/logger';
import { MessageHandler } from './fonzi2/events/handlers/message/message.handler';

new Fonzi2Client(options, [
	new CommandInteractionsHandler(),
	new ButtonInteractionHandler(),
  new MessageHandler(),
	new ClientEventsHandler(getRegisteredCommands()),
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
