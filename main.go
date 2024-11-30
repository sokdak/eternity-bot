package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sokdak/eternity-bot/pkg/environment"
	"github.com/sokdak/eternity-bot/pkg/handler"
	"time"
)

func main() {
	dg, err := discordgo.New("Bot " + environment.DiscordAPIKey)
	if err != nil {
		panic(err)
	}

	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}
	defer dg.Close()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := handler.UpdateMessageWithRoles(dg, environment.DiscordGuildID,
				environment.DiscordChannelID, environment.DiscordMessageID)
			if err != nil {
				fmt.Println("Error updating message:", err)
			}
		}
	}
	return
}
