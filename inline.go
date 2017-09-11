package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"strconv"
	"log"
	"strings"
	"fmt"
	"time"
)

func parseSlashQuestion(m *tgbotapi.Message) (q *Question, err error) {
	q = new(Question)
	q.Date = m.Time().UTC()
	q.User = m.From.UserName
	q.Rec = NewReceiver(AllGroupName)
	q.Text = m.CommandArguments()
	if strings.TrimSpace(q.Text) == "" {
		err = WrongCommandFormat
		return
	}
	q.Answers = []*Answer{}
	q.IsClosed = false
	q.ChatID = m.Chat.ID
	q.QuestionID = -1
	return
}

func processQuery(query string) (command string, commandArgs string) {
	splitRes := strings.SplitN(query, " ", 2)
	command = strings.TrimSpace(splitRes[0])
	if len(splitRes) == 2 {
		commandArgs = strings.TrimSpace(splitRes[1])
	}
	return
}

func sendEnterReply(bot *tgbotapi.BotAPI, update *tgbotapi.Update) (err error) {
	reply := tgbotapi.NewInlineQueryResultArticle("1", "Введите команду",
		EmptyMessage)
	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: update.InlineQuery.ID,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       []interface{}{reply},
		NextOffset:    "",
	}
	_, err = bot.AnswerInlineQuery(inlineConfig)
	if err != nil {
		return
	}
	return
}

func sendNotExistReply(bot *tgbotapi.BotAPI, update *tgbotapi.Update) (err error) {
	reply := tgbotapi.NewInlineQueryResultArticle("1", NotExistsMessage,
		NotExistsMessage)
	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: update.InlineQuery.ID,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       []interface{}{reply},
		NextOffset:    "",
	}
	_, err = bot.AnswerInlineQuery(inlineConfig)
	if err != nil {
		return
	}
	return
}


func markAsBotText(in string) (out string) {
	out = fmt.Sprintf("%s\n%s\n%s", BotMessageSign, in, BotMessageSign)
	return
}

func questionToReply(q *Question, id int) (reply tgbotapi.InlineQueryResultArticle) {
	dateText := q.Date.Local().Format("Jan 2, 2006 в 3:04pm")
	replyText := fmt.Sprintf(`
Информация о вопросе [%d]
*Задавший*: @%s
*Дата*: %s:
*Текст вопроса*:
"%s"`, q.QuestionID, q.User, dateText, q.Text)
	replyText = markAsBotText(replyText)

	replyTitle := fmt.Sprintf("От @%s в %s", q.User, dateText)

	reply = tgbotapi.NewInlineQueryResultArticleMarkdown(strconv.Itoa(id),
		replyTitle, replyText) //"Question"+strconv.Itoa(q.QuestionID))

	if len(q.Text) > MaxShownMessageLength {
		reply.Description = fmt.Sprintf("%s...", q.Text[:MaxShownMessageLength])
	} else {
		reply.Description = q.Text
	}
	return
}

func sendQuestionListReply(bot *tgbotapi.BotAPI,
	store *SQLStore, queryId string, receiver string) (err error) {
	questions, err := store.findAllQuestionsTo(receiver)
	if err != nil {
		return
	}
	var answers []interface{}
	if len(questions) == 0 {
		titleText := "Нет вопросов"
		messagesText := markAsBotText(titleText)
		reply := tgbotapi.NewInlineQueryResultArticle("1",
			titleText, messagesText)
		answers = append(answers, reply)
	} else {
		for id, q := range questions {
			answers = append(answers, questionToReply(q, id))
		}
	}

	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: queryId,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       answers,
		NextOffset:    "",
	}

	res, err := bot.AnswerInlineQuery(inlineConfig)
	log.Println(res)

	if err != nil {
		return
	}
	return
}

var INLINE_COMMANDS = []string{"list_questions", "list_answers", "list_questions_to_me",
	"watch_question", "watch_answer"}

func messageDeleter(bot *tgbotapi.BotAPI, config tgbotapi.DeleteMessageConfig, waitTime int) {
	time.Sleep(time.Second * time.Duration(waitTime))
	log.Printf("Deleting message %d from chat %d", config.MessageID, config.ChatID)
	_, err := bot.DeleteMessage(config)
	if err != nil {
		log.Println(err)
		return
	}
}

func processInlineQuery(bot *tgbotapi.BotAPI, update *tgbotapi.Update, store *SQLStore) (err error) {
	query := update.InlineQuery.Query
	log.Printf("[%s] inline %s\n", update.InlineQuery.From.UserName, query)

	if query == "" {
		err = sendEnterReply(bot, update)
		if err != nil {
			log.Printf("Error while sending enter reply: %v", err)
			return
		}
		return
	}
	command, commandArgs := processQuery(query)
	log.Println(command, commandArgs)
	if !inGroup(INLINE_COMMANDS, command) {
		err = sendNotExistReply(bot, update)
		if err != nil {
			log.Printf("Error while sending not exists reply")
			return
		}
		return
	}

	switch command {
	case "list_questions":
		err = sendQuestionListReply(bot, store, update.InlineQuery.ID, AllGroupName)
		if err != nil {
			log.Printf("Error while sending questions reply")
			return
		}
		return

	case "list_questions_to_me":
		err = sendQuestionListReply(bot, store, update.InlineQuery.ID, update.InlineQuery.From.UserName)
		if err != nil {
			log.Printf("Error while sending questions reply %v", err)
			return
		}
		return
	}
	var answers []interface{}

	offset := 0
	if update.InlineQuery.Offset != "" {
		offset, err = strconv.Atoi(update.InlineQuery.Offset)
		if err != nil {
			log.Println(err)
			return
		}
	}

	for i := 0; i < 10; i++ {
		result := tgbotapi.NewInlineQueryResultArticleMarkdown(strconv.Itoa(offset+i),
			"hi"+strconv.Itoa(offset+i), "k")
		reply := tgbotapi.NewInlineKeyboardMarkup([]tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData("fg", "fg")})
		result.ReplyMarkup = &reply
		answers = append(answers, result)
	}

	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: update.InlineQuery.ID,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       answers,
		NextOffset:    strconv.Itoa(offset + 1),
	}

	_, err = bot.AnswerInlineQuery(inlineConfig)

	if err != nil {
		log.Println(err)
	}

	return
}
