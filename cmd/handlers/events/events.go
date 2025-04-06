package events

import (
	"rolando/internal/logger"
	"rolando/internal/services"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

type EventsHandler struct {
	Client        *discordgo.Session
	ChainsService *services.ChainsService
	Handlers      map[string]EventHandler
}

type EventHandler func(s *discordgo.Session, e *discordgo.Event)

// Constructor for EventsHandler
func NewEventsHandler(client *discordgo.Session, chainsService *services.ChainsService) *EventsHandler {
	handler := &EventsHandler{
		Client:        client,
		ChainsService: chainsService,
		Handlers:      make(map[string]EventHandler),
	}

	// Register event handlers
	handler.registerEvents()

	return handler
}

func (h *EventsHandler) registerEvents() {
	h.Handlers["GUILD_UPDATE"] = h.onGuildUpdate
	h.Handlers["GUILD_CREATE"] = h.onGuildCreate
	h.Handlers["GUILD_DELETE"] = h.onGuildDelete
	h.Handlers["VOICE_STATE_UPDATE"] = h.onVoiceStateUpdate
	// Subscriptions Logs
	// h.Handlers["ENTITLEMENT_CREATE"] = h.onEntitlementCreate
	// h.Handlers["SUBSCRIPTION_CREATE"] = h.onSubscriptionCreate
	// h.Handlers["SUBSCRIPTION_UPDATE"] = h.onSubscriptionUpdate
}

func (h *EventsHandler) OnEventCreate(s *discordgo.Session, e *discordgo.Event) {
	if handler, ok := h.Handlers[e.Type]; ok {
		handler(s, e)
	}
}

// ---------------------- Helpers ---------------------------

func UpdatePresence(ds *discordgo.Session) {
	logger.Infoln("Updating presence...")
	err := ds.UpdateStatusComplex(discordgo.UpdateStatusData{
		Activities: []*discordgo.Activity{
			{
				Type: discordgo.ActivityTypeWatching,
				Name: strconv.Itoa(len(ds.State.Guilds)) + " servers",
			},
		},
		Status:    "online",
		AFK:       false,
		IdleSince: nil,
	})
	if err != nil {
		logger.Fatalf("error setting bot presence: %v", err)
	}
	logger.Infoln("Presence updated")
}
