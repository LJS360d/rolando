import 'reflect-metadata';
import { ClientEvents } from 'discord.js';
import { Handler } from '../handlers/base.handler';

type ClientEvent = keyof Omit<ClientEvents, 'messageCreate' | 'debug'>

export type ClientEventMetadata = { event: ClientEvent; method: Function };

const eventsKey = Symbol('client-event');
export function getEventsMetadata(target: any): ClientEventMetadata[] {
	return Reflect.getOwnMetadata(eventsKey, Object.getPrototypeOf(target)) || [];
}

export function ClientEvent(event: ClientEvent): MethodDecorator {
	return (target: Handler, _, descriptor: PropertyDescriptor) => {
		const clientEvents: ClientEventMetadata[] =
			Reflect.getOwnMetadata(eventsKey, target) || [];
		clientEvents.push({ event, method: descriptor.value });

		Reflect.defineMetadata(eventsKey, clientEvents, target);
	};
}
