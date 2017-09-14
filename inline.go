package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strconv"
	"strings"
	"time"
)

func parseQuery(query string) (command string, commandArgs string) {
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

type QuestionToReplyConverter func(q *Question, id int) (reply tgbotapi.InlineQueryResultArticle)

func simpleQuestionToReply(q *Question, id int) (reply tgbotapi.InlineQueryResultArticle) {
	dateText := formatDate(q.Date)
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

func appendReply(reply *tgbotapi.InlineQueryResultArticle, tag string, text string) {
	row := []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData(text, tag)}

	if reply.ReplyMarkup == nil {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(row)
		reply.ReplyMarkup = &keyboard
		return
	}
	buttonRows := reply.ReplyMarkup.InlineKeyboard
	reply.ReplyMarkup.InlineKeyboard = append(buttonRows, row)

	return
}

func sendEndReply(bot *tgbotapi.BotAPI, queryId string) (err error) {
	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: queryId,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       []interface{}{},
		NextOffset:    "",
	}
	_, err = bot.AnswerInlineQuery(inlineConfig)

	if err != nil {
		return
	}
	return
}

func sendNoQuestionsReply(bot *tgbotapi.BotAPI, queryId string) (err error) {
	err = sendSimpleStringReply(bot, queryId, "Нет вопросов")
	if err != nil {
		log.Printf("Error while sending no questions reply %v", err)
		return
	}
	return
}

func sendChunkQuestionsReply(bot *tgbotapi.BotAPI, queryId string,
	questions []*Question, offset int, converter QuestionToReplyConverter) (err error) {
	var replies []interface{}
	for id, q := range questions {
		replies = append(replies, converter(q, id))
	}

	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: queryId,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       replies,
		NextOffset:    strconv.Itoa(offset + len(replies)),
	}

	_, err = bot.AnswerInlineQuery(inlineConfig)
	if err != nil {
		return
	}
	return

}

func sendQuestionList(bot *tgbotapi.BotAPI,
	queryID string, offset int, questions []*Question,
	converter QuestionToReplyConverter) (err error) {
	if len(questions) == 0 {
		if offset == 0 {
			err = sendNoQuestionsReply(bot, queryID)
			if err != nil {
				log.Printf("Error while sending no questions reply: %v", err)
				return
			}
			return
		} else {
			err = sendEndReply(bot, queryID)
			if err != nil {
				log.Printf("Error while sending end reply: %v", err)
			}
			return
		}
	}

	err = sendChunkQuestionsReply(bot, queryID, questions, offset, converter)
	if err != nil {
		return
	}
	return
}

func sendQuestionListReply(bot *tgbotapi.BotAPI,
	store *SQLStore, queryID string, receiver string, offset_str string) (err error) {
	offset, err := convertQueryOffset(offset_str)
	if err != nil {
		log.Printf("Error while converting offset: %v", err)
		return
	}

	questions, err := store.findQuestionsTo(receiver, MaxSendInlineObjects, offset)
	if err != nil {
		log.Printf("Error accessing sql store: %v", err)
		return
	}
	err = sendQuestionList(bot, queryID, offset, questions, simpleQuestionToReply)
	if err != nil {
		log.Printf("Error sending questions: %v", err)
		return
	}
	return

}

func answerToReply(store *SQLStore, a *Answer, id int) (reply tgbotapi.InlineQueryResultArticle) {
	q, err := store.getQuestion(a.QuestionID)
	if err != nil {
		log.Printf("Error accessing SQL Database %v", err)
		return
	}

	log.Println(a.Date)
	dateText := formatDate(a.Date)
	replyText := fmt.Sprintf(`Информация о ответе %d
*Вопрос : %s*
*Ответивший*: @%s
*Дата*: %s:
*Текст ответа*:
"%s"`, a.AnswerID, q.Text, a.User, dateText, a.Text)

	replyText = markAsBotText(replyText)

	replyTitle := fmt.Sprintf("От @%s в %s", a.User, dateText)

	reply = tgbotapi.NewInlineQueryResultArticleMarkdown(strconv.Itoa(id),
		replyTitle, replyText)
	if len(a.Text) > MaxShownMessageLength {
		reply.Description = fmt.Sprintf("%s...", a.Text[:MaxShownMessageLength])
	} else {
		reply.Description = a.Text
	}
	return
}

func sendSimpleStringReply(bot *tgbotapi.BotAPI, queryId string,
	message string) (err error) {

	titleText := message
	messagesText := markAsBotText(titleText)
	reply := tgbotapi.NewInlineQueryResultArticle("1",
		titleText, messagesText)

	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: queryId,
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

func sendNoAnswersReply(bot *tgbotapi.BotAPI, queryID string) (err error) {
	err = sendSimpleStringReply(bot, queryID, "Нет ответов")
	if err != nil {
		return
	}
	return
}

func sendWrongFormatReply(bot *tgbotapi.BotAPI, queryID string) (err error) {
	err = sendSimpleStringReply(bot, queryID, "Неправильный формат команды")
	if err != nil {
		return
	}
	return
}

func convertQueryOffset(offset_str string) (offset int, err error) {
	if offset_str == "" {
		offset = 0
		return
	}
	offset, err = strconv.Atoi(offset_str)
	if err != nil {
		return
	}
	return
}

func sendListAnswers(bot *tgbotapi.BotAPI, store *SQLStore,
	queryID string, offset int, answers []*Answer) (err error) {
	if len(answers) == 0 {
		if offset == 0 {
			err = sendNoAnswersReply(bot, queryID)
			if err != nil {
				log.Printf("Error while sending no answers reply: %v", err)
				return
			}
			return
		} else {
			err = sendEndReply(bot, queryID)
			if err != nil {
				log.Printf("Error while sending end reply: %v", err)
				return
			}
			return
		}
	}

	err = sendChunkAnswersReply(bot, store, queryID, answers, offset)
	if err != nil {
		log.Printf("Error while sending chunk of answers: %v", err)
		return
	}
	return
}

func sendChunkAnswersReply(bot *tgbotapi.BotAPI, store *SQLStore, queryID string,
	questions []*Answer, offset int) (err error) {
	var replies []interface{}
	for id, a := range questions {
		replies = append(replies, answerToReply(store, a, id))
	}

	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: queryID,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       replies,
		NextOffset:    strconv.Itoa(offset + len(replies)),
	}

	_, err = bot.AnswerInlineQuery(inlineConfig)
	if err != nil {
		return
	}
	return
}

func sendAnswersListReply(bot *tgbotapi.BotAPI,
	store *SQLStore, queryID string, questionID int, offset_str string) (err error) {
	offset, err := convertQueryOffset(offset_str)
	if err != nil {
		log.Printf("Error while converting offset: %v", err)
		return
	}

	answers, err := store.findAnswersFor(questionID, MaxSendInlineObjects, offset)
	if err != nil {
		log.Printf("Error accesing sql store: %v", err)
		return
	}

	err = sendListAnswers(bot, store, queryID, offset, answers)
	if err != nil {
		log.Printf("Error while sending questions: %v", err)
		return
	}
	return
}

func sendListAnswersToUserReply(bot *tgbotapi.BotAPI, store *SQLStore,
	queryID string, user string, offset_str string) (err error) {
	offset, err := convertQueryOffset(offset_str)
	if err != nil {
		log.Printf("Error while converting offset: %v", err)
		return
	}
	answers, err := store.getAnswersFor(user, MaxSendInlineObjects, offset)

	if err != nil {
		log.Printf("Error accessing database: %v", err)
		return
	}

	err = sendListAnswers(bot, store, queryID, offset, answers)
	if err != nil {
		log.Printf("Error while sending questions: %v", err)
	}
	return
}

func sendUserQuestionListReply(bot *tgbotapi.BotAPI,
	store *SQLStore, queryID string, user string, offset_str string) (err error) {
	offset, err := convertQueryOffset(offset_str)
	if err != nil {
		log.Printf("Error while converting offset: %v", err)
		return
	}

	questions, err := store.findQuestionsFrom(user, MaxSendInlineObjects, offset)
	if err != nil {
		log.Printf("Error accessing sql store: %v", err)
		return
	}
	err = sendQuestionList(bot, queryID, offset, questions, simpleQuestionToReply)
	if err != nil {
		log.Printf("Error sending questions: %v", err)
		return
	}
	return
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
	command, commandArgs := parseQuery(query)

	switch command {
	case "list_questions":
		err = sendQuestionListReply(bot, store, update.InlineQuery.ID, AllGroupName,
			update.InlineQuery.Offset)
		if err != nil {
			log.Printf("Error while sending questions list reply: %v", err)
			return
		}
		return

	case "list_questions_to_me":
		err = sendQuestionListReply(bot, store, update.InlineQuery.ID,
			update.InlineQuery.From.UserName, update.InlineQuery.Offset)
		if err != nil {
			log.Printf("Error while sending questions list reply: %v", err)
			return
		}
		return
	case "list_answers":
		var questionID int
		questionID, err = parseListAnswersArgs(commandArgs)
		if err != nil {
			err = sendWrongFormatReply(bot, update.InlineQuery.ID)
			if err != nil {
				log.Printf("Error while sending bad command format reply")
				return
			}
			return
		}
		err = sendAnswersListReply(bot, store, update.InlineQuery.ID,
			questionID, update.InlineQuery.Offset)
		if err != nil {
			log.Printf("Error while sending answers list reply: %v", err)
			return
		}
		return

	case "list_answers_to_me":
		err = sendListAnswersToUserReply(bot, store, update.InlineQuery.ID,
			update.InlineQuery.From.UserName, update.InlineQuery.Offset)
		if err != nil {
			log.Printf("Error while sending answers list reply %v", err)
			return
		}
		return
	case "list_my_questions":
		err = sendUserQuestionListReply(bot,
			store, update.InlineQuery.ID, update.InlineQuery.From.UserName, update.InlineQuery.Offset)
		if err != nil {
			log.Printf("Error sending list_my_questions reply")
		}
		return
	case "question":
		err = sendAddQuestionToAllReply(bot, update.InlineQuery)
		if err != nil {
			log.Printf("Error sending question reply: %v", err)
		}
		return
	case "question_to":
		err = sendAddQuestionToUserReply(bot, update.InlineQuery)
		if err != nil {
			log.Printf("Error sending question_to reply: %v", err)
		}
		return
	case "answer":
		err = sendAddAnswerReply(bot, update.InlineQuery)
		if err != nil {
			log.Printf("Error sending answer reply: %v", err)
		}
		return
	case "close_my":
		err = sendCloseReply(bot, store, update.InlineQuery, "my")
		if err != nil {
			log.Printf("Error sending close_my reply: %v", err)
		}
		return
	case "close_to":
		err = sendCloseReply(bot, store, update.InlineQuery, "to")
		if err != nil {
			log.Printf("Error sending close_to reply: %v", err)
		}
		return
	case "a_close":

		err = sendCloseReply(bot, store, update.InlineQuery, "admin")
		if err != nil {
			log.Printf("Error sending a_close reply: %v", err)
		}
		return
	case "open_my":
		fallthrough
	case "open_to":
		fallthrough
	case "a_open":
		fallthrough
	default:
		err = sendNotExistReply(bot, update)
		if err != nil {
			log.Printf("Error while sending not exists reply")
			return
		}
		return
	}
	return
}

func sendCloseReply(bot *tgbotapi.BotAPI, store *SQLStore,
	query *tgbotapi.InlineQuery, accessType string) (err error) {
	if !inGroup(appConfig.Admins, query.From.UserName) {
		sendSimpleStringReply(bot, query.ID, "Недостаточные права")
		err = NotEnoughPermissions
		return
	}

	offset, err := convertQueryOffset(query.Offset)
	if err != nil {
		log.Printf("Error converting query offset")
		return
	}

	var questions []*Question
	switch accessType {
	case "my":
		questions, err = store.findQuestionsFrom(query.From.UserName, MaxSendInlineObjects, offset)
	case "to":
		questions, err = store.findQuestionsTo(query.From.UserName, MaxSendInlineObjects, offset)
	case "admin":
		questions, err = store.findQuestionsTo(AllGroupName, MaxSendInlineObjects, offset)
	default:
		err = WrongValue
		log.Printf("Wrong value for accessType: %v", accessType)
		return
	}

	if err != nil {
		log.Printf("Error accesing database: %v", err)
		return
	}

	converter := func(q *Question, id int) (reply tgbotapi.InlineQueryResultArticle) {
		reply = simpleQuestionToReply(q, id)
		yesTag := makeCallbackData(CallbackCloseCommand, strconv.Itoa(q.QuestionID))
		appendReply(&reply, yesTag, "Закрыть вопрос")
		return
	}

	err = sendQuestionList(bot, query.ID, offset, questions, converter)
	if err != nil {
		log.Println("Error sending question list")
		return
	}
	return
}

func sendAddAnswerReply(bot *tgbotapi.BotAPI, query *tgbotapi.InlineQuery) (err error) {
	answer, err := parseAnswerQuery(query)
	if err != nil {
		err = sendWrongFormatReply(bot, query.ID)
		if err != nil {
			log.Printf("Error while sending wrong format query: %v", err)
		}
		return
	}

	err = sendAddMessageReply(bot, query.ID, answer)
	if err != nil {
		log.Printf("Error sending answer inline query :%v", err)
		return
	}
	return
}

func parseAnswerQuery(query *tgbotapi.InlineQuery) (answer *Answer, err error) {
	_, args_str := parseQuery(query.Query)
	args_lst := strings.SplitN(args_str, " ", 2)
	if len(args_lst) != 2 {
		err = WrongCommandFormat
		return
	}
	questionID, err := strconv.Atoi(args_lst[0])
	if err != nil {
		return
	}
	answer = &Answer{
		AnswerID:   -1,
		User:       query.From.UserName,
		Text:       args_lst[1],
		Date:       time.Now(),
		QuestionID: questionID,
	}
	return

}

func sendAddQuestionToUserReply(bot *tgbotapi.BotAPI, query *tgbotapi.InlineQuery) (err error) {
	question, err := parseQuestionToQuery(query)
	if err != nil {
		err = sendWrongFormatReply(bot, query.ID)
		if err != nil {
			log.Printf("Error while sending wrong format query: %v", err)
		}
		return
	}

	err = sendAddMessageReply(bot, query.ID, question)
	if err != nil {
		log.Printf("Error sending answer inline query :%v", err)
		return
	}
	return
}

func sendAddQuestionToAllReply(bot *tgbotapi.BotAPI, query *tgbotapi.InlineQuery) (err error) {
	question, err := parseQuestionQuery(query)
	if err != nil {
		err = sendWrongFormatReply(bot, query.ID)
		if err != nil {
			log.Printf("Error while sending wrong format query: %v", err)
		}
		return
	}

	err = sendAddMessageReply(bot, query.ID, question)
	if err != nil {
		log.Printf("Error sending answer inline query :%v", err)
		return
	}

	return
}

func sendAddMessageReply(bot *tgbotapi.BotAPI, queryID string, message Message) (err error) {
	tag := messagePull.addMessage(message)
	log.Println(tag)
	data := makeCallbackData(CallbackAddCommand, tag)

	messageText := fmt.Sprintf(markAsBotText("Нажмите на кнопку, чтобы подтвердить действие"))

	reply := tgbotapi.NewInlineQueryResultArticleMarkdown("1",
		"Отправить", messageText)

	replyMarkup := tgbotapi.NewInlineKeyboardMarkup([]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Подтвердить",
			data)})

	reply.ReplyMarkup = &replyMarkup

	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: queryID,
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

func parseQuestionQuery(query *tgbotapi.InlineQuery) (question *Question, err error) {
	_, questionText := parseQuery(query.Query)
	if strings.TrimSpace(questionText) == "" {
		err = WrongCommandFormat
		return
	}
	question = &Question{
		User:       query.From.UserName,
		Text:       questionText,
		Date:       time.Now().UTC(),
		Rec:        &Receiver{AllGroupName},
		Answers:    []*Answer{},
		IsClosed:   false,
		ChatID:     -1,
		QuestionID: -1,
	}
	return
}

func parseQuestionToQuery(query *tgbotapi.InlineQuery) (question *Question, err error) {
	_, argsStr := parseQuery(query.Query)
	args := strings.SplitN(argsStr, " ", 2)
	if len(args) != 2 {
		err = WrongCommandFormat
		return
	}
	questionRecName := strings.Replace(args[0], "@", "", -1)
	questionText := args[1]
	if strings.TrimSpace(questionText) == "" || strings.TrimSpace(questionRecName) == "" {
		err = WrongCommandFormat
		return
	}

	question = &Question{
		User:       query.From.UserName,
		Text:       questionText,
		Date:       time.Now().UTC(),
		Rec:        &Receiver{questionRecName},
		Answers:    []*Answer{},
		IsClosed:   false,
		ChatID:     InlineChatID,
		QuestionID: -1,
	}
	return
}

func (s *SQLStore) findQuestionsFrom(user string,
	limit int, offset int) (questions []*Question, err error) {

	rows, err := s.db.Query(`SELECT id, user, content, time,  receiver, isClosed, chatID
                            FROM Questions WHERE user = ? AND isClosed = 0 LIMIT ? OFFSET ?`,
		user, limit, offset)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var q Question
		var unixTime int64
		var rec_name string
		err = rows.Scan(&q.QuestionID, &q.User, &q.Text, &unixTime, &rec_name, &q.IsClosed, &q.ChatID)
		if err != nil {
			log.Println(err)
			return
		}
		q.Rec = NewReceiver(rec_name)
		q.Date = time.Unix(unixTime, 0).UTC()
		questions = append(questions, &q)
	}
	return
}

func (s *SQLStore) getAnswersFor(user string, limit int, offset int) (answers []*Answer, err error) {
	rows, err := s.db.Query(`SELECT id, user, content, time, questionID
                      FROM Answers
		              WHERE Answers.questionID
		              IN (SELECT id FROM Questions
		                  WHERE user = ?) LIMIT ? OFFSET ?`, user, limit, offset)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var answer Answer
		var date int64
		err = rows.Scan(&answer.AnswerID, &answer.User, &answer.Text, &date, &answer.QuestionID)
		if err != nil {
			return
		}
		answer.Date = time.Unix(date, 0)
		answers = append(answers, &answer)
	}
	return
}

func parseListAnswersArgs(argsString string) (questionID int, err error) {
	if strings.Contains(strings.TrimSpace(argsString), " ") {
		err = WrongCommandFormat
		return
	}
	questionID, err = strconv.Atoi(argsString)
	if err != nil {
		return
	}
	return
}
