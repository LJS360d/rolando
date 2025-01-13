package handlers

import (
	"fmt"
	"rolando/cmd/log"
	"rolando/cmd/services"
	"rolando/cmd/utils"
	"time"

	"github.com/bwmarrin/discordgo"
)

type ButtonsHandler struct {
	Client           *discordgo.Session
	ChainsService    *services.ChainsService
	DataFetchService *services.DataFetchService
	Handlers         map[string]ButtonHandler
}

type ButtonHandler func(s *discordgo.Session, i *discordgo.InteractionCreate)

// Constructor for ButtonsHandler
func NewButtonsHandler(client *discordgo.Session, dataFetchService *services.DataFetchService, chainsService *services.ChainsService) *ButtonsHandler {
	handler := &ButtonsHandler{
		Client:           client,
		ChainsService:    chainsService,
		DataFetchService: dataFetchService,
		Handlers:         make(map[string]ButtonHandler),
	}

	// Register button handlers
	handler.registerButtons()

	return handler
}

// Register button handlers in the map
func (h *ButtonsHandler) registerButtons() {
	h.Handlers["confirm-train"] = h.onConfirmTrain
}

// Entry point for handling button interactions
func (h *ButtonsHandler) OnButtonInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	var where string
	if i.GuildID == "" {
		where = "DMs"
	} else {
		if guild, err := h.Client.Guild(i.GuildID); err != nil {
			log.Log.Errorf("Failed to fetch guild '%s' for button interaction: %v", i.GuildID, err)
			return
		} else {
			where = guild.Name
		}
	}

	var who string
	if i.User != nil {
		who = i.User.Username
	} else if i.Member != nil && i.Member.User != nil {
		who = i.Member.User.Username
	} else {
		log.Log.Errorf("Failed to determine user for button interaction in '%s'", where)
		return
	}
	buttonId := i.MessageComponentData().CustomID

	log.Log.Infof("from '%s' in '%s': btn:%s", who, where, buttonId)

	// Check if there's a handler for the button ID
	if handler, exists := h.Handlers[buttonId]; exists {
		handler(s, i) // Call the function bound to the button ID
	}
}

// Handle 'confirm-train' button interaction
func (h *ButtonsHandler) onConfirmTrain(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Defer the update
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Check if training is already completed
	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID)
	if err != nil {
		log.Log.Errorf("Failed to fetch chainDoc for guild %s: %v", i.GuildID, err)
		return
	}

	if chainDoc.Trained {
		s.ChannelMessageSend(i.ChannelID, "Training already completed for this server.")
		return
	}

	var userId string
	if i.User != nil {
		userId = i.User.ID
	} else if i.Member != nil && i.Member.User != nil {
		userId = i.Member.User.ID
	} else {
		log.Log.Errorf("Failed to determine user ID for interaction in '%s'", chainDoc.Name)
		return
	}

	// Start the training process
	// Send confirmation message
	content := fmt.Sprintf("<@%s> Started Fetching messages.\nI  will send a message when I'm done.\nEstimated Time: `1 Minute per every 5000 Messages in the Server`\nThis might take a while..", userId)
	s.ChannelMessageSend(i.ChannelID, content)
	s.InteractionResponseDelete(i.Interaction)

	// Update chain status
	chainDoc.Trained = true
	if _, err = h.ChainsService.UpdateChainMeta(i.GuildID, map[string]any{"trained": true}); err != nil {
		log.Log.Errorf("Failed to update chain document for guild %s: %v", i.GuildID, err)
		return
	}
	go func() {
		startTime := time.Now()
		messages, err := h.DataFetchService.FetchAllGuildMessages(i.GuildID)
		if err != nil {
			log.Log.Errorf("Failed to fetch messages for guild %s: %v", i.GuildID, err)
			// Revert chain status
			chainDoc.Trained = false
			if _, err = h.ChainsService.UpdateChainMeta(i.GuildID, map[string]any{"trained": false}); err != nil {
				log.Log.Errorf("Failed to update chain document for guild %s: %v", i.GuildID, err)
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
