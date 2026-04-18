package buttons

import (
	"context"
	"fmt"
	"rolando/internal/logger"
	"rolando/internal/utils"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// Handle 'confirm-train' button interaction
func (h *ButtonsHandler) onConfirmTrain(s *bot.Client, i *events.ComponentInteractionCreate) {
	ctx := context.Background()
	// Defer the update
	s.Rest.CreateInteractionResponse(i.ComponentInteraction.ID(), i.ComponentInteraction.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeDeferredCreateMessage,
	})

	chainDoc, err := h.ChainsService.GetChainConf(ctx, i.GuildID().String())
	if err != nil {
		logger.Errorf("Failed to fetch chainDoc for guild %s: %v", i.GuildID, err)
		return
	}

	// redundant check
	if chainDoc.TrainedAt != nil && chainDoc.TrainedAt.Before(time.Now()) {
		s.Rest.CreateMessage(i.Channel().ID(), discord.NewMessageCreate().WithContent("Training already completed for this server."))
		return
	}

	// Start the training process
	// Send confirmation message
	content := fmt.Sprintf("%s Started Fetching messages.\nI  will send a message when I'm done.\nEstimated Time: `1 Minute per every 5000 Messages in the Server`\nThis might take a while..", i.User().Mention())
	s.Rest.CreateMessage(i.Channel().ID(), discord.NewMessageCreate().WithContent(content))
	s.Rest.DeleteInteractionResponse(s.ApplicationID, i.ComponentInteraction.Token())

	// Update chain status
	now := time.Now()
	chainDoc.TrainedAt = &now
	if _, err = h.ChainsService.UpdateChainMeta(ctx, i.GuildID().String(), map[string]any{"trained_at": now}); err != nil {
		logger.Errorf("Failed to update chain document for guild %s: %v", i.GuildID, err)
		return
	}

	// background job
	go func(s *bot.Client, i *events.ComponentInteractionCreate) {
		startTime := time.Now()
		n, err := h.DataFetchService.FetchAllGuildMessages(i.GuildID().String())
		if err != nil {
			logger.Errorf("Failed to fetch messages for guild %s: %v", i.GuildID, err)
			// Revert chain status
			chainDoc.TrainedAt = nil
			if _, err = h.ChainsService.UpdateChainMeta(ctx, i.GuildID().String(), map[string]any{"trained_at": nil}); err != nil {
				logger.Errorf("Failed to update chain document for guild %s: %v", i.GuildID, err)
			}
			return
		}

		// Send completion message
		finalMsg := fmt.Sprintf("%s Finished Fetching messages.\nMessages fetched: `%s`\nTime elapsed: `%s`\nMessages/Second: `%s`",
			i.User().Mention(),
			utils.FormatNumber(float64(n)),
			time.Since(startTime).String(),
			utils.FormatNumber(
				float64(n)/(time.Since(startTime).Seconds()),
			),
		)

		if _, err := s.Rest.CreateMessage(i.Channel().ID(), discord.NewMessageCreate().WithContent(finalMsg)); err != nil {
			logger.Errorf("Failed to send training finished msg: %v", err)
		}
	}(s, i)
}
