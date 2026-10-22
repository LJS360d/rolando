package commands

import (
	"context"
	"rolando/internal/data"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /vc language command
func (h *SlashCommandsHandler) vcLanguageCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	ctx := context.Background()
	var lang string
	for _, option := range i.SlashCommandInteractionData().Options {
		if option.Name == "language" && option.Type == discord.ApplicationCommandOptionTypeInt {
			lang = data.Langs[option.Int()]
			break
		}
	}

	chainDoc, err := h.ChainsService.GetChainConf(ctx, i.GuildID().String())
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

	if lang != "" {
		if !h.checkAdmin(i) {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(ctx, chainDoc.ID, map[string]interface{}{"tts_language": lang}); err != nil {
			s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
				Type: discord.InteractionResponseTypeCreateMessage,
				Data: discord.MessageCreate{
					Content: "Failed to update tts language.",
					Flags:   discord.MessageFlagEphemeral,
				},
			})
			return
		}
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "Set language to use in vc to `" + lang + "`",
			},
		})
		return
	}

	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Content: "Current language is `" + chainDoc.TTSLanguage + "`",
		},
	})
}
