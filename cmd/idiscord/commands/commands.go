package commands

import (
	"rolando/cmd/idiscord/services"
	"rolando/internal/config"
	"rolando/internal/logger"
	"slices"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

type SlashCommandsHandler struct {
	Client        *bot.Client
	ChainsService *services.ChainsService
	Commands      map[string]SlashCommandHandler
}

type SlashCommandHandler func(client *bot.Client, event *events.ApplicationCommandInteractionCreate)

type SlashCommand struct {
	Command  discord.ApplicationCommandCreate
	Handler  SlashCommandHandler
	GuildIds []string // Optional guild IDs to restrict the command to specific guilds
}

// Constructor function for SlashCommandsHandler
func NewSlashCommandsHandler(
	client *bot.Client,
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
			Command: discord.SlashCommandCreate{
				Name:        "train",
				Description: "Fetches all available messages in the server to be used as training data",
			},
			Handler: handler.withAdminPermission(handler.trainCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "gif",
				Description: "Returns a gif from the ones it knows",
			},
			Handler: handler.gifCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "image",
				Description: "Returns an image from the ones it knows",
			},
			Handler: handler.imageCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "video",
				Description: "Returns a video from the ones it knows",
			},
			Handler: handler.videoCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "analytics",
				Description: "Returns analytics about the bot in this server",
			},
			Handler: handler.analyticsCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "togglepings",
				Description: "Toggles wether pings are enabled or not",
			},
			Handler: handler.withAdminPermission(handler.togglePingsCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "replyrate",
				Description: "View or set the reply rate",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionInt{
						Name:        "rate",
						Description: "the rate to set (leave empty to view)",
						Required:    false,
					},
				},
			},
			Handler: handler.replyRateCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "reactionrate",
				Description: "View or set the reaction rate",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionInt{
						Name:        "rate",
						Description: "the rate to set (leave empty to view)",
						Required:    false,
					},
				},
			},
			Handler: handler.reactionRateCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "cohesion",
				Description: "View or set the cohesion value; A higher value makes sentences more coherent",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionInt{
						MinValue:    new(2),
						MaxValue:    new(255),
						Name:        "value",
						Description: "the value to set, must be at least 2 (leave empty to view)",
						Required:    false,
					},
				},
			},
			Handler: handler.cohesionCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "opinion",
				Description: "Generates a random opinion based on the provided seed",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "about",
						Description: "The seed of the message",
						Required:    true,
					},
				},
			},
			Handler: handler.opinionCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "wipe",
				Description: "Deletes the given argument `data` from the training data",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "data",
						Description: "The data to be deleted",
						Required:    true,
					},
				},
			},
			Handler: handler.wipeCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "channels",
				Description: "View which channels can be accessed for training data",
			},
			Handler: handler.channelsCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "src",
				Description: "Provides the URL to the repository with bot source code",
			},
			Handler: handler.srcCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "vc-join",
				Description: "Joins the voice channel you are currently in",
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcJoinCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "vc-leave",
				Description: "Leaves the voice channel you are currently in",
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcLeaveCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "vc-speak",
				Description: "Speaks a message in the VC you are in, and then leaves",
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcSpeakCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "vc-language",
				Description: "View or set the language to use when generating audio",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionInt{
						Name: "language",
						Choices: []discord.ApplicationCommandOptionChoiceInt{
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
						},
						Description: "the language to set (leave empty to view)",
						Required:    false,
					},
				},
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcLanguageCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "vc-joinrate",
				Description: "View or set the VC random join rate",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionInt{
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
	registeredCommands, err := h.Client.Rest.GetGlobalCommands(h.Client.ApplicationID, false)
	if err != nil {
		logger.Errorf("Failed to fetch registered commands: %v", err)
		registeredCommands = []discord.ApplicationCommand{}
	}

	// Create a map of registered commands for fast lookup
	registeredCommandsMap := make(map[string]discord.ApplicationCommand)
	for _, cmd := range registeredCommands {
		registeredCommandsMap[cmd.Name()] = cmd
		isOutdated := !slices.ContainsFunc(commands, func(c SlashCommand) bool {
			return shouldRefreshCommand(cmd, c.Command)
		})
		if isOutdated {
			logger.Warnf("Removing outdated slash command: %s", cmd.Name())
			h.Client.Rest.DeleteGlobalCommand(h.Client.ApplicationID, cmd.ID())
		}
	}

	// Iterate through new commands and check if they are already registered
	for _, cmd := range commands {
		if existingCmd, exists := registeredCommandsMap[cmd.Command.CommandName()]; exists {
			// Compare if the new command differs in some way (e.g., updated description or options)
			if shouldRefreshCommand(existingCmd, cmd.Command) {
				logger.Infof("Updating slash command: %s", cmd.Command.CommandName())
				for _, guildId := range cmd.GuildIds {
					guildSnowflake := snowflake.MustParse(guildId)
					h.Client.Rest.DeleteGuildCommand(h.Client.ApplicationID, guildSnowflake, existingCmd.ID())
					_, err := h.Client.Rest.CreateGuildCommand(h.Client.ApplicationID, guildSnowflake, cmd.Command)
					if err != nil {
						logger.Errorf("Failed to register slash command: %v", err)
					}
				}
				// If no guild IDs, create globally
				if len(cmd.GuildIds) == 0 {
					h.Client.Rest.DeleteGlobalCommand(h.Client.ApplicationID, existingCmd.ID())
					_, err := h.Client.Rest.CreateGlobalCommand(h.Client.ApplicationID, cmd.Command)
					if err != nil {
						logger.Errorf("Failed to register slash command: %v", err)
					}
				}
			}
		} else {
			// Register the new command
			logger.Infof("Registering slash command: %s", cmd.Command.CommandName())
			for _, guildId := range cmd.GuildIds {
				guildSnowflake := snowflake.MustParse(guildId)
				_, err := h.Client.Rest.CreateGuildCommand(h.Client.ApplicationID, guildSnowflake, cmd.Command)
				if err != nil {
					logger.Errorf("Failed to register slash command: %v", err)
				}
			}
			// If no guild IDs, create globally
			if len(cmd.GuildIds) == 0 {
				_, err := h.Client.Rest.CreateGlobalCommand(h.Client.ApplicationID, cmd.Command)
				if err != nil {
					logger.Errorf("Failed to register slash command: %v", err)
				}
			}
		}
		h.Commands[cmd.Command.CommandName()] = cmd.Handler
	}
}

// Entry point for handling slash command interactions
func (h *SlashCommandsHandler) OnSlashCommandInteraction(event *events.ApplicationCommandInteractionCreate) {
	commandName := event.SlashCommandInteractionData().CommandName()

	var where string
	if event.GuildID() == nil {
		where = "DMs"
	} else {
		if guild, ok := h.Client.Caches.Guild(*event.GuildID()); ok {
			where = guild.Name
		} else {
			logger.Errorf("Failed to fetch guild '%s' for command interaction", event.GuildID().String())
			return
		}
	}

	who := event.User().EffectiveName()

	startTime := time.Now()
	logger.Infof("from '%s' in '%s': /%s", who, where, commandName)
	if handler, exists := h.Commands[commandName]; exists {
		handler(h.Client, event) // Call the function bound to this command
	}
	logger.Infof("/%s handler from '%s' in '%s' done in %s", commandName, who, where, time.Since(startTime).String())
}

// compares two commands to check if they are identical in the significant fields
func shouldRefreshCommand(cached discord.ApplicationCommand, loaded discord.ApplicationCommandCreate) bool {
	if cached.Type() != loaded.Type() || cached.Name() != loaded.CommandName() {
		return true
	}
	// switch cachedCmd := cached.(type) {
	// case discord.SlashCommand:
	// 	loadedCmd, ok := loaded.(discord.SlashCommandCreate)
	// 	if !ok {
	// 		return true
	// 	}

	// 	if cachedCmd.Description != loadedCmd.Description {
	// 		return true
	// 	}

	// 	if !reflect.DeepEqual(cachedCmd.Options, loadedCmd.Options) {
	// 		return true
	// 	}

	// 	// TODO
	// 	// Check Permissions (Discord returns null or a value)
	// 	// Note: Check if you care about DMPermissions vs Contexts here too

	// case discord.UserCommand, discord.MessageCommand:
	// 	// TODO
	// }

	return false
}
