package commands

import (
	"rolando/internal/utils"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /opinion command
func (h *SlashCommandsHandler) opinionCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	options := i.SlashCommandInteractionData().Options
	var about string
	for _, option := range options {
		if option.Name == "about" && option.Type == discord.ApplicationCommandOptionTypeString {
			about = option.String()
			break
		}
	}

	if about == "" {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "You must provide a word as the seed.",
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

	// Generate text with random length between 8 and 40
	msg := chain.GenerateTextFromSeed(about, utils.GetRandom(8, 40))
	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Content: msg,
		},
	})
}
