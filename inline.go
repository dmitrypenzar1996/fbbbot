package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strconv"
	"strings"
	"time"
)

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
	questions []*Question, offset int) (err error) {
	var replies []interface{}
	for id, q := range questions {
		replies = append(replies, questionToReply(q, id))
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

func sendQuestionList(bot *tgbotapi.BotAPI, store *SQLStore,
	queryID string, offset int, questions []*Question) (err error) {
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

	err = sendChunkQuestionsReply(bot, queryID, questions, offset)
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
	err = sendQuestionList(bot, store, queryID, offset, questions)
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
	err = sendQuestionList(bot, store, queryID, offset, questions)
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
	command, commandArgs := processQuery(query)

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
	case "list_my_questions":
		err = sendUserQuestionListReply(bot,
			store, update.InlineQuery.ID, update.InlineQuery.From.UserName, update.InlineQuery.Offset)
		if err != nil {
			log.Printf("Error sending list_my_questions reply")
		}
	case "question":
		fallthrough
	case "question_to":
		fallthrough
	case "answer":
		fallthrough
	case "delete_answer":
		fallthrough
	case "delete_question":
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
