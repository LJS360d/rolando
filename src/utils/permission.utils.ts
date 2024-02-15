import {
	ClientUser,
	GuildBasedChannel,
	GuildTextBasedChannel,
	PermissionFlagsBits,
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
