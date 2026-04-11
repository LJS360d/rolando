package helpers

import (
	"slices"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// HasGuildTextChannelAccess checks if the bot user has access to the specified guild text channel.
func HasGuildTextChannelAccess(client *bot.Client, userId snowflake.ID, channel discord.GuildChannel) bool {
	if channel.Type() != discord.ChannelTypeGuildText && channel.Type() != discord.ChannelTypeGuildNews {
		return false
	}

	member, ok := client.Caches.Member(channel.GuildID(), userId)
	if !ok {
		return false
	}

	permissions := client.Caches.MemberPermissionsInChannel(channel, member)

	return permissions.Has(
		discord.PermissionViewChannel,
		discord.PermissionReadMessageHistory,
		discord.PermissionSendMessages,
	)
}

// MentionsUser checks if the user is mentioned in the message.
func MentionsUser(message discord.Message, member discord.Member) bool {
	// Check direct mentions
	for _, user := range message.Mentions {
		if user.ID == member.User.ID {
			return true
		}
	}

	// Check role mentions
	for _, roleID := range message.MentionRoles {
		if slices.Contains(member.RoleIDs, roleID) {
			return true
		}
	}

	return false
}
