package buttons

import (
	"rolando/cmd/idiscord/services"
	"rolando/internal/logger"
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
	h.Handlers["confirm-train-again"] = h.onConfirmTrainAgain
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
			logger.Errorf("Failed to fetch guild '%s' for button interaction: %v", i.GuildID, err)
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
		logger.Errorf("Failed to determine user for button interaction in '%s'", where)
		return
	}
	buttonId := i.MessageComponentData().CustomID

	startTime := time.Now()
	logger.Infof("from '%s' in '%s': btn:%s", who, where, buttonId)
	// Check if there's a handler for the button ID
	if handler, exists := h.Handlers[buttonId]; exists {
		handler(s, i) // Call the function bound to the button ID
	}
	logger.Infof("btn:%s handler from '%s' in '%s' completed in %s", buttonId, who, where, time.Since(startTime).String())
}
