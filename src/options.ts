import {
  GatewayIntentBits as Intents,
  Partials,
  type ClientOptions
} from 'discord.js';

const options: ClientOptions = {
	// allowedMentions: { parse: ['users', 'roles'] },
	// closeTimeout: 0,
	// failIfNotExists: true,
	// rest: { version: '10' },

	intents: [
		// Intents.AutoModerationConfiguration,
		// Intents.AutoModerationExecution,
		// Intents.DirectMessageReactions,
		Intents.DirectMessageReactions,
		Intents.DirectMessageTyping,
		Intents.DirectMessages,
		// Intents.GuildEmojisAndStickers,
		// Intents.GuildIntegrations,
		Intents.GuildInvites,
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
	/* jsonTransformer: (obj) => {
		Logger.trace(JSON.stringify(obj, null, 2));
		return obj;
	}, */
	/* presence: {
		activities: [{ name: 'a dumb dev', type: ActivityType.Watching }],
		afk: false,
		status: 'idle',
	}, */
};

export default options;
