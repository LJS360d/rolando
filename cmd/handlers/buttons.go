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
	h.Handlers["cancel-train"] = h.onCancelTrain
}

// Entry point for handling button interactions
func (h *ButtonsHandler) OnButtonInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	// Check if there's a handler for the button ID
	if handler, exists := h.Handlers[i.MessageComponentData().CustomID]; exists {
		handler(s, i) // Call the function bound to the button ID
	}
}

// Handle 'confirm-train' button interaction
func (h *ButtonsHandler) onConfirmTrain(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Defer the update
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	// Check if training is already completed
	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID)
	if err != nil {
		log.Log.Errorf("Failed to fetch chain for guild %s: %v", i.GuildID, err)
		return
	}

	if chainDoc.Trained {
		s.ChannelMessageSend(i.ChannelID, "Training already completed for this server.")
		return
	}

	// Start the training process
	// Send confirmation message
	s.ChannelMessageSend(i.ChannelID, fmt.Sprintf("<@%s> Started Fetching messages.\nI  will send a message when I'm done.\nEstimated Time: `1 Minute per every 5000 Messages in the Server`\nThis might take a while..", i.User.ID))

	go func() {
		startTime := time.Now()
		messages, err := h.DataFetchService.FetchAllGuildMessages(i.GuildID)
		if err != nil {
			log.Log.Errorf("Failed to fetch messages for guild %s: %v", i.GuildID, err)
			return
		}

		// Update chain status
		chainDoc.Trained = true
		_, err = h.ChainsService.UpdateChainMeta(i.GuildID, map[string]any{"trained": true})
		if err != nil {
			log.Log.Errorf("Failed to update chain document for guild %s: %v", i.GuildID, err)
			return
		}

		// Send completion message
		s.ChannelMessageSend(i.ChannelID, fmt.Sprintf("<@%s> Finished Fetching messages.\nMessages fetched: `%s`\nTime elapsed: `%s`\nMessages/Second: `%s`",
			i.User.ID,
			utils.FormatNumber(float64(len(messages))),
			time.Since(startTime).String(),
			utils.FormatNumber(float64(len(messages))/(time.Since(startTime).Seconds()))))
	}()
}

// Handle 'cancel-train' button interaction
func (h *ButtonsHandler) onCancelTrain(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Respond to cancel interaction
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Training process has been canceled.",
		},
	})
}
