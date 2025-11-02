package buttons

import (
	"fmt"
	"rolando/internal/logger"
	"rolando/internal/utils"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Handle 'confirm-train-again' button interaction
func (h *ButtonsHandler) onConfirmTrainAgain(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Defer the update
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID)
	if err != nil {
		logger.Errorf("Failed to fetch chainDoc for guild %s: %v", i.GuildID, err)
		errMsg := "Failed to fetch current chain document for this server. Please try again later."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errMsg,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}
	var userId string
	if i.User != nil {
		userId = i.User.ID
	} else if i.Member != nil && i.Member.User != nil {
		userId = i.Member.User.ID
	} else {
		logger.Errorf("Failed to determine user ID for interaction in '%s'", chainDoc.Name)
		errMsg := "Unable to determine user ID for this interaction. Please try again later."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errMsg,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	cnt := "Deleting fetched data from this server.\nThis might take a while.."
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &cnt,
		Flags:   discordgo.MessageFlagsEphemeral,
	})

	// recreate the chain
	id := chainDoc.ID
	name := chainDoc.Name
	err = h.ChainsService.DeleteChain(id)
	if err != nil {
		logger.Errorf("Failed to delete chain for guild %s: %v", i.GuildID, err)
		// Send error message
		errMsg := "Failed to delete chain data for this server. Please try again later."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errMsg,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}
	_, err = h.ChainsService.CreateChain(id, name)
	if err != nil {
		logger.Errorf("Failed to create chain for guild %s: %v", i.GuildID, err)
		// Send error message
		errMsg := "Failed to recreate a new chain for this server. Please try again later."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errMsg,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// Start the training process
	// Send confirmation message
	content := fmt.Sprintf("<@%s> Started Refetching messages.\nI  will send a message when I'm done.\nEstimated Time: `1 Minute per every 5000 Messages in the Server`\nThis might take a while..", userId)
	s.ChannelMessageSend(i.ChannelID, content)
	s.InteractionResponseDelete(i.Interaction)

	// Update chain status
	now := time.Now()
	chainDoc.TrainedAt = &now
	if _, err = h.ChainsService.UpdateChainMeta(i.GuildID, map[string]any{"trained_at": now}); err != nil {
		logger.Errorf("Failed to update chain document for guild %s: %v", i.GuildID, err)
		return
	}
	go func() {
		startTime := time.Now()
		messages, err := h.DataFetchService.FetchAllGuildMessages(i.GuildID)
		if err != nil {
			logger.Errorf("Failed to fetch messages for guild %s: %v", i.GuildID, err)
			// Revert chain status
			chainDoc.TrainedAt = nil
			if _, err = h.ChainsService.UpdateChainMeta(i.GuildID, map[string]any{"trained_at": nil}); err != nil {
				logger.Errorf("Failed to update chain document for guild %s: %v", i.GuildID, err)
			}
			return
		}

		// Send completion message
		s.ChannelMessageSend(i.ChannelID, fmt.Sprintf("<@%s> Finished Fetching messages.\nMessages fetched: `%s`\nTime elapsed: `%s`\nMessages/Second: `%s`",
			userId,
			utils.FormatNumber(float64(len(messages))),
			time.Since(startTime).String(),
			utils.FormatNumber(float64(len(messages))/(time.Since(startTime).Seconds()))))
	}()
}
