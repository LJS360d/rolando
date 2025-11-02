package commands

import (
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// implementation of /cohesion command
func (h *SlashCommandsHandler) cohesionCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var cohesion *int
	for _, option := range options {
		if option.Name == "value" && option.Type == discordgo.ApplicationCommandOptionInteger {
			value := int(option.IntValue())
			cohesion = &value
			break
		}
	}

	guildID := i.GuildID
	chainDoc, err := h.ChainsService.GetChainDocument(guildID)
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
	if cohesion != nil {
		if !h.checkAdmin(i, "You are not authorized to change the cohesion value.") {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(chainDoc.ID, map[string]interface{}{"n_gram_size": *cohesion}); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Failed to update cohesion value.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Set cohesion value to `" + strconv.Itoa(*cohesion) + "`",
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Current cohesion value is `" + strconv.Itoa(chainDoc.NGramSize) + "`",
		},
	})
}
