package handlers

import (
	"fmt"
	"reflect"
	"rolando/cmd/log"
	"rolando/cmd/model"
	"rolando/cmd/services"
	"rolando/cmd/utils"
	"rolando/config"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type SlashCommandsHandler struct {
	Client        *discordgo.Session
	ChainsService *services.ChainsService
	Commands      map[string]SlashCommandHandler
}

type SlashCommandHandler func(s *discordgo.Session, i *discordgo.InteractionCreate)

type SlashCommand struct {
	Command *discordgo.ApplicationCommand
	Handler SlashCommandHandler
}

// Constructor function for SlashCommandsHandler
func NewSlashCommandsHandler(
	client *discordgo.Session,
	chainsService *services.ChainsService,
) *SlashCommandsHandler {
	handler := &SlashCommandsHandler{
		Client:        client,
		ChainsService: chainsService,
		Commands:      make(map[string]SlashCommandHandler),
	}

	// Initialize commands
	handler.registerCommands([]SlashCommand{
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "train",
				Description: "Fetches all available messages in the server to be used as training data",
			},
			Handler: handler.trainCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "gif",
				Description: "Returns a gif from the ones it knows",
			},
			Handler: handler.gifCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "image",
				Description: "Returns an image from the ones it knows",
			},
			Handler: handler.imageCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "video",
				Description: "Returns a video from the ones it knows",
			},
			Handler: handler.videoCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "analytics",
				Description: "Returns analytics about the bot",
			},
			Handler: handler.analyticsCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "togglepings",
				Description: "Toggles wether pings are enabled or not",
			},
			Handler: handler.togglePingsCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "replyrate",
				Description: "View or set the reply rate for the bot",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "rate",
						Description: "the rate to set (leave empty to view)",
						Required:    false,
					},
				},
			},
			Handler: handler.replyRateCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "opinion",
				Description: "Generates a random opinion based on the provided seed",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "about",
						Description: "The seed of the message",
						Required:    true,
					},
				},
			},
			Handler: handler.opinionCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "wipe",
				Description: "Deletes the given argument `data` from the training data",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "data",
						Description: "The data to be deleted",
						Required:    true,
					},
				},
			},
			Handler: handler.wipeCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "channels",
				Description: "View which channels are being used by the bot",
			},
			Handler: handler.channelsCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "src",
				Description: "Provides the URL to the repository with bot source code.",
			},
			Handler: handler.srcCommand,
		},
	})

	return handler
}

// registerCommands registers only new or modified slash commands
func (h *SlashCommandsHandler) registerCommands(commands []SlashCommand) {
	// Fetch currently registered commands from Discord
	registeredCommands, err := h.Client.ApplicationCommands(h.Client.State.User.ID, "")
	if err != nil {
		log.Log.Errorf("Failed to fetch registered commands: %v", err)
		registeredCommands = []*discordgo.ApplicationCommand{}
	}

	// Create a map of registered commands for fast lookup
	registeredCommandsMap := make(map[string]*discordgo.ApplicationCommand)
	for _, cmd := range registeredCommands {
		registeredCommandsMap[cmd.Name] = cmd
	}

	// Iterate through new commands and check if they are already registered
	for _, cmd := range commands {
		if existingCmd, exists := registeredCommandsMap[cmd.Command.Name]; exists {
			// Compare if the new command differs in some way (e.g., updated description or options)
			if !shouldRefreshCommand(*existingCmd, *cmd.Command) {
				log.Log.Infof("Updating slash command: %s", cmd.Command.Name)
				h.Client.ApplicationCommandDelete(h.Client.State.User.ID, "", existingCmd.ID)
				h.Client.ApplicationCommandCreate(h.Client.State.User.ID, "", cmd.Command)
			}
		} else {
			// Register the new command
			log.Log.Infof("Registering slash command: %s", cmd.Command.Name)
			h.Client.ApplicationCommandCreate(h.Client.State.User.ID, "", cmd.Command)
		}
	}
}

// Entry point for handling slash command interactions
func (h *SlashCommandsHandler) OnSlashCommandInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	commandName := i.ApplicationCommandData().Name
	if handler, exists := h.Commands[commandName]; exists {
		handler(s, i) // Call the function bound to this command
	}
}

// ------------- Commands -------------

// implementation of /train command
func (h *SlashCommandsHandler) trainCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAdmin(i) {
		return
	}
	h.ChainsService.GetChain(i.GuildID)
	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID)
	if err != nil {
		log.Log.Errorf("Failed to fetch chain document for guild %s: %v", i.GuildID, err)
		return
	}
	if chainDoc.Trained {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Training already completed for this server.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Create buttons for confirmation and cancellation
	confirmButton := &discordgo.Button{
		Label:    "Confirm",
		Style:    discordgo.PrimaryButton,
		CustomID: "confirm-train",
	}
	cancelButton := &discordgo.Button{
		Label:    "Cancel",
		Style:    discordgo.DangerButton,
		CustomID: "cancel-train",
	}

	// Create an action row with the buttons
	actionRow := &discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			confirmButton,
			cancelButton,
		},
	}

	// Send the reply with buttons
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: `Are you sure you want to use **ALL SERVER MESSAGES** as training data for me?
This will fetch data in all accessible channels and delete all previous training data for this server.
If you wish to exclude specific channels, revoke my typing permissions in those channels.
`,
			Components: []discordgo.MessageComponent{*actionRow},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

// implementation of /gif command
func (h *SlashCommandsHandler) gifCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		return
	}
	gif, err := chain.MediaStorage.GetMedia("gif")
	if err != nil || gif == "" {
		gif = "No valid gif found."
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: gif,
		},
	})
}

// implementation of /image command
func (h *SlashCommandsHandler) imageCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		return
	}
	image, err := chain.MediaStorage.GetMedia("image")
	if err != nil || image == "" {
		image = "No valid image found."
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: image,
		},
	})
}

// implementation of /video command
func (h *SlashCommandsHandler) videoCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		return
	}
	video, err := chain.MediaStorage.GetMedia("video")
	if err != nil || video == "" {
		video = "No valid video found."
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: video,
		},
	})
}

// implementation of /analytics command
func (h *SlashCommandsHandler) analyticsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Fetch the chain data for the given guild
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		log.Log.Errorf("Failed to fetch chain for guild %s: %v", i.GuildID, err)
		return
	}
	analytics := model.NewMarkovChainAnalyzer(chain).GetRawAnalytics()
	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID)
	if err != nil {
		log.Log.Errorf("Failed to fetch chain document for guild %s: %v", i.GuildID, err)
		return
	}
	// Constructing the embed
	embed := &discordgo.MessageEmbed{
		Title:       "Analytics",
		Description: "",
		Color:       0xFFD700, // Gold color
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Complexity Score",
				Value:  fmt.Sprintf("```%d```", analytics.ComplexityScore),
				Inline: true,
			},
			{
				Name:   "Vocabulary",
				Value:  fmt.Sprintf("```%d words```", analytics.Words),
				Inline: true,
			},
			{
				Name:   "\t", // Empty field for spacing
				Value:  "\t",
				Inline: false,
			},
			{
				Name:   "Gifs",
				Value:  fmt.Sprintf("```%d```", analytics.Gifs),
				Inline: true,
			},
			{
				Name:   "Videos",
				Value:  fmt.Sprintf("```%d```", analytics.Videos),
				Inline: true,
			},
			{
				Name:   "Images",
				Value:  fmt.Sprintf("```%d```", analytics.Images),
				Inline: true,
			},
			{
				Name:   "\t", // Empty field for spacing
				Value:  "\t",
				Inline: false,
			},
			{
				Name:   "Processed Messages",
				Value:  fmt.Sprintf("```%d```", analytics.Messages),
				Inline: true,
			},
			{
				Name:   "Size",
				Value:  fmt.Sprintf("```%s / %s```", utils.FormatBytes(analytics.Size), utils.FormatBytes(uint64(chainDoc.MaxSizeMb*1024*1024))),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    fmt.Sprintf("Version: %s", config.Version),
			IconURL: s.State.User.AvatarURL("256"),
		},
	}

	// Send the response with the embed
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		log.Log.Errorf("Failed to send analytics embed: %v", err)
	}
}

// implementation of /togglepings command
func (h *SlashCommandsHandler) togglePingsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAdmin(i, "You are not authorized to toggle pings.") {
		return
	}

	guildID := i.GuildID
	chain, err := h.ChainsService.GetChain(guildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
			},
		})
		return
	}

	chain.Pings = !chain.Pings
	if _, err := h.ChainsService.UpdateChainDocument(chain.ID, map[string]interface{}{"pings": chain.Pings}); err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to toggle pings state.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	state := "disabled"
	if chain.Pings {
		state = "enabled"
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Pings are now `" + state + "`",
		},
	})
}

// implementation of /replyrate command
func (h *SlashCommandsHandler) replyRateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var rate *int
	for _, option := range options {
		if option.Name == "rate" && option.Type == discordgo.ApplicationCommandOptionInteger {
			value := int(option.IntValue())
			rate = &value
			break
		}
	}

	guildID := i.GuildID
	chain, err := h.ChainsService.GetChain(guildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
			},
		})
		return
	}

	if rate != nil {
		if !h.checkAdmin(i, "You are not authorized to change the reply rate.") {
			return
		}
		chain.ReplyRate = *rate
		if _, err := h.ChainsService.UpdateChainDocument(chain.ID, map[string]interface{}{"replyRate": *rate}); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Failed to update reply rate.",
				},
			})
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Set reply rate to `" + strconv.Itoa(*rate) + "`",
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Current rate is `" + strconv.Itoa(chain.ReplyRate) + "`",
		},
	})
}

// implementation of /opinion command
func (h *SlashCommandsHandler) opinionCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var about string
	for _, option := range options {
		if option.Name == "about" && option.Type == discordgo.ApplicationCommandOptionString {
			about = option.StringValue()
			break
		}
	}

	if about == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must provide a word as the seed.",
			},
		})
		return
	}

	words := strings.Split(about, " ")
	seed := words[len(words)-1]

	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
			},
		})
		return
	}

	msg := chain.GenerateText(seed, utils.GetRandom(8, 40)) // Generate text with random length between 8 and 40
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
		},
	})
}

// implementation of /wipe command
func (h *SlashCommandsHandler) wipeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var data string
	for _, option := range options {
		if option.Name == "data" && option.Type == discordgo.ApplicationCommandOptionString {
			data = option.StringValue()
			break
		}
	}

	if data == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must provide the data to be erased.",
			},
		})
		return
	}

	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
			},
		})
		return
	}

	err = h.ChainsService.DeleteTextData(i.GuildID, data)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to delete the specified data.",
			},
		})
		return
	}

	chain.Delete(data)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Deleted `%s`", data),
		},
	})
}

// implementation of /channels command
func (h *SlashCommandsHandler) channelsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve guild information.",
			},
		})
		return
	}

	var channels []*discordgo.Channel
	for _, channel := range guild.Channels {
		if channel.Type != discordgo.ChannelTypeGuildVoice && channel.Type != discordgo.ChannelTypeGuildCategory {
			channels = append(channels, channel)
		}
	}

	accessEmote := func(hasAccess bool) string {
		if hasAccess {
			return ":green_circle:"
		}
		return ":red_circle:"
	}

	channelsPermissionMap := make([]map[string]string, 0)
	for _, ch := range channels {
		hasAccess := channelAccessCheck(s, ch.ID)
		channelsPermissionMap = append(channelsPermissionMap, map[string]string{
			"name":   ch.Name,
			"access": accessEmote(hasAccess),
		})
	}

	channelFields := make([]*discordgo.MessageEmbedField, 0)
	for _, cp := range channelsPermissionMap {
		channelFields = append(channelFields, &discordgo.MessageEmbedField{
			Name:   " ",
			Value:  fmt.Sprintf("%s #%s", cp["access"], cp["name"]),
			Inline: true,
		})
	}

	// Chunk fields into groups of 15
	chunkedFields := chunkFields(channelFields, 15)

	embeds := make([]*discordgo.MessageEmbed, 0)
	for _, fields := range chunkedFields {
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title:       "Available Channels",
			Description: channelsDescription(":green_circle:", ":red_circle:"),
			Color:       0xFFD700, // Gold color
			Fields:      append([]*discordgo.MessageEmbedField{{Name: "\t", Value: "\t"}}, fields...),
		})
	}

	paginateEmbeds(s, i, embeds)
}

// implementation of /src command
func (h *SlashCommandsHandler) srcCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	repoURL := "YOUR_REPO_URL_HERE"
	err := h.Client.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: repoURL,
		},
	})
	if err != nil {
		log.Log.Errorf("Failed to send repo URL response: %v", err)
	}
}

// ------------- Helpers -------------

func (h *SlashCommandsHandler) checkAdmin(i *discordgo.InteractionCreate, msg ...string) bool {
	for _, ownerID := range config.OwnerIDs {
		if i.Member.User.ID == ownerID {
			return true
		}
	}

	perms := i.Member.Permissions
	if perms&discordgo.PermissionAdministrator != 0 {
		return true
	}
	var content string
	if len(msg) > 0 {
		content = strings.Join(msg, "")
	} else {
		content = "You are not authorized to use this command."
	}
	h.Client.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	return false
}

func channelsDescription(hasAccess string, noAccess string) string {
	return fmt.Sprintf(
		`Channels the bot has access to are marked with: %s
While channels with no access are marked with: %s

Make a channel accessible by giving %s these permissions:
%s %s %s`,
		hasAccess,
		noAccess,
		"**ALL**",
		"`View Channel`", "`Send Messages`", "`Read Message History`",
	)
}

func channelAccessCheck(s *discordgo.Session, channelID string) bool {
	channel, err := s.State.Channel(channelID)
	if err != nil {
		return false
	}
	permissions, err := s.UserChannelPermissions(s.State.User.ID, channel.ID)
	if err != nil {
		return false
	}
	return permissions&discordgo.PermissionViewChannel != 0
}

func chunkFields(fields []*discordgo.MessageEmbedField, chunkSize int) [][]*discordgo.MessageEmbedField {
	var chunks [][]*discordgo.MessageEmbedField
	for i := 0; i < len(fields); i += chunkSize {
		end := i + chunkSize
		if end > len(fields) {
			end = len(fields)
		}
		chunks = append(chunks, fields[i:end])
	}
	return chunks
}

func paginateEmbeds(s *discordgo.Session, i *discordgo.InteractionCreate, embeds []*discordgo.MessageEmbed) {
	if len(embeds) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No channels available.",
			},
		})
		return
	}

	for _, embed := range embeds {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	}
}

// compares two commands to check if they are identical in the significant fields
func shouldRefreshCommand(cached, loaded discordgo.ApplicationCommand) bool {
	// For simplicity, compare the name, description, and options here. You can extend this logic if necessary.
	if cached.Name != loaded.Name {
		return false
	}
	if cached.Description != loaded.Description {
		return false
	}
	if len(cached.Options) != len(loaded.Options) {
		return false
	}
	// Compare command options, if any
	for i, option := range cached.Options {
		if !reflect.DeepEqual(option, loaded.Options[i]) {
			return false
		}
	}
	return true
}
