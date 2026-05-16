package commands

import (
	"context"
	"strings"

	"rolando/internal/utils"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func (h *SlashCommandsHandler) rhymeCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	options := i.SlashCommandInteractionData().Options
	var text string
	for _, option := range options {
		if option.Name == "with" && option.Type == discord.ApplicationCommandOptionTypeString {
			text = option.String()
			break
		}
	}

	if text == "" {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "You must provide a word to rhyme with.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	fields := strings.Fields(text)
	rhymeWord := fields[len(fields)-1]

	msg, err := h.ChainsService.GenerateRhyme(context.Background(), i.GuildID().String(), rhymeWord, utils.GetRandom(4, 14))
	if err != nil || msg == "" {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "Failed to generate a rhyme.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Content: msg,
		},
	})
}
