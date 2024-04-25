import {
	type ClientUser,
	type GuildBasedChannel,
	type GuildTextBasedChannel,
	PermissionFlagsBits,
	type Message,
	type User,
	Collection,
	type Role,
} from 'discord.js';

export function hasChannelAccess(
	user: ClientUser,
	channel: GuildBasedChannel
): channel is GuildTextBasedChannel {
	const perms = channel.permissionsFor(user);
	if (!perms) return false;
	const canReadChannel = perms.has(PermissionFlagsBits.ReadMessageHistory);
	const canAccessChannel = perms.has(PermissionFlagsBits.SendMessages);
	const canViewChannel = perms.has(PermissionFlagsBits.ViewChannel);
	return (
		channel.isTextBased() &&
		!channel.isVoiceBased() &&
		canReadChannel &&
		canAccessChannel &&
		canViewChannel
	);
}

export function mentionsUser(message: Message, user: User) {
	const { guild, mentions } = message;
	const botRoles =
		guild?.members.cache.find((m) => m.id === user.id)?.roles.cache ??
		new Collection<string, Role>();
	return (
		message.mentions.users.has(user.id) ||
		mentions.roles.some((role) => botRoles.hasAny(role.id))
	);
}
