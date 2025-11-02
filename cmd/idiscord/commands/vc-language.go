package commands

import (
	"rolando/internal/data"

	"github.com/bwmarrin/discordgo"
)

// implementation of /vc language command
func (h *SlashCommandsHandler) vcLanguageCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var lang string
	for _, option := range i.ApplicationCommandData().Options {
		if option.Name == "language" && option.Type == discordgo.ApplicationCommandOptionInteger {
			lang = data.Langs[option.IntValue()]
			break
		}
	}

	chainId := i.GuildID
	chainDoc, err := h.ChainsService.GetChainDocument(chainId)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if lang != "" {
		if !h.checkAdmin(i) {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(chainDoc.ID, map[string]interface{}{"tts_language": lang}); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Failed to update tts language.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Set language to use in vc to `" + lang + "`",
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Current language is `" + chainDoc.TTSLanguage + "`",
		},
	})
}
