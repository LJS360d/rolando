package main

import (
	"fmt"
	"os"
	"os/signal"
	"rolando/config"
	"rolando/server"
	"strconv"
	"syscall"

	"rolando/cmd/handlers"
	"rolando/cmd/log"
	"rolando/cmd/repositories"
	"rolando/cmd/services"

	"github.com/bwmarrin/discordgo"
)

// LDFLAGS
var (
	Version string
	Env     string
)

func main() {
	config.Version = Version
	config.Env = Env
	fmt.Println("Version: ", config.Version)
	fmt.Println("Env: ", config.Env)
	log.Log.Infoln("Creating discord session...")
	ds, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Log.Fatalf("error creating Discord session,", err)
	}

	ds.Identify.Intents = config.Intents

	// Open a websocket connection to Discord and begin listening.
	err = ds.Open()
	if err != nil {
		log.Log.Fatalln("error opening connection,", err)
	}
	log.Log.Infoln("Discord session created")
	log.Log.Infoln("Updating presence...")
	err = ds.UpdateStatusComplex(discordgo.UpdateStatusData{
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
	log.Log.Infoln("Initializing services...")
	// DI
	messagesRepo, err := repositories.NewMessagesRepository(config.DatabasePath)
	if err != nil {
		log.Log.Fatalf("error creating messages repository: %v", err)
	}
	chainsRepo, err := repositories.NewChainsRepository(config.DatabasePath)
	if err != nil {
		log.Log.Fatalf("error creating chains repository: %v", err)
	}
	chainsService := services.NewChainsService(ds, *chainsRepo, *messagesRepo)
	dataFetchService := services.NewDataFetchService(ds, chainsService, messagesRepo)
	// Handlers
	messagesHandler := handlers.NewMessageHandler(ds, chainsService)
	commandsHandler := handlers.NewSlashCommandsHandler(ds, chainsService)
	buttonsHandler := handlers.NewButtonsHandler(ds, dataFetchService, chainsService)
	eventsHandler := handlers.NewEventsHandler(ds, chainsService)
	log.Log.Infoln("All services initialized")
	chainsService.LoadChains()
	ds.AddHandler(commandsHandler.OnSlashCommandInteraction)
	ds.AddHandler(messagesHandler.OnMessageCreate)
	ds.AddHandler(buttonsHandler.OnButtonInteraction)
	ds.AddHandler(eventsHandler.OnEventCreate)
	log.Log.Infof("Logged in as %s", ds.State.User.String())
	srv := server.NewHttpServer(ds, chainsService, messagesRepo)
	srv.Start()

	// Wait here until SIGINT or other term signal is received.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	ds.Close()
}
