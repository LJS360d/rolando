package messages

import (
	"rolando/internal/discord/services"

	"github.com/disgoorg/disgo/bot"
)

type MessageHandler struct {
	Client        *bot.Client
	ChainsService *services.ChainsService
}

// Constructor function for MessageHandler
func NewMessageHandler(client *bot.Client, chainsService *services.ChainsService) *MessageHandler {
	return &MessageHandler{
		Client:        client,
		ChainsService: chainsService,
	}
}
