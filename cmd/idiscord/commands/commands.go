package commands

import (
	"rolando/cmd/idiscord/services"
	"rolando/internal/config"
	"rolando/internal/logger"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
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
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.withAdminPermission(handler.trainCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "gif",
				Description: "Returns a gif from the ones it knows",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.gifCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "image",
				Description: "Returns an image from the ones it knows",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.imageCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "video",
				Description: "Returns a video from the ones it knows",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.videoCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "analytics",
				Description: "Returns analytics about the bot in this server",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.analyticsCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "togglepings",
				Description: "Toggles wether pings are enabled or not",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.withAdminPermission(handler.togglePingsCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "replyrate",
				Description: "View or set the reply rate",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
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
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
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
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
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
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
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
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
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
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.channelsCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "src",
				Description: "Provides the URL to the repository with bot source code",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.srcCommand,
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "vc-join",
				Description: "Joins the voice channel you are currently in",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcJoinCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "vc-leave",
				Description: "Leaves the voice channel you are currently in",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcLeaveCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "vc-speak",
				Description: "Speaks a message in the VC you are in, and then leaves",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcSpeakCommand),
		},
		{
			Command: discord.SlashCommandCreate{
				Name:        "vc-language",
				Description: "View or set the language to use when generating audio",
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
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
				Contexts: []discord.InteractionContextType{
					discord.InteractionContextTypeGuild,
				},
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionInt{
						Name:        "rate",
						Description: "the rate to set (1/rate) (leave empty to view)",

						Required: false,
					},
				},
			},
			Handler: handler.withGuildSubscription(config.VoiceChatFeaturesSKU, handler.vcJoinRateCommand),
		},
	})

	return handler
}

func commandCreates(cmds []SlashCommand) []discord.ApplicationCommandCreate {
	out := make([]discord.ApplicationCommandCreate, len(cmds))
	for i := range cmds {
		out[i] = cmds[i].Command
	}
	return out
}

func (h *SlashCommandsHandler) registerCommands(commands []SlashCommand) {
	for i := range commands {
		h.Commands[commands[i].Command.CommandName()] = commands[i].Handler
	}

	var global []SlashCommand
	guildSeen := make(map[string]struct{})
	for i := range commands {
		if len(commands[i].GuildIds) == 0 {
			global = append(global, commands[i])
			continue
		}
		for _, gid := range commands[i].GuildIds {
			guildSeen[gid] = struct{}{}
		}
	}

	if len(global) <= 0 {
		logger.Infof("No global slash commands to sync.")
		return
	}

	refresh := config.ForceCommandRefresh
	if refresh {
		logger.Infof("Syncing global slash command interactions...")
		_, err := h.Client.Rest.SetGlobalCommands(h.Client.ApplicationID, commandCreates(global))
		if err != nil {
			logger.Errorf("Failed to sync global slash commands: %v", err)
		} else {
			logger.Infof("Global slash commands synced successfully.")
		}
	}

	// for gid := range guildSeen {
	// 	var guildCmds []SlashCommand
	// 	for i := range commands {
	// 		if slices.Contains(commands[i].GuildIds, gid) {
	// 			guildCmds = append(guildCmds, commands[i])
	// 		}
	// 	}
	// 	guildID := snowflake.MustParse(gid)
	// 	remote, err := h.Client.Rest.GetGuildCommands(h.Client.ApplicationID, guildID, false)
	// 	if err != nil {
	// 		logger.Errorf("Could not verify guild %s slash commands: %v", gid, err)
	// 		continue
	// 	}
	// 	if force || needsResync(remote, guildCmds) {
	// 		logger.Infof("Syncing guild %s slash command interactions...", gid)
	// 		_, err := h.Client.Rest.SetGuildCommands(h.Client.ApplicationID, guildID, commandCreates(guildCmds))
	// 		if err != nil {
	// 			logger.Errorf("Failed to sync guild %s slash commands: %v", gid, err)
	// 		} else {
	// 			logger.Infof("Guild %s slash commands synced successfully.", gid)
	// 		}
	// 	} else {
	// 		logger.Infof("Guild %s slash commands are up to date. Skipping sync.", gid)
	// 	}
	// }
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
