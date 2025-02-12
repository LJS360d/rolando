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
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/exp/slices"
)

type SlashCommandsHandler struct {
	Client        *discordgo.Session
	ChainsService *services.ChainsService
	Commands      map[string]SlashCommandHandler
}

type SlashCommandHandler func(s *discordgo.Session, i *discordgo.InteractionCreate)

type SlashCommand struct {
	Command  *discordgo.ApplicationCommand
	Handler  SlashCommandHandler
	GuildIds []string // Optional guild IDs to restrict the command to specific guilds
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
				Description: "Returns analytics about the bot in this server",
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
				Description: "View or set the reply rate",
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
				Description: "View which channels can be accessed for training data",
			},
			Handler: handler.channelsCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "src",
				Description: "Provides the URL to the repository with bot source code",
			},
			Handler: handler.srcCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "vc-join",
				Description: "Joins the voice channel you are currently in",
			},
			Handler: handler.vcJoinCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "vc-leave",
				Description: "Leaves the voice channel you are currently in",
			},
			Handler: handler.vcLeaveCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "vc-speak",
				Description: "Speaks a message in the VC you are in, and then leaves",
			},
			Handler: handler.vcSpeakCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "vc-language",
				Description: "View or set the language to use when generating audio",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type: discordgo.ApplicationCommandOptionInteger,
						Name: "language",
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{
								Name:  "English",
								Value: 0,
							},
							{
								Name:  "Italian",
								Value: 1,
							},
							{
								Name:  "German",
								Value: 2,
							},
							{
								Name:  "Spanish",
								Value: 3,
							},
							{
								Name:  "Chinese",
								Value: 4,
							},
						},
						Description: "the language to set (leave empty to view)",
						Required:    false,
					},
				},
			},
			Handler: handler.vcLanguageCommand,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "vc-joinrate",
				Description: "View or set the VC random join rate",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "rate",
						Description: "the rate to set (1/rate) (leave empty to view)",
						Required:    false,
					},
				},
			},
			Handler: handler.vcJoinRateCommand,
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
		isOutdated := !slices.ContainsFunc(commands, func(c SlashCommand) bool {
			return shouldRefreshCommand(*cmd, *c.Command)
		})
		if isOutdated {
			log.Log.Warnf("Removing outdated slash command: %s", cmd.Name)
			h.Client.ApplicationCommandDelete(h.Client.State.User.ID, "", cmd.ID)
		}
	}

	// Iterate through new commands and check if they are already registered
	for _, cmd := range commands {
		if existingCmd, exists := registeredCommandsMap[cmd.Command.Name]; exists {
			// Compare if the new command differs in some way (e.g., updated description or options)
			if !shouldRefreshCommand(*existingCmd, *cmd.Command) {
				log.Log.Infof("Updating slash command: %s", cmd.Command.Name)
				for _, guildId := range cmd.GuildIds {
					h.Client.ApplicationCommandDelete(h.Client.State.User.ID, guildId, existingCmd.ID)
					_, err := h.Client.ApplicationCommandCreate(h.Client.State.User.ID, guildId, cmd.Command)
					if err != nil {
						log.Log.Errorf("Failed to register slash command: %v", err)
					}
				}
				// If no guild IDs, create globally
				if len(cmd.GuildIds) == 0 {
					h.Client.ApplicationCommandDelete(h.Client.State.User.ID, "", existingCmd.ID)
					_, err := h.Client.ApplicationCommandCreate(h.Client.State.User.ID, "", cmd.Command)
					if err != nil {
						log.Log.Errorf("Failed to register slash command: %v", err)
					}
				}
			}
		} else {
			// Register the new command
			log.Log.Infof("Registering slash command: %s", cmd.Command.Name)
			for _, guildId := range cmd.GuildIds {
				_, err := h.Client.ApplicationCommandCreate(h.Client.State.User.ID, guildId, cmd.Command)
				if err != nil {
					log.Log.Errorf("Failed to register slash command: %v", err)
				}
			}
			// If no guild IDs, create globally
			if len(cmd.GuildIds) == 0 {
				_, err := h.Client.ApplicationCommandCreate(h.Client.State.User.ID, "", cmd.Command)
				if err != nil {
					log.Log.Errorf("Failed to register slash command: %v", err)
				}
			}
		}
		h.Commands[cmd.Command.Name] = cmd.Handler
	}
}

// Entry point for handling slash command interactions
func (h *SlashCommandsHandler) OnSlashCommandInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	commandName := i.ApplicationCommandData().Name

	var where string
	if i.GuildID == "" {
		where = "DMs"
	} else {
		if guild, err := h.Client.Guild(i.GuildID); err != nil {
			log.Log.Errorf("Failed to fetch guild '%s' for command interaction: %v", i.GuildID, err)
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
		log.Log.Errorf("Failed to determine user for command interaction in '%s'", where)
		return
	}

	log.Log.Infof("from '%s' in '%s': /%s", who, where, commandName)
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

	// Create an action row with the buttons
	actionRow := &discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			confirmButton,
		},
	}

	// Send the reply with buttons
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: `Are you sure you want to use **ALL SERVER MESSAGES** as training data for me?
This will fetch data in all accessible text channels,
you can use the` + "`/channels`" + ` command to see which are accessible.
If you wish to exclude specific channels, revoke my typing permissions in those channels.
`,
			Components: []discordgo.MessageComponent{*actionRow},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		log.Log.Errorf("Failed to send reply to /train command: %v", err)
	}
}

// implementation of /gif command
func (h *SlashCommandsHandler) gifCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		return
	}
	gif, err := chain.MediaStorage.GetMedia("gif")
	if err != nil || gif == "" {
		gif = "No valid gif found."
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &gif,
	})
}

// implementation of /image command
func (h *SlashCommandsHandler) imageCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		return
	}
	image, err := chain.MediaStorage.GetMedia("image")
	if err != nil || image == "" {
		image = "No valid image found."
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &image,
	})
}

// implementation of /video command
func (h *SlashCommandsHandler) videoCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		return
	}
	video, err := chain.MediaStorage.GetMedia("video")
	if err != nil || video == "" {
		video = "No valid video found."
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &video,
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
	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID)
	if err != nil {
		log.Log.Errorf("Failed to fetch chain document for guild %s: %v", i.GuildID, err)
		return
	}
	analytics := model.NewMarkovChainAnalyzer(chain).GetRawAnalytics()
	// Constructing the embed
	embed := &discordgo.MessageEmbed{
		Title:       "Analytics",
		Description: "**Complexity Score**: indicates how *smart* the bot is.\nA higher value means smarter",
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
	if !h.checkAdmin(i) {
		return
	}

	guildID := i.GuildID
	chain, err := h.ChainsService.GetChain(guildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if _, err := h.ChainsService.UpdateChainMeta(guildID, map[string]interface{}{"pings": !chain.Pings}); err != nil {
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
	chainDoc, err := h.ChainsService.GetChainDocument(guildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	var ratePercent float64
	if rate != nil {
		if !h.checkAdmin(i, "You are not authorized to change the reply rate.") {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(chainDoc.ID, map[string]interface{}{"reply_rate": *rate}); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Failed to update reply rate.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		if *rate == 0 {
			ratePercent = 0
		} else {
			ratePercent = float64(1 / float64(*rate) * 100)
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Set reply rate to `" + strconv.Itoa(*rate) + "` (" + strconv.FormatFloat(ratePercent, 'f', 2, 64) + "%)",
			},
		})
		return
	}

	if chainDoc.ReplyRate == 0 {
		ratePercent = 0
	} else {
		ratePercent = float64(1 / float64(chainDoc.ReplyRate) * 100)
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Current reply rate is `" + strconv.Itoa(chainDoc.ReplyRate) + "` (" + strconv.FormatFloat(ratePercent, 'f', 2, 64) + "%)",
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
				Flags:   discordgo.MessageFlagsEphemeral,
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
				Flags:   discordgo.MessageFlagsEphemeral,
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
				Flags:   discordgo.MessageFlagsEphemeral,
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
				Flags:   discordgo.MessageFlagsEphemeral,
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
				Flags:   discordgo.MessageFlagsEphemeral,
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
				Flags:   discordgo.MessageFlagsEphemeral,
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

	responseBuilder := &strings.Builder{}
	responseBuilder.WriteString(fmt.Sprintf("Channels the bot has access to are marked with: %s\nWhile channels with no access are marked with: %s\nMake a channel accessible by giving %s these permissions:\n%s %s %s\n\n",
		":green_circle:",
		":red_circle:",
		"**ALL**",
		"`View Channel`", "`Send Messages`", "`Read Message History`",
	))

	for _, ch := range channels {
		hasAccess := utils.HasGuildTextChannelAccess(s, s.State.User.ID, ch)
		fmt.Fprintf(responseBuilder, "%s <#%s>\n", accessEmote(hasAccess), ch.ID)
	}

	responseText := responseBuilder.String()
	if len(responseText) == 0 {
		responseText = "No available channels to display."
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: responseText,
		},
	})
}

// implementation of /src command
func (h *SlashCommandsHandler) srcCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	repoURL := "https://github.com/LJS360d/rolando"
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

// implementation of /vc join command
func (h *SlashCommandsHandler) vcJoinCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// step 0: defer a response to the interaction
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	// step 1: get the user's voice state
	voiceState, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
	if err != nil {
		content := "You must be in a voice channel to use this command."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}

	// step 2: join the voice channel
	vc, err := s.ChannelVoiceJoin(i.GuildID, voiceState.ChannelID, false, false)
	if err != nil || !vc.Ready {
		channel, _ := s.State.Channel(voiceState.ChannelID)
		content := fmt.Sprintf("Failed to join the voice channel: %s", channel.Name)
		log.Log.Errorln(content, err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}
	voiceChannel, _ := s.State.Channel(voiceState.ChannelID)
	// step 3: having joined the vc, respond to the interaction
	content := fmt.Sprintf("Joined the voice channel '%s', i'll speak sometimes", voiceChannel.Name)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})

	// step 4: generate a static TTS audio and stream it to the vc
	chainDoc, _ := h.ChainsService.GetChainDocument(voiceState.GuildID)
	chain, _ := h.ChainsService.GetChain(chainDoc.ID)
	var ttsMutex sync.Mutex
	d, err := utils.GenerateTTSDecoder("i am here", chainDoc.TTSLanguage)
	if err != nil {
		log.Log.Errorf("Failed to generate TTS decoder: %v", err)
		return
	}
	if err := utils.StreamAudioDecoder(vc, d); err != nil {
		log.Log.Errorf("Failed to stream audio in '%s' in '%s': %v", voiceChannel.Name, chainDoc.Name, err)
	}

	// step 5: start listening in the vc
	leaveChan := make(chan struct{})
	var cleanupHandler func()
	go func() {
		defer close(leaveChan)
		cleanupHandler = s.AddHandler(func(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
			if vs.GuildID != i.GuildID {
				return // Not the guild we're in
			}
			if vs.UserID == s.State.User.ID {
				return // the bot leaving
			}
			currentUsers, _ := getVCUsers(s, i.GuildID, voiceState.ChannelID)
			if len(currentUsers) < 1 { // All other users have left the vc
				leaveChan <- struct{}{} // Signal to leave the vc
			}
		})
		for range vc.OpusRecv {
			random := utils.GetRandom(1, 1000)
			if random != 1 {
				continue
			}
			go func() {
				ttsMutex.Lock()
				defer ttsMutex.Unlock()
				d, err := utils.GenerateTTSDecoder(chain.TalkOnlyText(10), chainDoc.TTSLanguage)
				if err != nil {
					log.Log.Errorf("Failed to generate random TTS decoder in '%s' in '%s': %v", voiceChannel.Name, chainDoc.Name, err)
					return
				}
				if err := utils.StreamAudioDecoder(vc, d); err != nil {
					log.Log.Errorf("Failed to stream random TTS audio in '%s' in '%s': %v", voiceChannel.Name, chainDoc.Name, err)
				}
			}()
		}
	}()

	// cleanup: leave the vc when receiving the signal or after 8 hours
	select {
	case <-leaveChan:
		log.Log.Infof("Leaving vc '%s' in '%s'", voiceChannel.Name, chainDoc.Name)
		cleanupHandler()
		vc.Disconnect()
		vc.Close()
		break
	case <-time.After(8 * time.Hour): // timeout after 8 hours
		log.Log.Infof("VC Timeout in '%s' in '%s'", voiceChannel.Name, chainDoc.Name)
		cleanupHandler()
		vc.Disconnect()
		vc.Close()
		break
	}
}

// implementation of /vc speak command
func (h *SlashCommandsHandler) vcSpeakCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// step 1: get the user's voice state
	voiceState, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must be in a voice channel to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	var vc *discordgo.VoiceConnection
	vc, exists := s.VoiceConnections[voiceState.GuildID]
	if !exists {
		content := "Joining Voice Channel..."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		// join the voice channel
		vc, err = s.ChannelVoiceJoin(i.GuildID, voiceState.ChannelID, false, false)
		if err != nil || !vc.Ready {
			content := "You must be in a voice channel to use this command."
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})
			return
		}
	}

	chainDoc, err := h.ChainsService.GetChainDocument(voiceState.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	chain, _ := h.ChainsService.GetChain(chainDoc.ID)
	content := chain.TalkOnlyText(100)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
	d, err := utils.GenerateTTSDecoder(content, chainDoc.TTSLanguage)
	if err != nil {
		log.Log.Errorf("Failed to generate TTS decoder: %v", err)
		return
	}
	if err := utils.StreamAudioDecoder(vc, d); err != nil {
		log.Log.Errorf("Failed to stream audio: %v", err)
	}
	err = vc.Disconnect()
	if err != nil {
		log.Log.Errorf("Failed to disconnect from voice channel: %v", err)
	}
	vc.Close()
}

// implementation of /vc leave command
func (h *SlashCommandsHandler) vcLeaveCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	vc, exists := s.VoiceConnections[i.GuildID]
	var res string
	if !exists {
		res = "I am not connected to a voice channel."
	} else {
		res = "I am leaving the voice channel"
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: res,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	chainDoc, _ := h.ChainsService.GetChainDocument(i.GuildID)
	d, err := utils.GenerateTTSDecoder("bye bye", chainDoc.TTSLanguage)
	if err != nil {
		log.Log.Errorf("Failed to generate TTS decoder: %v", err)
		return
	}
	if err := utils.StreamAudioDecoder(vc, d); err != nil {
		log.Log.Errorf("Failed to stream audio: %v", err)
	} else {
		log.Log.Infof("Spoke Bye Bye message in vc, leaving...")
	}
	err = vc.Disconnect()
	if err != nil {
		log.Log.Errorf("Failed to disconnect from voice channel: %v", err)
	}
	vc.Close()
}

var langs = []string{"en", "it", "de", "es", "zh"}

// implementation of /vc language command
func (h *SlashCommandsHandler) vcLanguageCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var lang string
	for _, option := range i.ApplicationCommandData().Options {
		if option.Name == "language" && option.Type == discordgo.ApplicationCommandOptionInteger {
			lang = langs[option.IntValue()]
			break
		}
	}

	chainId := i.GuildID
	chainDoc, err := h.ChainsService.GetChainDocument(chainId)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if lang != "" {
		if !h.checkAdmin(i) {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(chainDoc.ID, map[string]interface{}{"tts_language": lang}); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Failed to update tts language.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Set language to use in vc to `" + lang + "`",
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Current language is `" + chainDoc.TTSLanguage + "`",
		},
	})
}

// implementation of /vc joinrate command
func (h *SlashCommandsHandler) vcJoinRateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	chainDoc, err := h.ChainsService.GetChainDocument(guildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	var ratePercent float64
	if rate != nil {
		if !h.checkAdmin(i, "You are not authorized to change the VC join rate.") {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(chainDoc.ID, map[string]interface{}{"vc_join_rate": *rate}); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Failed to update VC join rate.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		if *rate == 0 {
			ratePercent = 0
		} else {
			ratePercent = float64(1 / float64(*rate) * 100)
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Set VC join rate to `" + strconv.Itoa(*rate) + "` (" + strconv.FormatFloat(ratePercent, 'f', 2, 64) + "%)",
			},
		})
		return
	}

	if chainDoc.VcJoinRate == 0 {
		ratePercent = 0
	} else {
		ratePercent = float64(1 / float64(chainDoc.VcJoinRate) * 100)
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Current VC join rate is `" + strconv.Itoa(chainDoc.VcJoinRate) + "` (" + strconv.FormatFloat(ratePercent, 'f', 2, 64) + "%)",
		},
	})
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

func getVCUsers(s *discordgo.Session, guildID, channelID string) ([]*discordgo.Member, error) {
	guild, err := s.Guild(guildID)
	if err != nil {
		return nil, err
	}

	var users []*discordgo.Member
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID == channelID {
			for _, member := range guild.Members {
				if member.User.ID == vs.UserID {
					users = append(users, member)
					break
				}
			}
		}
	}
	return users, nil
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
	for i, option := range loaded.Options {
		if len(option.Choices) != 0 {
			// Compare each choice name and value
			for j, choice := range option.Choices {
				cachedChoice := cached.Options[i].Choices[j]
				if choice.Name != cachedChoice.Name || float64(choice.Value.(int)) != cachedChoice.Value.(float64) {
					return false
				}
			}
		} else if !reflect.DeepEqual(option, cached.Options[i]) {
			return false
		}
	}
	return true
}
