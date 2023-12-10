import { GatewayIntentBits as Intents, type ClientOptions, Partials } from 'discord.js';

export const options: ClientOptions = {
	// allowedMentions: { parse: ['users', 'roles'] },
	intents: [
		// Intents.AutoModerationConfiguration,
		// Intents.AutoModerationExecution,
		// Intents.DirectMessageReactions,
		Intents.DirectMessageReactions,
		Intents.DirectMessageTyping,
		Intents.DirectMessages,
		// Intents.GuildEmojisAndStickers,
		// Intents.GuildIntegrations,
		// Intents.GuildInvites,
		Intents.GuildMembers,
		Intents.GuildMessageReactions,
		Intents.GuildMessageTyping,
		Intents.GuildMessages,
		// Intents.GuildModeration,
		// Intents.GuildPresences,
		// Intents.GuildScheduledEvents,
		// Intents.GuildVoiceStates,
		// Intents.GuildWebhooks,
		Intents.Guilds,
		Intents.MessageContent,
	],
	partials: [
		Partials.Channel,
		Partials.Message,
		Partials.User,
		// Partials.GuildMember,
		// Partials.Reaction,
	],
	// closeTimeout: 0,
	// failIfNotExists: true,
	/* jsonTransformer: (obj) => {
		console.log(obj);
		return obj;
	}, */
	// rest: { version: '10' },
	/* presence: {
		activities: [{ name: 'a dumb dev', type: ActivityType.Watching }],
		afk: false,
		status: 'online',
	}, */
};
