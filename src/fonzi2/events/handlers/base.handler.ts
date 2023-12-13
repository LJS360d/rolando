import { Client } from 'discord.js';

export abstract class Handler {
	public abstract readonly type: HandlersType;
	public client?: Client;
}

export enum HandlersType {
	commandInteraction = 'command-interaction',
	buttonInteraction = 'button-interaction',
	clientEvent = 'client-event',
	messageEvent = 'message-event',
}
