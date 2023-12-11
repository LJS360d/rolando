import { Handler, HandlersType } from '../../fonzi2/events/handlers/base.handler';

export class CommandsHandler extends Handler {
	public readonly type = HandlersType.commandInteraction;
}
