package commands

import (
	"context"
	"strings"

	"rolando/internal/logger"
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

	fields := strings.Fields(text)
	if len(fields) == 0 {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "You must provide a word to rhyme with.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	if err := i.DeferCreateMessage(false); err != nil {
		logger.Errorf("Failed to defer interaction response: %v", err)
		return
	}
	go h.rhyme(s, i, fields[len(fields)-1])
}

func (h *SlashCommandsHandler) rhyme(s *bot.Client, i *events.ApplicationCommandInteractionCreate, rhymeWord string) {
	msg, err := h.ChainsService.GenerateRhyme(context.Background(), i.GuildID().String(), rhymeWord, utils.GetRandom(4, 9))
	if err != nil {
		logger.Errorf("Failed to generate a rhyme: %v", err)
	}
	if err != nil || msg == "" {
		msg = "Failed to generate a rhyme"
	}
	if _, err := s.Rest.UpdateInteractionResponse(s.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent(msg)); err != nil {
		logger.Errorf("Failed to update interaction response: %v", err)
	}
}
