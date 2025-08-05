package commands

import (
	"reflect"
	"rolando/internal/config"
	"rolando/internal/logger"
	"rolando/internal/services"
	"slices"

	"github.com/bwmarrin/discordgo"
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
	var CohesionMinValue float64 = 2.0
	handler.registerCommands([]SlashCommand{
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "train",
				Description: "Fetches all available messages in the server to be used as training data",
			},
			Handler: handler.withAdminPermission(handler.trainCommand),
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
			Handler: handler.withAdminPermission(handler.togglePingsCommand),
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
				Name:        "cohesion",
				Description: "View or set the cohesion value; A higher value makes sentences more coherent",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						MinValue:    &CohesionMinValue,
						MaxValue:    255,
						Name:        "value",
						Description: "the value to set, must be at least 2 (leave empty to view)",
						Required:    false,
					},
				},
			},
			Handler: handler.cohesionCommand,
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
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcJoinCommand),
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "vc-leave",
				Description: "Leaves the voice channel you are currently in",
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcLeaveCommand),
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "vc-speak",
				Description: "Speaks a message in the VC you are in, and then leaves",
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcSpeakCommand),
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
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcLanguageCommand),
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
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcJoinRateCommand),
		},
	})

	return handler
}

// registerCommands registers only new or modified slash commands
func (h *SlashCommandsHandler) registerCommands(commands []SlashCommand) {
	// Fetch currently registered commands from Discord
	registeredCommands, err := h.Client.ApplicationCommands(h.Client.State.User.ID, "")
	if err != nil {
		logger.Errorf("Failed to fetch registered commands: %v", err)
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
			logger.Warnf("Removing outdated slash command: %s", cmd.Name)
			h.Client.ApplicationCommandDelete(h.Client.State.User.ID, "", cmd.ID)
		}
	}

	// Iterate through new commands and check if they are already registered
	for _, cmd := range commands {
		if existingCmd, exists := registeredCommandsMap[cmd.Command.Name]; exists {
			// Compare if the new command differs in some way (e.g., updated description or options)
			if !shouldRefreshCommand(*existingCmd, *cmd.Command) {
				logger.Infof("Updating slash command: %s", cmd.Command.Name)
				for _, guildId := range cmd.GuildIds {
					h.Client.ApplicationCommandDelete(h.Client.State.User.ID, guildId, existingCmd.ID)
					_, err := h.Client.ApplicationCommandCreate(h.Client.State.User.ID, guildId, cmd.Command)
					if err != nil {
						logger.Errorf("Failed to register slash command: %v", err)
					}
				}
				// If no guild IDs, create globally
				if len(cmd.GuildIds) == 0 {
					h.Client.ApplicationCommandDelete(h.Client.State.User.ID, "", existingCmd.ID)
					_, err := h.Client.ApplicationCommandCreate(h.Client.State.User.ID, "", cmd.Command)
					if err != nil {
						logger.Errorf("Failed to register slash command: %v", err)
					}
				}
			}
		} else {
			// Register the new command
			logger.Infof("Registering slash command: %s", cmd.Command.Name)
			for _, guildId := range cmd.GuildIds {
				_, err := h.Client.ApplicationCommandCreate(h.Client.State.User.ID, guildId, cmd.Command)
				if err != nil {
					logger.Errorf("Failed to register slash command: %v", err)
				}
			}
			// If no guild IDs, create globally
			if len(cmd.GuildIds) == 0 {
				_, err := h.Client.ApplicationCommandCreate(h.Client.State.User.ID, "", cmd.Command)
				if err != nil {
					logger.Errorf("Failed to register slash command: %v", err)
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
			logger.Errorf("Failed to fetch guild '%s' for command interaction: %v", i.GuildID, err)
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
		logger.Errorf("Failed to determine user for command interaction in '%s'", where)
		return
	}

	logger.Infof("from '%s' in '%s': /%s", who, where, commandName)
	if handler, exists := h.Commands[commandName]; exists {
		handler(s, i) // Call the function bound to this command
	}
}

// compares two commands to check if they are identical in the significant fields
func shouldRefreshCommand(cached, loaded discordgo.ApplicationCommand) bool {
	// For simplicity, compare the name, description, and options. extend this logic if necessary.
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
