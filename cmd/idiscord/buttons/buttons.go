package buttons

import (
	"rolando/cmd/idiscord/services"
	"rolando/internal/logger"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
)

type ButtonsHandler struct {
	Client           *bot.Client
	ChainsService    *services.ChainsService
	DataFetchService *services.DataFetchService
	Handlers         map[string]ButtonHandler
}

type ButtonHandler func(client *bot.Client, i *events.ComponentInteractionCreate)

// Constructor for ButtonsHandler
func NewButtonsHandler(client *bot.Client, dataFetchService *services.DataFetchService, chainsService *services.ChainsService) *ButtonsHandler {
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
func (h *ButtonsHandler) OnButtonInteraction(i *events.ComponentInteractionCreate) {

	var where string
	if i.GuildID() == nil {
		where = "DMs"
	} else {
		if guild, ok := h.Client.Caches.Guild(*i.GuildID()); !ok {
			logger.Errorf("Guild with id '%s' not found in cache", i.GuildID())
			return
		} else {
			where = guild.Name
		}
	}

	who := i.User().Username
	buttonId := i.Data.CustomID()

	startTime := time.Now()
	logger.Infof("from '%s' in '%s': btn:%s", who, where, buttonId)
	// Check if there's a handler for the button ID
	if handler, exists := h.Handlers[buttonId]; exists {
		handler(h.Client, i) // Call the function bound to the button ID
	}
	logger.Infof("btn:%s handler from '%s' in '%s' completed in %s", buttonId, who, where, time.Since(startTime).String())
}
