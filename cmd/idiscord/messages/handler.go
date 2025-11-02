package messages

import (
	"rolando/cmd/idiscord/services"

	discord "github.com/bwmarrin/discordgo"
)

type MessageHandler struct {
	Client        *discord.Session
	ChainsService *services.ChainsService
}

// Constructor function for MessageHandler
func NewMessageHandler(client *discord.Session, chainsService *services.ChainsService) *MessageHandler {
	return &MessageHandler{
		Client:        client,
		ChainsService: chainsService,
	}
}
