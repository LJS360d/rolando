package commands

import (
	"rolando/internal/discord/helpers"
	"rolando/internal/discord/pager"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func init() {
	pager.Register("channels", pager.Source{
		Header: "Channels the bot has access to are marked with: :green_circle:\n" +
			"While channels with no access are marked with: :red_circle:\n" +
			"Make a channel accessible by giving **ALL** these permissions:\n" +
			"`View Channel` `Send Messages` `Read Message History`\n\n",
		Empty: "No available channels to display.",
		Lines: channelLines,
	})
}

func channelLines(client *bot.Client, guildID snowflake.ID) []string {
	var lines []string
	for ch := range client.Caches.ChannelsForGuild(guildID) {
		if ch.Type() != discord.ChannelTypeGuildText && ch.Type() != discord.ChannelTypeGuildNews {
			continue
		}
		mark := ":red_circle:"
		if helpers.HasGuildTextChannelAccess(client, client.ID(), ch) {
			mark = ":green_circle:"
		}
		lines = append(lines, mark+" "+ch.Mention())
	}
	return lines
}

// implementation of /channels command
func (h *SlashCommandsHandler) channelsCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	content, components := pager.Render(s, "channels", *i.GuildID(), 0)
	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.NewMessageCreate().WithContent(content).WithComponents(components...),
	})
}
