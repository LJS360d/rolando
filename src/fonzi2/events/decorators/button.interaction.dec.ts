import 'reflect-metadata';
import { Handler } from '../handlers/base.handler';

type ButtonInteractionMetadata = {
	id: string;
	method: Function;
};

const buttonsKey = Symbol('button-interaction');

export function getButtonsMetadata(target: any): ButtonInteractionMetadata[] {
	return Reflect.getOwnMetadata(buttonsKey, Object.getPrototypeOf(target)) || [];
}

// ? @Button decorator
export function Button(id: string): MethodDecorator {
	return (target: Handler, _, descriptor: PropertyDescriptor) => {
		const method: Function = descriptor.value;
		const buttonMetadata = Reflect.getOwnMetadata(buttonsKey, target) || [];
		buttonMetadata.push({ id, method });
		Reflect.defineMetadata(buttonsKey, buttonMetadata, target);
	};
}
