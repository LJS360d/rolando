package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"rolando/internal/config"
	"rolando/internal/logger"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"rolando/internal/cluster"
	"rolando/internal/dbl"
	"rolando/internal/discord/buttons"
	"rolando/internal/discord/commands"
	"rolando/internal/discord/events"
	"rolando/internal/discord/helpers"
	"rolando/internal/discord/messages"
	"rolando/internal/discord/services"
	"rolando/internal/guilds"
	"rolando/internal/httpapi"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/sharding"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave/golibdave"
)

// LDFLAGS
var (
	Version string
)

func main() {
	config.Version = Version
	if len(os.Args) > 1 {
		runSubcommand(os.Args[1:])
		return
	}
	logger.Infof("Version: %s", config.Version)
	if config.Token == "" {
		logger.Fatalf("TOKEN not set in the environment")
	}
	logger.Debugf("Env: %s", config.Env)
	setMemoryLimit()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Infof("Node %d/%d, shards %v of %d", config.NodeIndex, config.NodeCount, cluster.LocalShards(), config.ShardCount)
	var client *bot.Client
	client, err := disgo.New(config.Token,
		bot.WithShardManagerConfigOpts(
			sharding.WithShardIDs(cluster.LocalShards()...),
			sharding.WithShardCount(config.ShardCount),
			sharding.WithIdentifyRateLimiter(&cluster.SlotLimiter{}),
			sharding.WithGatewayConfigOpts(
				gateway.WithIntents(config.Intents),
				gateway.WithCompression(gateway.CompressionZlibStream),
			),
		),
		bot.WithVoiceManagerConfigOpts(
			voice.WithDaveSessionCreateFunc(golibdave.NewSession),
			// supress all logs
			voice.WithDaveSessionLogger(slog.New(slog.DiscardHandler)),
			voice.WithConnConfigOpts(
				voice.WithConnAudioSenderCreateFunc(helpers.NewAudioSender),
				voice.WithConnDaveSessionLogger(slog.New(slog.DiscardHandler)),
				voice.WithConnLogger(slog.New(slog.DiscardHandler)),
			),
		),
		bot.WithCacheConfigOpts(
			cache.WithCaches(
				cache.FlagGuilds,
				cache.FlagChannels,
				cache.FlagRoles,
				cache.FlagMembers,
				cache.FlagVoiceStates,
				cache.FlagEmojis,
			),
			// only self member is cache read; other user come with every message and are used stateless
			cache.WithMemberCachePolicy(func(m discord.Member) bool { return m.User.ID == client.ID() }),
			cache.WithChannelCachePolicy(func(ch discord.GuildChannel) bool {
				switch ch.Type() {
				case discord.ChannelTypeGuildText, discord.ChannelTypeGuildNews,
					discord.ChannelTypeGuildVoice, discord.ChannelTypeGuildStageVoice:
					return true
				}
				return false
			}),
		),
	)
	if err != nil {
		logger.Fatalf("error creating Discord client: %v", err)
	}

	manager := guilds.NewManager(filepath.Join(config.DataDir, "guilds"), config.ChainIdleTTL, config.SnapshotInterval)
	managerCtx, stopManager := context.WithCancel(context.Background())
	managerDone := make(chan struct{})
	go func() {
		manager.Run(managerCtx)
		close(managerDone)
	}()

	chainsService := services.NewChainsService(client, manager)
	dataFetchService := services.NewDataFetchService(client, chainsService)
	jackboxService := services.NewJackboxService(client, chainsService)
	messagesHandler := messages.NewMessageHandler(client, chainsService)
	commandsHandler := commands.NewSlashCommandsHandler(client, chainsService, jackboxService)
	buttonsHandler := buttons.NewButtonsHandler(client, dataFetchService, chainsService)
	eventsHandler := events.NewEventsHandler(client, chainsService)
	client.EventManager.AddEventListeners(
		bot.NewListenerFunc(commandsHandler.OnSlashCommandInteraction),
		bot.NewListenerFunc(messagesHandler.OnMessageCreate),
		bot.NewListenerFunc(buttonsHandler.OnButtonInteraction),
		bot.NewListenerFunc(eventsHandler.OnEventCreate),
	)

	srv := httpapi.NewServer(client, chainsService)
	if config.RunHttpServer {
		go srv.Start()
	}

	if err := client.OpenShardManager(ctx); err != nil {
		logger.Fatalf("error opening shards: %v", err)
	}

	go func() {
		t := time.NewTimer(time.Minute)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				events.UpdatePresence(client, srv.FreshTotalGuilds())
				t.Reset(24 * time.Hour)
			}
		}
	}()
	go dbl.Run(ctx, client, commandsHandler.Global)
	logger.Infof("All shards connected in %s", time.Since(config.StartupTime).Round(time.Millisecond))

	<-ctx.Done()
	stop()
	logger.Infof("Shutting down gracefully")
	done := make(chan struct{})
	go func() {
		shutdown(client, stopManager, managerDone)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		pprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
		logger.Fatalf("shutdown timed out")
	}
}

func shutdown(client *bot.Client, stopManager context.CancelFunc, managerDone <-chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	step("closing voice", func() { client.VoiceManager.Close(ctx) })
	step("closing rest", func() { client.Rest.Close(ctx) })
	step("closing shard manager", func() { client.ShardManager.Close(ctx) })
	step("flushing guilds", func() {
		stopManager()
		<-managerDone
	})
	logger.Infof("Shutdown complete")
}

func step(name string, fn func()) {
	logger.Debugf("%s", name)
	start := time.Now()
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				logger.Debugf("still %s after %s", name, time.Since(start).Round(time.Second))
			}
		}
	}()
	fn()
	close(done)
	logger.Debugf("%s done in %s", name, time.Since(start).Round(time.Millisecond))
}

// setMemoryLimit defaults the Go soft memory limit to 80% of the cgroup limit
// when GOMEMLIMIT is unset, so the GC works harder before the container OOMs.
// Native memory (vosk, dave) is not counted by the Go runtime.
func setMemoryLimit() {
	if os.Getenv("GOMEMLIMIT") != "" {
		return
	}
	b, err := os.ReadFile("/sys/fs/cgroup/memory.max")
	if err != nil {
		return
	}
	limit, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return
	}
	debug.SetMemoryLimit(limit / 10 * 8)
	logger.Infof("Memory limit set to %d MiB (80%% of cgroup limit)", limit/10*8>>20)
}
