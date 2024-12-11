package main

import (
	"context"
	"fmt"
	"github.com/dstotijn/go-notion"
	"github.com/sokdak/eternity-bot/pkg/cache"
	"github.com/sokdak/eternity-bot/pkg/handler"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sokdak/eternity-bot/pkg/environment"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	dg, err := discordgo.New("Bot " + environment.DiscordAPIKey)
	if err != nil {
		panic(err)
	}

	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsMessageContent |
		discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	if err := handler.PollerInit(dg); err != nil {
		fmt.Println("Error initializing poller:", err)
		return
	}
	defer handler.PollerFinalize()

	dg.LogLevel = discordgo.LogDebug
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}
	defer dg.Close()

	// Run cache eviction policy
	cache.RunDiscordCacheEvictionPolicy(dg, 6*time.Hour, 10*time.Minute)

	n := notion.NewClient(environment.NotionBotAPIKey)
	if n == nil {
		fmt.Println("Error creating notion client")
		return
	}

	veryShortTermTicker := time.NewTicker(15 * time.Second)
	defer veryShortTermTicker.Stop()

	shortTermTicker := time.NewTicker(30 * time.Minute)
	defer shortTermTicker.Stop()

	longTermTicker := time.NewTicker(6 * time.Hour)
	defer longTermTicker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			log.Println("Received context cancellation, shutting down gracefully...")
			return
		case <-sigCh:
			log.Println("Received OS signal, shutting down gracefully...")
			return
		case <-veryShortTermTicker.C:
			err := handler.CounselPoller(dg, n, startTime, environment.NotionCounselDBID, environment.DiscordGuildID, environment.DiscordCounselChannelID)
			if err != nil {
				fmt.Println("Error polling counsel:", err)
			}
			if err := handler.PollFinishChecker(dg); err != nil {
				fmt.Println("Error checking poll finish:", err)
			}
		case <-shortTermTicker.C:
			err := handler.UpdateMessageWithRoles(dg,
				environment.DiscordGuildInfoChannelID, environment.DiscordGuildInfoByRoleMessageID)
			if err != nil {
				fmt.Println("Error updating message with roles:", err)
			}
			err = handler.UpdateMessagesWithLevels(dg,
				environment.DiscordGuildInfoChannelID, environment.DiscordGuildInfoByLevelMessageID)
			if err != nil {
				fmt.Println("Error updating messages with levels:", err)
			}
		case <-longTermTicker.C:
			err = handler.GeneralizeUsername(dg, environment.DiscordGuildID)
			if err != nil {
				fmt.Println("Error generalizing username:", err)
			}
		}
	}
	return
}
