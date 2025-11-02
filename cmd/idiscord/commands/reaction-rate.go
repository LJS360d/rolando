package commands

import (
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// implementation of /reactionrate command
func (h *SlashCommandsHandler) reactionRateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var rate *int
	for _, option := range options {
		if option.Name == "rate" && option.Type == discordgo.ApplicationCommandOptionInteger {
			value := int(option.IntValue())
			rate = &value
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
	var ratePercent float64
	if rate != nil {
		if !h.checkAdmin(i, "You are not authorized to change the reaction rate.") {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(chainDoc.ID, map[string]interface{}{"reaction_rate": *rate}); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Failed to update reaction rate.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		if *rate == 0 {
			ratePercent = 0
		} else {
			ratePercent = float64(1 / float64(*rate) * 100)
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Set reaction rate to `" + strconv.Itoa(*rate) + "` (" + strconv.FormatFloat(ratePercent, 'f', 2, 64) + "%)",
			},
		})
		return
	}

	if chainDoc.ReactionRate == 0 {
		ratePercent = 0
	} else {
		ratePercent = float64(1 / float64(chainDoc.ReactionRate) * 100)
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Current reaction rate is `" + strconv.Itoa(chainDoc.ReactionRate) + "` (" + strconv.FormatFloat(ratePercent, 'f', 2, 64) + "%)",
		},
	})
}
