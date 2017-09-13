package main

import (
	"database/sql"
	"encoding/json"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io/ioutil"
	"log"
	"strings"
)

func readAppConfig() (config *AppConfig, err error) {
	config_json, err := ioutil.ReadFile(appConfigPath)
	if err != nil {
		return
	}

	config = new(AppConfig)
	err = json.Unmarshal(config_json, config)
	if err != nil {
		return
	}
	return
}

var messagePull *MessagePull

func init() {
	var err error
	log.Println("Loading app config")
	appConfig, err = readAppConfig()
	if err != nil {
		log.Printf("Error occured at init : %v", err)
		log.Fatal("Cant run app due to fatal errors during init")
	}
	messagePull = NewMessagePull(cleanQuestionPoolInterval, inlineTempQuestionStoreTime)
	messagePull.init()
}

func main() {
	sqlstore, err := NewSQLStore("botbase.sql")
	if err != nil {
		log.Fatal(err)
	}
	defer sqlstore.db.Close()
	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramBotToken)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		log.Println("receive update")
		go processUpdate(bot, &update, sqlstore)
	}

}

func processUpdate(bot *tgbotapi.BotAPI, update *tgbotapi.Update, store *SQLStore) (err error) {
	log.Println("Receive update")

	if update.CallbackQuery != nil {
		log.Println("Receive callback")
		var m Message
		m, err = messagePull.getMessage(update.CallbackQuery.Data)
		if err != nil {
			log.Printf("Access to deleted question with tag %v", err)
			//TODO here we should send notification, that question's been deleted
			return
		}

		switch m.(type) {
		case *Question:
			log.Println("Adding question")
			question := m.(*Question)

			question.QuestionID, err = store.addQuestion(question)
			if err != nil {
				log.Printf("Error adding question database: %v", err)
				return
			}

			var chatID int64
			if question.Rec.User == AllGroupName {
				chatID = AllGroupChatID
			} else {
				chatID, err = store.getUserChatID(question.Rec.User)
				if err == sql.ErrNoRows {
					log.Printf("Don't know user personal chat adress: %v", err)
					return
				} else if err != nil {
					log.Printf("Error accesing sql database : %v", err)
					return
				}
			}

			msg := makeAskedPersonNotification(question, chatID)
			_, err = bot.Send(msg)

			if err != nil {
				log.Printf("Error sending notification")
				return
			}

			return
		case *Answer:
			log.Println("Adding answer")
			log.Println(m.(*Answer).Text)
			return
		}

		return
	}
	if (update.Message == nil) && (update.InlineQuery == nil) {
		return
	}
	if update.InlineQuery != nil {
		log.Println("Recognised as inline query")
		err = processInlineQuery(bot, update, store)
		if err != nil {
			log.Println(err)
		}
	} else {

		if isUserChat(update.Message.Chat) {
			log.Println("Received message from user chat")
			_, err = store.getUserChatID(update.Message.From.UserName)
			log.Println("Find it's personal chatID in database")
			if err != nil {
				log.Println(err)
				log.Println("Adding user chatID with fbbbot to database")
				user := &User{
					ID:     update.Message.From.ID,
					Name:   strings.Replace(update.Message.From.UserName, "@", "", -1),
					ChatID: update.Message.Chat.ID,
				}
				err = store.addUser(user)
				if err != nil {
					log.Printf("Error accesing database: %v", err)
					return
				}
			}
		} else {
			log.Println(update.Message.Chat.ID)
		}

		if !update.Message.IsCommand() {
			if strings.HasPrefix(update.Message.Text, "------") {
				log.Println("Recognised as inline answer")
				deleteConfig := tgbotapi.DeleteMessageConfig{
					ChatID:    update.Message.Chat.ID,
					MessageID: update.Message.MessageID,
				}
				go messageDeleter(bot, deleteConfig, appConfig.InlineAnswersTimeToDelete)
			}
		} else {
			log.Println("Recognised as command")
			err = processCommand(bot, update, store)
			if err != nil {
				log.Println(err)
			}
		}
	}
	return
}
