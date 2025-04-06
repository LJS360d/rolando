package commands

import (
	"fmt"
	"rolando/internal/utils"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// implementation of /channels command
func (h *SlashCommandsHandler) channelsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve guild information.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	var channels []*discordgo.Channel
	for _, channel := range guild.Channels {
		if channel.Type != discordgo.ChannelTypeGuildVoice && channel.Type != discordgo.ChannelTypeGuildCategory {
			channels = append(channels, channel)
		}
	}

	accessEmote := func(hasAccess bool) string {
		if hasAccess {
			return ":green_circle:"
		}
		return ":red_circle:"
	}

	responseBuilder := &strings.Builder{}
	responseBuilder.WriteString(fmt.Sprintf("Channels the bot has access to are marked with: %s\nWhile channels with no access are marked with: %s\nMake a channel accessible by giving %s these permissions:\n%s %s %s\n\n",
		":green_circle:",
		":red_circle:",
		"**ALL**",
		"`View Channel`", "`Send Messages`", "`Read Message History`",
	))

	for _, ch := range channels {
		hasAccess := utils.HasGuildTextChannelAccess(s, s.State.User.ID, ch)
		fmt.Fprintf(responseBuilder, "%s <#%s>\n", accessEmote(hasAccess), ch.ID)
	}

	responseText := responseBuilder.String()
	if len(responseText) == 0 {
		responseText = "No available channels to display."
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: responseText,
		},
	})
}
