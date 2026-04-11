package commands

import (
	"fmt"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /wipe command
func (h *SlashCommandsHandler) wipeCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	options := i.SlashCommandInteractionData().Options
	var data string
	for _, option := range options {
		if option.Name == "data" && option.Type == discord.ApplicationCommandOptionTypeString {
			data = option.String()
			break
		}
	}

	if data == "" {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "You must provide the data to be erased.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	chain, err := h.ChainsService.GetChain(i.GuildID().String())
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

	err = h.ChainsService.DeleteTextData(i.GuildID().String(), data)
	if err != nil {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "Failed to delete the specified data.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	chain.Delete(data)
	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Content: fmt.Sprintf("Deleted `%s`", data),
		},
	})
}
