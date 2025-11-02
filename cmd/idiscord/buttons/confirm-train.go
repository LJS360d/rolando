package buttons

import (
	"fmt"
	"rolando/internal/logger"
	"rolando/internal/utils"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Handle 'confirm-train' button interaction
func (h *ButtonsHandler) onConfirmTrain(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Defer the update
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID)
	if err != nil {
		logger.Errorf("Failed to fetch chainDoc for guild %s: %v", i.GuildID, err)
		return
	}

	// redundant check
	if chainDoc.TrainedAt != nil && chainDoc.TrainedAt.Before(time.Now()) {
		s.ChannelMessageSend(i.ChannelID, "Training already completed for this server.")
		return
	}

	var userId string
	if i.User != nil {
		userId = i.User.ID
	} else if i.Member != nil && i.Member.User != nil {
		userId = i.Member.User.ID
	} else {
		logger.Errorf("Failed to determine user ID for interaction in '%s'", chainDoc.Name)
		return
	}

	// Start the training process
	// Send confirmation message
	content := fmt.Sprintf("<@%s> Started Fetching messages.\nI  will send a message when I'm done.\nEstimated Time: `1 Minute per every 5000 Messages in the Server`\nThis might take a while..", userId)
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
