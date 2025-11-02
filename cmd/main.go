package main

import (
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

	"github.com/bwmarrin/discordgo"
)

// LDFLAGS
var (
	Version string
)

func main() {
	config.Version = Version
	logger.Infof("Version: %s", config.Version)
	logger.Debugf("Env: %s", config.Env)
	logger.Debugln("Creating discord session...")
	ds, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		logger.Fatalf("error creating Discord session,", err)
	}

	ds.Identify.Intents = config.Intents

	// Open a websocket connection to Discord and begin listening.
	err = ds.Open()
	if err != nil {
		logger.Fatalln("error opening connection,", err)
	}
	logger.Debugln("Discord session created")
	events.UpdatePresence(ds)
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
	chainsService := services.NewChainsService(ds, *chainsRepo, *messagesRepo)
	dataFetchService := services.NewDataFetchService(ds, chainsService, messagesRepo)
	// Handlers
	messagesHandler := messages.NewMessageHandler(ds, chainsService)
	commandsHandler := commands.NewSlashCommandsHandler(ds, chainsService)
	buttonsHandler := buttons.NewButtonsHandler(ds, dataFetchService, chainsService)
	eventsHandler := events.NewEventsHandler(ds, chainsService)
	logger.Debugln("All services initialized")
	chainsService.LoadChains()
	ds.AddHandler(commandsHandler.OnSlashCommandInteraction)
	ds.AddHandler(messagesHandler.OnMessageCreate)
	ds.AddHandler(buttonsHandler.OnButtonInteraction)
	ds.AddHandler(eventsHandler.OnEventCreate)
	logger.Infof("Logged in as %s", ds.State.User.String())
	if config.RunHttpServer {
		srv := ihttp.NewHttpServer(ds, chainsService, messagesRepo)
		srv.Start()
	}
	logger.Infof("Startup time: %s", time.Since(config.StartupTime).String())

	// Wait here until SIGINT or other term signal is received.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	ds.Close()
}
