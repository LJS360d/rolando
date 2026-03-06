package helpers

import (
	"errors"

	"github.com/bwmarrin/discordgo"
)

// HasGuildTextChannelAccess checks if the bot user has access to the specified guild text channel.
func HasGuildTextChannelAccess(ds *discordgo.Session, userId string, channel *discordgo.Channel) bool {
	if channel.Type != discordgo.ChannelTypeGuildText && channel.Type != discordgo.ChannelTypeGuildNews {
		return false
	}

	permissions, err := ds.UserChannelPermissions(userId, channel.ID)
	if err != nil {
		return false
	}

	canReadChannel := permissions&discordgo.PermissionReadMessageHistory != 0
	canAccessChannel := permissions&discordgo.PermissionSendMessages != 0
	canViewChannel := permissions&discordgo.PermissionViewChannel != 0

	return canReadChannel && canAccessChannel && canViewChannel
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
				for _, botRole := range botMember.Roles {
					if botRole == roleID {
						return true
					}
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
