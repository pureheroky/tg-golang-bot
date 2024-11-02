package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"

	"github.com/pureheroky/tg-golang-bot/handlers"
	"github.com/pureheroky/tg-golang-bot/models"
	"github.com/pureheroky/tg-golang-bot/utils"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Println("Error loading .env file:", err)
	}

	errorLogger, workLogger := utils.SetupLogging()
	defer func() {
		if err := recover(); err != nil {
			errorLogger.Printf("Application panicked: %v", err)
		}
	}()

	botToken := os.Getenv("TOKEN")
	skillsURL := os.Getenv("SKILLS_URL")
	bot, err := telego.NewBot(botToken, telego.WithDefaultLogger(true, false))
	if err != nil {
		errorLogger.Fatal("Failed to create bot:", err)
		os.Exit(1)
	}

	updates, _ := bot.UpdatesViaLongPolling(nil)
	bh, _ := th.NewBotHandler(bot, updates)
	defer bh.Stop()
	defer bot.StopLongPolling()

	awaitingRequests := &models.AwaitingRequests{
		M: make(map[int64]bool),
	}

	dataStore := &models.DataStore{
		UserProjectIndex:   make(map[int]int),
		UserGitCommitIndex: make(map[int]int),
	}

	username := "pureheroky"
	gitApiUrl := "https://api.github.com"
	gitToken := os.Getenv("GIT_TOKEN")

	if err := utils.LoadData(dataStore, gitApiUrl, username, gitToken, errorLogger); err != nil {
		errorLogger.Fatal("Failed to load data:", err)
	}

	workLogger.Println("Bot started successfully.")

	handlers.RegisterHandlers(bh, bot, dataStore, awaitingRequests, skillsURL, errorLogger, workLogger)
	bh.Start()
}
