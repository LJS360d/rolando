package commands

import (
	"strconv"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /cohesion command
func (h *SlashCommandsHandler) cohesionCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	options := i.SlashCommandInteractionData().Options
	var cohesion *int
	for _, option := range options {
		if option.Name == "value" && option.Type == discord.ApplicationCommandOptionTypeInt {
			value := int(option.Int())
			cohesion = &value
			break
		}
	}

	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID().String())
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
	if cohesion != nil {
		if !h.checkAdmin(i, "You are not authorized to change the cohesion value.") {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(chainDoc.ID, map[string]interface{}{"n_gram_size": *cohesion}); err != nil {
			s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
				Type: discord.InteractionResponseTypeCreateMessage,
				Data: discord.MessageCreate{
					Content: "Failed to update cohesion value.",
					Flags:   discord.MessageFlagEphemeral,
				},
			})
			return
		}

		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "Set cohesion value to `" + strconv.Itoa(*cohesion) + "`",
			},
		})
		return
	}

	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Content: "Current cohesion value is `" + strconv.Itoa(chainDoc.NGramSize) + "`",
		},
	})
}
