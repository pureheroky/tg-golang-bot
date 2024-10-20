package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/pureheroky/tg-golang-bot/models"
	"github.com/pureheroky/tg-golang-bot/utils"
)

type AwaitingRequests struct {
	sync.RWMutex
	m map[int64]bool
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Println("Error loading .env file:", err)
	}

	errorLogger, workLogger := setupLogging()
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

	awaitingRequests := &AwaitingRequests{
		m: make(map[int64]bool),
	}
	dataStore := &models.DataStore{
		UserProjectIndex:   make(map[int]int),
		UserGitCommitIndex: make(map[int]int),
	}

	username := "pureheroky"
	gitApiUrl := "https://api.github.com"
	gitToken := os.Getenv("GIT_TOKEN")

	if err := loadData(dataStore, gitApiUrl, username, gitToken, errorLogger); err != nil {
		errorLogger.Fatal("Failed to load data:", err)
	}

	workLogger.Println("Bot started successfully.")

	registerHandlers(bh, bot, dataStore, awaitingRequests, skillsURL, errorLogger, workLogger)
	bh.Start()
}

func setupLogging() (*log.Logger, *log.Logger) {
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		os.Mkdir("logs", os.ModePerm)
	}

	errorLogFile, err := os.OpenFile("logs/error.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Failed to open error log file:", err)
		os.Exit(1)
	}
	errorLogger := log.New(errorLogFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	workLogFile, err := os.OpenFile("logs/work.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		errorLogger.Println("Failed to open work log file:", err)
		os.Exit(1)
	}
	workLogger := log.New(workLogFile, "INFO: ", log.Ldate|log.Ltime)

	return errorLogger, workLogger
}

func loadData(dataStore *models.DataStore, apiUrl, username, token string, errorLogger *log.Logger) error {
	dataStore.Lock()
	defer dataStore.Unlock()

	var err error
	dataStore.Projects, err = utils.GetProjects(apiUrl, username, token, dataStore)
	if err != nil {
		return fmt.Errorf("failed to get projects: %w", err)
	}

	dataStore.Git, err = utils.GetGitConcurrently(apiUrl, username, token, dataStore)
	if err != nil {
		return fmt.Errorf("failed to get git data: %w", err)
	}

	return nil
}

func registerHandlers(bh *th.BotHandler, bot *telego.Bot, dataStore *models.DataStore, awaitingRequests *AwaitingRequests, skillsURL string, errorLogger, workLogger *log.Logger) {
	bh.Handle(startCommandHandler(bot, workLogger), th.CommandEqual("start"))
	bh.Handle(acceptCommandHandler(bot, errorLogger), th.CommandEqual("accept"))
	bh.Handle(declineCommandHandler(bot, errorLogger), th.CommandEqual("decline"))
	bh.HandleCallbackQuery(callbackQueryHandler(bot, dataStore, awaitingRequests, skillsURL, errorLogger, workLogger))
	bh.Handle(messageHandler(bot, awaitingRequests, errorLogger), th.AnyMessage())
}

func startCommandHandler(bot *telego.Bot, workLogger *log.Logger) func(*telego.Bot, telego.Update) {
	return func(bot *telego.Bot, update telego.Update) {
		workLogger.Printf("Received /start command from user %d", update.Message.From.ID)

		_ = bot.DeleteMessage(tu.Delete(
			tu.ID(update.Message.Chat.ID),
			update.Message.MessageID,
		))

		messageText := getWelcomeMessage()
		message := tu.Message(
			tu.ID(update.Message.Chat.ID),
			messageText,
		)
		message.ParseMode = telego.ModeHTML
		message = message.WithReplyMarkup(getMainMenuMarkup())

		if _, err := bot.SendMessage(message); err != nil {
			workLogger.Println("Failed to send start message:", err)
		}
	}
}

func acceptCommandHandler(bot *telego.Bot, errorLogger *log.Logger) func(*telego.Bot, telego.Update) {
	return func(bot *telego.Bot, update telego.Update) {
		parts := strings.Fields(update.Message.Text)
		if len(parts) < 2 {
			errorLogger.Println("Invalid /accept command format")
			return
		}

		userId := parts[1]
		id, err := strconv.ParseInt(userId, 10, 64)
		if err != nil {
			errorLogger.Println("Invalid user ID in /accept command:", err)
			return
		}

		message := tu.Message(
			tu.ID(id),
			"Your request was accepted!\n\nDeveloper will soon contact you\n\nThis message will be deleted after <b>2 minutes</b>",
		)
		message.ParseMode = telego.ModeHTML

		sentMessage, err := bot.SendMessage(message)
		if err != nil {
			errorLogger.Println("Failed to send acceptance message:", err)
			return
		}

		time.AfterFunc(2*time.Minute, func() {
			_ = bot.DeleteMessage(tu.Delete(
				tu.ID(id),
				sentMessage.MessageID,
			))
		})
	}
}

func declineCommandHandler(bot *telego.Bot, errorLogger *log.Logger) func(*telego.Bot, telego.Update) {
	return func(bot *telego.Bot, update telego.Update) {
		parts := strings.Fields(update.Message.Text)
		if len(parts) < 2 {
			errorLogger.Println("Invalid /decline command format")
			return
		}

		userId := parts[1]
		id, err := strconv.ParseInt(userId, 10, 64)
		if err != nil {
			errorLogger.Println("Invalid user ID in /decline command:", err)
			return
		}

		answer := strings.Join(parts[2:], " ")

		message := tu.Message(
			tu.ID(id),
			fmt.Sprintf("Your request was declined!\n\nDeveloper message: \n%s\n\nThis message will be deleted after <b>2 minutes</b>", answer),
		)
		message.ParseMode = telego.ModeHTML

		sentMessage, err := bot.SendMessage(message)
		if err != nil {
			errorLogger.Println("Failed to send decline message:", err)
			return
		}

		time.AfterFunc(2*time.Minute, func() {
			_ = bot.DeleteMessage(tu.Delete(
				tu.ID(id),
				sentMessage.MessageID,
			))
		})
	}
}

func callbackQueryHandler(bot *telego.Bot, dataStore *models.DataStore, awaitingRequests *AwaitingRequests, skillsURL string, errorLogger, workLogger *log.Logger) func(*telego.Bot, telego.CallbackQuery) {
	return func(bot *telego.Bot, query telego.CallbackQuery) {
		workLogger.Printf("Received callback query from user %d: %s", query.From.ID, query.Data)

		chatID := query.Message.GetChat().ID

		const pageSize = 1

		projectMarkup := getProjectMarkup()
		gitMarkup := getGitMarkup()
		mainMenuMarkup := getMainMenuMarkup()
		BackMarkup := getBackMarkup()

		editedMessage := telego.EditMessageTextParams{
			ChatID:      tu.ID(chatID),
			MessageID:   query.Message.GetMessageID(),
			ParseMode:   telego.ModeHTML,
			ReplyMarkup: mainMenuMarkup,
		}

		switch query.Data {
		case "request":
			handleRequestCallback(bot, query, BackMarkup, awaitingRequests, editedMessage)
		case "skills":
			handleSkillsCallback(bot, query, skillsURL, BackMarkup, editedMessage, errorLogger)
		case "git":
			handleGitCallback(bot, query, dataStore, gitMarkup, pageSize, editedMessage, errorLogger)
		case "projects":
			handleProjectsCallback(bot, query, dataStore, projectMarkup, editedMessage, errorLogger)
		case "next_git", "previous_git":
			handleGitPagination(bot, query, dataStore, gitMarkup, pageSize, editedMessage, errorLogger)
		case "next_project", "previous_project":
			handleProjectPagination(bot, query, dataStore, projectMarkup, editedMessage)
		case "back":
			handleBackCallback(bot, query, awaitingRequests, editedMessage)
		default:
			workLogger.Printf("Unknown callback data: %s", query.Data)
		}
	}
}

func messageHandler(bot *telego.Bot, awaitingRequests *AwaitingRequests, errorLogger *log.Logger) func(*telego.Bot, telego.Update) {
	return func(bot *telego.Bot, update telego.Update) {
		chatID := update.Message.Chat.ID

		awaitingRequests.RLock()
		awaiting, ok := awaitingRequests.m[chatID]
		awaitingRequests.RUnlock()

		if ok && awaiting {
			handleRequestMessage(bot, update, awaitingRequests, errorLogger)
		} else {
			// Удаление неопознанных сообщений
			_ = bot.DeleteMessage(tu.Delete(
				tu.ID(chatID),
				update.Message.MessageID,
			))
		}
	}
}

func getWelcomeMessage() string {
	return `
<b><i>pureheroky</i></b> was created to help people contact/learn about me.

It has a couple of different <strong>buttons</strong> that show any information (knowledge stacks, projects, etc.).

Command list:

<code><b>Request:</b>
create a job request</code>

<code><b>Git:</b>
get last commits/accessible repos</code>

<code><b>Skills:</b>
get knowledge stack</code>

<code><b>Projects:</b>
get list of complete/under development projects</code>

Bot will be open source someday (look on my <a href='https://pureheroky.com'>website</a> or in the bot description)
`
}

func getMainMenuMarkup() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardCols(2,
			tu.InlineKeyboardButton("request").WithCallbackData("request"),
			tu.InlineKeyboardButton("git").WithCallbackData("git"),
			tu.InlineKeyboardButton("skills").WithCallbackData("skills"),
			tu.InlineKeyboardButton("projects").WithCallbackData("projects"),
		)...,
	)
}

func getProjectMarkup() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("previous").WithCallbackData("previous_project"),
			tu.InlineKeyboardButton("next").WithCallbackData("next_project"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("back").WithCallbackData("back"),
		),
	)
}

func getBackMarkup() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("back").WithCallbackData("back"),
		),
	)
}

func getGitMarkup() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("previous").WithCallbackData("previous_git"),
			tu.InlineKeyboardButton("next").WithCallbackData("next_git"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("back").WithCallbackData("back"),
		),
	)
}

func handleRequestCallback(bot *telego.Bot, query telego.CallbackQuery, requestMarkup *telego.InlineKeyboardMarkup, awaitingRequests *AwaitingRequests, editedMessage telego.EditMessageTextParams) {
	messageText := `
You are on <b>Request</b> page.

If you want to make a job request, please follow the <b>sample</b>:

<b>1. Your name</b>
<b>2. Direction of the task (any web-development/python apps etc.)</b>
<b>3. Task description</b>
<b>4. Ways to contact you</b>

Requests not similar to the sample <span class='tg-spoiler'><b>will be ignored.</b></span>
`
	editedMessage.Text = messageText
	editedMessage.ReplyMarkup = requestMarkup
	bot.EditMessageText(&editedMessage)

	chatID := query.Message.GetChat().ID
	awaitingRequests.Lock()
	awaitingRequests.m[chatID] = true
	awaitingRequests.Unlock()
}

func handleSkillsCallback(bot *telego.Bot, query telego.CallbackQuery, skillsURL string, skillsMarkup *telego.InlineKeyboardMarkup, editedMessage telego.EditMessageTextParams, errorLogger *log.Logger) {
	messageText := "You are on <b>Skills</b> page\nAll my knowledge will be shown here\n\n\n<b><i>Loading skills...</i></b>"
	editedMessage.Text = messageText
	bot.EditMessageText(&editedMessage)

	skills, err := utils.GetSkills(skillsURL)
	if err != nil {
		errorLogger.Println("Failed to get skills:", err)
		editedMessage.Text = "Failed to load skills."
		bot.EditMessageText(&editedMessage)
		return
	}

	messageText = "There is my knowledge stack:\n\n"
	for key, value := range skills {
		messageText += fmt.Sprintf("<i>%d</i>. <b>%s</b>\n", key+1, value)
	}
	messageText += "\n\nMore information about the projects can be found <a href='https://pureheroky.com'><b>here</b></a>"
	editedMessage.Text = messageText
	editedMessage.ReplyMarkup = skillsMarkup
	bot.EditMessageText(&editedMessage)
}

func handleGitCallback(bot *telego.Bot, query telego.CallbackQuery, dataStore *models.DataStore, gitMarkup *telego.InlineKeyboardMarkup, pageSize int, editedMessage telego.EditMessageTextParams, errorLogger *log.Logger) {
	chatID := query.Message.GetChat().ID

	dataStore.Lock()
	dataStore.UserGitCommitIndex[int(chatID)] = 0
	dataStore.Unlock()

	messageText := "You are on <b>Git</b> page\nThe latest commits will be shown here\n\n\n<b><i>Loading commits...</i></b>"
	editedMessage.ReplyMarkup = gitMarkup
	editedMessage.Text = messageText
	bot.EditMessageText(&editedMessage)

	dataStore.RLock()
	gitData := dataStore.Git
	dataStore.RUnlock()

	if len(gitData) > 0 {
		messageText = utils.FormatGitMessagePage(gitData, 0, pageSize)
	} else {
		messageText = "\n\nNo git commits found."
	}

	messageText += "\n<b><i>More information about the projects can be found <a href='https://github.com/pureheroky'>here</a></i></b>\n\n"
	editedMessage.Text = messageText
	bot.EditMessageText(&editedMessage)
}

func handleProjectsCallback(bot *telego.Bot, query telego.CallbackQuery, dataStore *models.DataStore, projectMarkup *telego.InlineKeyboardMarkup, editedMessage telego.EditMessageTextParams, errorLogger *log.Logger) {
	chatID := query.Message.GetChat().ID

	dataStore.Lock()
	dataStore.UserProjectIndex[int(chatID)] = 0
	dataStore.Unlock()

	messageText := "You are on <b>Projects</b> page\n<b><i>Loading projects...</i></b>\n"
	editedMessage.ReplyMarkup = projectMarkup
	editedMessage.Text = messageText
	bot.EditMessageText(&editedMessage)

	dataStore.RLock()
	projects := dataStore.Projects
	dataStore.RUnlock()

	if len(projects) > 0 {
		messageText += utils.FormatProjectMessage(projects[0])
	} else {
		messageText += "\n\nNo projects found."
	}

	editedMessage.Text = messageText
	bot.EditMessageText(&editedMessage)
}

func handleGitPagination(bot *telego.Bot, query telego.CallbackQuery, dataStore *models.DataStore, gitMarkup *telego.InlineKeyboardMarkup, pageSize int, editedMessage telego.EditMessageTextParams, errorLogger *log.Logger) {
	chatID := query.Message.GetChat().ID
	isNext := query.Data == "next_git"

	dataStore.Lock()
	currentIndex := dataStore.UserGitCommitIndex[int(chatID)]
	totalCommits := len(dataStore.Git)
	dataStore.Unlock()

	if isNext {
		if currentIndex < (totalCommits-1)/pageSize {
			currentIndex++
		} else {
			return
		}
	} else {
		if currentIndex > 0 {
			currentIndex--
		} else {
			return
		}
	}

	dataStore.Lock()
	dataStore.UserGitCommitIndex[int(chatID)] = currentIndex
	gitData := dataStore.Git
	dataStore.Unlock()

	messageText := utils.FormatGitMessagePage(gitData, currentIndex, pageSize)
	editedMessage.ReplyMarkup = gitMarkup
	editedMessage.Text = messageText
	bot.EditMessageText(&editedMessage)
}

func handleProjectPagination(bot *telego.Bot, query telego.CallbackQuery, dataStore *models.DataStore, projectMarkup *telego.InlineKeyboardMarkup, editedMessage telego.EditMessageTextParams) {
	chatID := query.Message.GetChat().ID
	isNext := query.Data == "next_project"

	dataStore.Lock()
	currentIndex := dataStore.UserProjectIndex[int(chatID)]
	totalProjects := len(dataStore.Projects)
	dataStore.Unlock()

	if isNext {
		if currentIndex < totalProjects-1 {
			currentIndex++
		} else {
			return
		}
	} else {
		if currentIndex > 0 {
			currentIndex--
		} else {
			return
		}
	}

	dataStore.Lock()
	dataStore.UserProjectIndex[int(chatID)] = currentIndex
	projects := dataStore.Projects
	dataStore.Unlock()

	messageText := utils.FormatProjectMessage(projects[currentIndex])
	editedMessage.ReplyMarkup = projectMarkup
	editedMessage.Text = messageText
	bot.EditMessageText(&editedMessage)
}

func handleBackCallback(bot *telego.Bot, query telego.CallbackQuery, awaitingRequests *AwaitingRequests, editedMessage telego.EditMessageTextParams) {
	messageText := getWelcomeMessage()
	editedMessage.Text = messageText
	editedMessage.ReplyMarkup = getMainMenuMarkup()
	bot.EditMessageText(&editedMessage)

	chatID := query.Message.GetChat().ID
	awaitingRequests.Lock()
	delete(awaitingRequests.m, chatID)
	awaitingRequests.Unlock()
}

func handleRequestMessage(bot *telego.Bot, update telego.Update, awaitingRequests *AwaitingRequests, errorLogger *log.Logger) {
	chatID := update.Message.Chat.ID
	requestText := update.Message.Text
	requestID := update.Message.From.ID
	requestUsername := update.Message.From.Username

	awaitingRequests.Lock()
	delete(awaitingRequests.m, chatID)
	awaitingRequests.Unlock()

	adminID, err := strconv.ParseInt(os.Getenv("USER_ID"), 10, 64)
	if err != nil {
		errorLogger.Println("Invalid admin USER_ID:", err)
		return
	}

	messageToAdmin := tu.Message(
		tu.ID(adminID),
		fmt.Sprintf("Request from <code>%s</code> | <code>%d</code>\n\n%s", requestUsername, requestID, requestText),
	)
	messageToAdmin.ParseMode = telego.ModeHTML
	if _, err := bot.SendMessage(messageToAdmin); err != nil {
		errorLogger.Println("Failed to send request message to admin:", err)
	}

	message := tu.Message(
		tu.ID(chatID),
		"Thank you for your job request.\n\nYour and this message will be deleted after <b>2 minutes</b>\n\nI'll write you after reviewing your request",
	)
	message.ParseMode = telego.ModeHTML

	sentMessage, err := bot.SendMessage(message)
	if err != nil {
		errorLogger.Println("Failed to send confirmation message:", err)
		return
	}

	time.AfterFunc(2*time.Minute, func() {
		_ = bot.DeleteMessage(tu.Delete(
			tu.ID(chatID),
			sentMessage.MessageID,
		))
		_ = bot.DeleteMessage(tu.Delete(
			tu.ID(chatID),
			update.Message.MessageID,
		))
	})
}
