package handlers

import (
	"fmt"
	"rolando/cmd/log"
	"rolando/cmd/services"
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
}

func (h *EventsHandler) OnEventCreate(s *discordgo.Session, e *discordgo.Event) {
	if handler, ok := h.Handlers[e.Type]; ok {
		handler(s, e)
	}
}

func (h *EventsHandler) onGuildUpdate(s *discordgo.Session, e *discordgo.Event) {
	guildUpdate, ok := e.Struct.(*discordgo.GuildUpdate)
	if !ok {
		return
	}
	oldGuild, err := s.State.Guild(guildUpdate.ID)
	if err != nil {
		log.Log.Errorf("Failed to fetch guild for guild update event: %v", err)
		return
	}
	h.ChainsService.UpdateChainMeta(oldGuild.ID, map[string]interface{}{"name": guildUpdate.Name})
	log.Log.Infof("Guild %s updated: %s -> %s", guildUpdate.ID, oldGuild.Name, guildUpdate.Name)
}

func (h *EventsHandler) onGuildCreate(s *discordgo.Session, e *discordgo.Event) {
	guildCreate, ok := e.Struct.(*discordgo.GuildCreate)
	if !ok {
		return
	}
	log.Log.Infof("Joined guild %s", guildCreate.Name)
	h.ChainsService.CreateChain(guildCreate.ID, guildCreate.Name)
	s.ChannelMessage(guildCreate.SystemChannelID, fmt.Sprintf("Hello %s.\nperform the command `/train` to use all the server's messages as training data", guildCreate.Name))
	UpdatePresence(s)
}

func (h *EventsHandler) onGuildDelete(s *discordgo.Session, e *discordgo.Event) {
	guildDelete, ok := e.Struct.(*discordgo.GuildDelete)
	if !ok {
		return
	}
	log.Log.Infof("Left guild %s", guildDelete.Name)
	h.ChainsService.DeleteChain(guildDelete.ID)
	UpdatePresence(s)
}

// ---------------------- Helpers ---------------------------

func UpdatePresence(ds *discordgo.Session) {
	log.Log.Infoln("Updating presence...")
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
		log.Log.Fatalf("error setting bot presence: %v", err)
	}
	log.Log.Infoln("Presence updated")
}
