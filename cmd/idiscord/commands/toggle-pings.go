package commands

import (
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /togglepings command
func (h *SlashCommandsHandler) togglePingsCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	guildID := i.GuildID().String()
	chain, err := h.ChainsService.GetChain(guildID)
	if err != nil {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "Failed to retrieve chain data.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	if _, err := h.ChainsService.UpdateChainMeta(guildID, map[string]interface{}{"pings": !chain.Pings}); err != nil {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "Failed to toggle pings state.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	state := "disabled"
	if chain.Pings {
		state = "enabled"
	}
	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Content: "Pings are now `" + state + "`",
		},
	})
}
