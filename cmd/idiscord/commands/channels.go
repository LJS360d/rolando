package commands

import (
	"fmt"
	"rolando/cmd/idiscord/helpers"
	"strings"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /channels command
func (h *SlashCommandsHandler) channelsCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	var channels []discord.GuildChannel
	s.Caches.ChannelsForGuild(*i.GuildID())(func(ch discord.GuildChannel) bool {
		if ch.Type() == discord.ChannelTypeGuildText || ch.Type() == discord.ChannelTypeGuildNews {
			channels = append(channels, ch)
		}
		return true // continue iterating
	})

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
		hasAccess := helpers.HasGuildTextChannelAccess(s, s.ID(), ch)
		fmt.Fprintf(responseBuilder, "%s %s\n", accessEmote(hasAccess), ch.Mention())
	}

	responseText := responseBuilder.String()
	if len(responseText) == 0 {
		responseText = "No available channels to display."
	}

	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.NewMessageCreate().WithContent(responseText),
	})
}
