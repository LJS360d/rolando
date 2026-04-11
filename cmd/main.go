package main

import (
	"context"
	"os"
	"os/signal"
	"rolando/internal/config"
	"rolando/internal/logger"
	"syscall"
	"time"

	"rolando/cmd/idiscord/buttons"
	"rolando/cmd/idiscord/commands"
	"rolando/cmd/idiscord/events"
	"rolando/cmd/idiscord/messages"
	"rolando/cmd/idiscord/services"
	"rolando/cmd/ihttp"
	"rolando/internal/repositories"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	discordevents "github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave/golibdave"
)

// LDFLAGS
var (
	Version string
)

func main() {
	config.Version = Version
	logger.Infof("Version: %s", config.Version)
	logger.Debugf("Env: %s", config.Env)
	logger.Debugln("Creating discord client...")

	ctx := context.Background()

	client, err := disgo.New(config.Token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(config.Intents),
		),
		bot.WithVoiceManagerConfigOpts(
			voice.WithDaveSessionCreateFunc(golibdave.NewSession),
		),
		bot.WithCacheConfigOpts(
			cache.WithCaches(
				cache.FlagGuilds,
				cache.FlagChannels,
				cache.FlagRoles,
				cache.FlagMembers,
				cache.FlagVoiceStates,
			),
		),
		bot.WithEventListenerFunc(func(e *discordevents.GuildsReady) {
			events.UpdatePresence(e.Client())
		}),
	)
	if err != nil {
		logger.Fatalf("error creating Discord client: %v", err)
	}

	// Open a websocket connection to Discord and begin listening.
	err = client.OpenGateway(ctx)
	if err != nil {
		logger.Fatalln("error opening gateway connection:", err)
	}
	logger.Debugln("Discord client created and connected")

	logger.Debugln("Initializing services...")
	// DI
	messagesRepo, err := repositories.NewMessagesRepository(config.DatabasePath)
	if err != nil {
		logger.Fatalf("error creating messages repository: %v", err)
	}
	chainsRepo, err := repositories.NewChainsRepository(config.DatabasePath)
	if err != nil {
		logger.Fatalf("error creating chains repository: %v", err)
	}
	chainsService := services.NewChainsService(client, *chainsRepo, *messagesRepo)
	dataFetchService := services.NewDataFetchService(client, chainsService, messagesRepo)
	// Handlers
	messagesHandler := messages.NewMessageHandler(client, chainsService)
	commandsHandler := commands.NewSlashCommandsHandler(client, chainsService)
	buttonsHandler := buttons.NewButtonsHandler(client, dataFetchService, chainsService)
	eventsHandler := events.NewEventsHandler(client, chainsService)
	logger.Debugln("All services initialized")
	chainsService.LoadChains()

	client.EventManager.AddEventListeners(
		bot.NewListenerFunc(commandsHandler.OnSlashCommandInteraction),
		bot.NewListenerFunc(messagesHandler.OnMessageCreate),
		bot.NewListenerFunc(buttonsHandler.OnButtonInteraction),
		bot.NewListenerFunc(eventsHandler.OnEventCreate),
	)

	botUser, err := client.Rest.GetUser(client.ID())
	if err != nil {
		logger.Fatalf("error getting bot user: %v", err)
	}
	logger.Infof("Logged in as %s#%s", botUser.Username, botUser.Discriminator)
	if config.RunHttpServer {
		srv := ihttp.NewHttpServer(client, chainsService, messagesRepo)
		srv.Start()
	}
	logger.Infof("Startup time: %s", time.Since(config.StartupTime).String())

	// Wait here until SIGINT or other term signal is received.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	client.Close(ctx)
}
