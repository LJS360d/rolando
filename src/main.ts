import Fonzi2Client from './client/client';
import { options } from './client/options';
import {
  getRegisteredCommands
} from './events/decorators/command.interaction.dec';
import { ButtonInteractionHandler } from './events/handlers/buttons/buttons.handler';
import { CommandInteractionsHandler } from './events/handlers/commands/commands.handler';
import { ClientEventsHandler } from './events/handlers/client-events/client.events.handler';
import { Logger } from './lib/logger';
import { MessageHandler } from './events/handlers/message/message.handler';

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
