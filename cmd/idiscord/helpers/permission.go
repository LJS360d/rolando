package helpers

import (
	"errors"
	"slices"

	"github.com/bwmarrin/discordgo"
)

// HasGuildTextChannelAccess checks if the bot user has access to the specified guild text channel.
func HasGuildTextChannelAccess(ds *discordgo.Session, userId string, channel *discordgo.Channel) bool {
	if channel.Type != discordgo.ChannelTypeGuildText && channel.Type != discordgo.ChannelTypeGuildNews {
		return false
	}

	botMember, err := ds.State.Member(channel.GuildID, ds.State.User.ID)
	if err == nil && botMember != nil {
		perms := botMember.Permissions
		canRead := perms&discordgo.PermissionReadMessageHistory != 0
		canSend := perms&discordgo.PermissionSendMessages != 0
		canView := perms&discordgo.PermissionViewChannel != 0
		canReact := perms&discordgo.PermissionAddReactions != 0
		return canRead && canSend && canView && canReact
	}

	permissions, err := ds.UserChannelPermissions(userId, channel.ID)
	if err != nil {
		return false
	}

	canRead := permissions&discordgo.PermissionReadMessageHistory != 0
	canSend := permissions&discordgo.PermissionSendMessages != 0
	canView := permissions&discordgo.PermissionViewChannel != 0
	canReact := permissions&discordgo.PermissionAddReactions != 0

	return canRead && canSend && canView && canReact
}

// MentionsUser checks if the user is mentioned in the message.
func MentionsUser(message *discordgo.Message, userID string, guild *discordgo.Guild) bool {
	// Check direct mentions
	for _, user := range message.Mentions {
		if user.ID == userID {
			return true
		}
	}

	// Check role mentions
	if guild != nil {
		botMember, err := guildMember(guild, userID)
		if err == nil {
			for _, roleID := range message.MentionRoles {
				if slices.Contains(botMember.Roles, roleID) {
					return true
				}
			}
		}
	}

	return false
}

// guildMember retrieves a member from a guild by user ID.
func guildMember(guild *discordgo.Guild, userID string) (*discordgo.Member, error) {
	for _, member := range guild.Members {
		if member.User.ID == userID {
			return member, nil
		}
	}
	return nil, errors.New("member not found")
}
