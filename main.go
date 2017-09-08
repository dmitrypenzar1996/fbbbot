package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/telegram-bot-api.v4"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type QuestionList []*Question

type Question struct {
	User       string
	Text       string
	Date       time.Time
	Rec        *Receiver
	Answers    []*Answer
	IsClosed   bool
	ChatID     int64
	QuestionID int
}

type Answer struct {
	User       string
	Text       string
	Date       time.Time
	QuestionID int
	AnswerID   int
}

type Receiver struct {
	User string
}

type SQLStore struct {
	db   *sql.DB
	path string
	sync.Mutex
}

type AppConfig struct {
	TelegramBotToken string
	Admins           []string
}

var appConfig *AppConfig

var QuestionDoesntExist = errors.New("Question with such ID doesn't exist")
var AnswerDoesntExist = errors.New("Answer with such ID doesn't exist")
var WrongCommandFormat = errors.New("Wrong command format")
var NotEnoughPermissions = errors.New("User has not enough permission for that action")
var NoteDoesntExist = errors.New("Note with such ID doesn't exist")

const appConfigPath string = "config.json"
const allGroupName string = "all"

func init() {
	var err error
	log.Println("Loading app config")
	appConfig, err = readAppConfig()
	if err != nil {
		log.Printf("Error occured at init : %v", err)
		log.Fatal("Cant run app due to fatal errors during init")
	}
}

func main() {
	sqlstore, err := NewSQLStore("botbase.sql")
	if err != nil {
		log.Fatal(err)
	}
	defer sqlstore.db.Close()
	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramBotToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		go processUpdate(bot, &update, sqlstore)
	}

}

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

func NewSQLStore(path string) (store *SQLStore, err error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return
	}
	store = &SQLStore{db: db, path: path}
	err = store.createQuestionsTable()
	if err != nil {
		return
	}
	err = store.createAnswersTable()
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) createQuestionsTable() (err error) {
	creationQuery := `
	create table if not exists Questions(
		id integer primary key,
		user text,
		content text,
		time integer,
		receiver text,
		isClosed integer,
		chatID integer
	)`
	_, err = s.db.Exec(creationQuery)
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) createAnswersTable() (err error) {
	creationQuery := `
	create table if not exists Answers(
		id integer primary key,
		user text,
		content text,
		time integer,
		questionID integer
	)`
	_, err = s.db.Exec(creationQuery)
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) createNotesTable() (err error) {
	creationQuery := `
	CREATE TABLE IF NOT EXISTS Notes (
	    id integer primary key
	    user text
	    content text
	    time integer
	)`
	_, err = s.db.Exec(creationQuery)
	if err != nil {
		return
	}
	return
}

type Note struct {
	NoteID int
	User   string
	Text   string
	Date   time.Time
}

func (s *SQLStore) addNote(n *Note) (err error) {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer func(tx *sql.Tx, err *error) {
		*err = tx.Commit()
	}(tx, &err)
	insertQuery, err := tx.Prepare(`
	    INSERT INTO Notes (user, content, time)
	    VALUES (?, ?, ?)`)
	if err != nil {
		return
	}
	defer insertQuery.Close()
	_, err = insertQuery.Exec(n.User, n.Text, n.Date.Unix())
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) getNote(noteId int) (note *Note, err error) {
	rows, err := s.db.Query(`SELECT (id, user, content, time)
                                      FROM Notes
                                          WHERE id = ?`, noteId)
	defer rows.Close()
	if rows.Next() {

	} else {
		err = NoteDoesntExist
	}
	return
}

func (s *SQLStore) closeQuestion(questionID int) (err error) {
	_, err = s.db.Exec("UPDATE Questions SET isClosed = 1 WHERE id = ?",
		questionID)
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) openQuestion(questionID int) (err error) {
	_, err = s.db.Exec("UPDATE Questions SET isClosed = 0 WHERE id = ?",
		questionID)
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) findAllQuestionsTo(receiver string) (questions []*Question, err error) {
	rows, err := s.db.Query(`SELECT id, user, content, time,  receiver, isClosed, chatID
                            FROM Questions WHERE receiver = ? AND isClosed = 0`, receiver)
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

// here and below - the more convenient way to do writing is to orgаnize chanel and
// read by specific writer from him and do all writings at the same time, but for
// our application it isn't necessary

func (s *SQLStore) addQuestion(q *Question) (questionID int, err error) {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer func(tx *sql.Tx, err *error) {
		*err = tx.Commit()
	}(tx, &err)

	insertQuery, err := tx.Prepare(`
	INSERT INTO Questions 
	    (user, content, time, receiver, isClosed, chatID)
		    VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Println(err)
		return
	}
	defer insertQuery.Close()
	if err != nil {
		return
	}
	result, err := insertQuery.Exec(q.User, q.Text, q.Date.Unix(),
		q.Rec.User, q.IsClosed, q.ChatID)
	if err != nil {
		return
	}

	questionID64, err := result.LastInsertId()
	questionID = int(questionID64)
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) addAnswer(a *Answer) (answerID int, err error) {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer func(tx *sql.Tx, err *error) {
		*err = tx.Commit()
	}(tx, &err)

	insertQuery, err := tx.Prepare(
		`INSERT INTO Answers
		     (user, content, time, questionID)
			 VALUES (?, ?, ?, ?)`)
	if err != nil {
		return
	}
	defer insertQuery.Close()

	result, err := insertQuery.Exec(a.User, a.Text, a.Date.Unix(), a.QuestionID)
	if err != nil {
		return
	}

	answerID64, err := result.LastInsertId()
	answerID = int(answerID64)
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) deleteQuestion(questionID int) (err error) {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer func(tx *sql.Tx, err *error) {
		*err = tx.Commit()
	}(tx, &err)

	delete_query, err := tx.Prepare("DELETE FROM Questions WHERE id = ?")
	defer delete_query.Close()
	if err != nil {
		return
	}
	_, err = delete_query.Exec(questionID)
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) deleteAnswer(answerID int) (err error) {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer func(tx *sql.Tx, err *error) {
		*err = tx.Commit()
	}(tx, &err)

	delete_query, err := tx.Prepare("DELETE FROM Answers WHERE id = ?")
	defer delete_query.Close()
	if err != nil {
		return
	}
	_, err = delete_query.Exec(answerID)
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) findAllAnswersFor(questionID int) (answers []*Answer, err error) {
	rows, err := s.db.Query("SELECT id, user, content, time, questionID FROM Answers WHERE questionID = ?",
		questionID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var a Answer
		var unixTime int64
		err = rows.Scan(&a.AnswerID, &a.User, &a.Text, &unixTime, &a.QuestionID)
		if err != nil {
			return
		}
		a.Date = time.Unix(unixTime, 0)
		answers = append(answers, &a)
	}
	return
}

func (s *SQLStore) getQuestion(questionID int) (q *Question, err error) {
	rows, err := s.db.Query("SELECT id, user, content, time,  receiver, isClosed, chatID FROM Questions WHERE id = ?", questionID)
	if err != nil {
		return
	}
	defer rows.Close()
	q = new(Question)
	var unixTime int64
	var recName string
	if rows.Next() {
		err = rows.Scan(&q.QuestionID, &q.User, &q.Text, &unixTime, &recName, &q.IsClosed,
			&q.ChatID)
		if err != nil {
			return
		}
	} else {
		err = QuestionDoesntExist
		return
	}
	q.Date = time.Unix(unixTime, 0)
	q.Rec = &Receiver{recName}
	return
}

func (s *SQLStore) getAnswer(answerID int) (a *Answer, err error) {
	rows, err := s.db.Query("SELECT id, user, content, time, questionID FROM Answers WHERE id = ?", answerID)
	if err != nil {
		return
	}
	defer rows.Close()
	a = new(Answer)
	var unixTime int64
	if rows.Next() {
		err = rows.Scan(&a.AnswerID, &a.User, &a.Text, &unixTime, &a.QuestionID)
		if err != nil {
			return
		}
	} else {
		err = AnswerDoesntExist
		return
	}
	a.Date = time.Unix(unixTime, 0)
	return
}

func NewReceiver(user string) *Receiver {
	return &Receiver{user}
}

func parseSlashQuestion(m *tgbotapi.Message) (q *Question, err error) {
	q = new(Question)
	q.Date = m.Time().UTC()
	q.User = m.From.UserName
	q.Rec = NewReceiver(allGroupName)
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

func parseSlashQuestionTo(m *tgbotapi.Message) (q *Question, err error) {
	q = new(Question)
	q.Date = m.Time().UTC()
	q.User = m.From.UserName
	cmd_args := strings.SplitN(m.CommandArguments(), " ", 3)
	if len(cmd_args) != 3 {
		err = WrongCommandFormat
		return
	}
	q.Rec = NewReceiver(strings.Replace(cmd_args[0], "@", "", -1))
	q.Text = cmd_args[1]
	q.Answers = []*Answer{}
	q.IsClosed = false
	q.ChatID = m.Chat.ID
	q.QuestionID = -1
	return
}

func questionCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	q, err := parseSlashQuestion(m)
	if err != nil {
		log.Printf("Unvalid command format\n")
		reply = "Неверный формат команды"
		return
	}
	questionID, err := store.addQuestion(q)
	if err != nil {
		log.Printf("Error while adding question : %v\n", err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = fmt.Sprintf("Вопрос принят, его id: %d", questionID)
	return
}

func questionToCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	q, err := parseSlashQuestionTo(m)
	if err != nil {
		log.Printf("Error while parsing message")
		reply = "Неправильный формат"
		return
	}
	questionID, err := store.addQuestion(q)
	if err != nil {
		log.Printf("Error while adding question : %v\n", err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = fmt.Sprintf("Вопрос принят, его id: %d", questionID)
	return
}

func inGroup(userList []string, user string) (flag bool) {
	for _, value := range userList {
		if value == user {
			flag = true
			return
		}
	}
	flag = false
	return
}

func closeCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	qID, err := parseSlashClose(m)
	if err != nil {
		log.Print(err)
		reply = "Неверный формат команды"
		return
	}

	question, err := store.getQuestion(qID)
	if err != nil {
		if err == QuestionDoesntExist {
			log.Println(err)
			reply = "Вопрос с таким id не существует"
			return
		} else {
			log.Println(err)
			reply = "Ошибка доступа к базе данных"
			return
		}
	}

	if m.From.UserName != question.User && !(inGroup(appConfig.Admins, m.From.UserName)) {
		err = NotEnoughPermissions
		reply = "Недостаточно прав"
		return
	}

	err = store.closeQuestion(qID)
	if err != nil {
		log.Print(err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = "Вопрос закрыт"
	return
}

func openCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	qID, err := parseSlashOpen(m)
	if err != nil {
		log.Print(err)
		reply = "Неверный формат команды"
		return
	}

	question, err := store.getQuestion(qID)
	if m.From.UserName != question.User && !(inGroup(appConfig.Admins, m.From.UserName)) {
		err = NotEnoughPermissions
		reply = "Недостаточно прав"
		return
	}

	err = store.openQuestion(qID)
	if err != nil {
		log.Print(err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = "Вопрос открыт"
	return
}

func listToMeQuestionsCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	questions, err := store.findAllQuestionsTo(m.From.UserName)
	if err != nil {
		log.Printf("Error while accessing questiong: %v\n", err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = listQuestions(questions)
	return
}

func listQuestionsCommandExec(store *SQLStore) (reply string, err error) {
	questions, err := store.findAllQuestionsTo(allGroupName)
	if err != nil {
		log.Printf("Error with list_questions : %v\n", err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = listQuestions(questions)
	return
}

func answerCommandExec(m *tgbotapi.Message, store *SQLStore, bot *tgbotapi.BotAPI) (reply string, err error) {
	answer, err := parseSlashAnswer(m)
	if err != nil {
		if err == WrongCommandFormat {
			reply = "Неправильный формат команды"
		} else {
			reply = "Неправильный id вопроса"
		}
		return
	}

	question, err := store.getQuestion(answer.QuestionID)
	if err != nil {
		log.Printf("Error in getting question by id %v\n", err)
		if err == QuestionDoesntExist {
			reply = "Вопрос с таким id отсутствует в базе данных"
		} else {
			reply = "Ошибка доступа к базе данных"
		}
		return
	}

	answerID, err := store.addAnswer(answer)
	if err != nil {
		reply = "Ошибка доступа к базе данных"
		return
	}

	if question.Rec.User != allGroupName {
		// answer by Receiver automatically closes question
		err = store.closeQuestion(question.QuestionID)
		if err != nil {
			log.Println(err)
			reply = "Ошибка доступа к базе данных"
			return
		}
	}

	if question.ChatID != m.Chat.ID {
		log.Println("Making asker notification")
		msg := makeAskerNotification(answer, question)
		_, err = bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
	}

	reply = fmt.Sprintf("Ответ сохранен, его id: %d", answerID)
	return
}

func listAnswersCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	questionID, err := parseSlashListAnswers(m)
	if err != nil {
		reply = "Неверный формат команды"
		return
	}
	question, err := store.getQuestion(questionID)
	if err != nil {
		if err == QuestionDoesntExist {
			reply = "Вопроса с таким id нет в базе данных"
		} else {
			reply = "Ошибка доступа к базе данных"
		}
		return
	}
	answers, err := store.findAllAnswersFor(questionID)
	if err != nil {
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = listAnswers(question, answers)
	return
}

func parseSlashDeleteAnswer(m *tgbotapi.Message) (answerID int, err error) {
	if m.CommandArguments() == "" {
		err = WrongCommandFormat
		return
	}
	answerID, err = strconv.Atoi(m.CommandArguments())
	if err != nil {
		return
	}
	return
}

func deleteAnswerCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	answerID, err := parseSlashDeleteAnswer(m)
	if err != nil {
		reply = "Неверный формат команды"
		return
	}
	answer, err := store.getAnswer(answerID)
	if err != nil {
		if err == AnswerDoesntExist {
			reply = "Вопроса с таким id нет в базе данных"
		} else {
			reply = "Ошибка доступа к базе данных"
		}
		return
	}
	if answer.User != m.From.UserName && !(inGroup(appConfig.Admins, m.From.UserName)) {
		reply = "Недостаточно прав"
		err = NotEnoughPermissions
		return
	}
	err = store.deleteAnswer(answerID)
	if err != nil {
		reply = "Ошибка доступа к базе данных"
		return
	}
	return
}

func parseSlashDeleteQuestion(m *tgbotapi.Message) (questionID int, err error) {
	if m.CommandArguments() == "" {
		err = WrongCommandFormat
		return
	}
	questionID, err = strconv.Atoi(m.CommandArguments())
	if err != nil {
		return
	}
	return
}

func deleteQuestionCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	questionID, err := parseSlashDeleteQuestion(m)
	if err != nil {
		reply = "Неверный формат команды"
		return
	}
	question, err := store.getQuestion(questionID)
	if err != nil {
		if err == QuestionDoesntExist {
			reply = "Вопроса с таким id нет в базе данных"
		} else {
			reply = "Ошибка доступа к базе данных"
		}
		return
	}

	if question.User != m.From.UserName && !(inGroup(appConfig.Admins, m.From.UserName)) {
		err = NotEnoughPermissions
		return
	}
	err = store.deleteQuestion(questionID)

	if err != nil {
		return
	}
	return
}

func commandExec(bot *tgbotapi.BotAPI, update *tgbotapi.Update, store *SQLStore) (reply string, err error) {
	switch update.Message.Command() {
	case "start":
		reply = getStartReply()
	case "close":
		reply, err = closeCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "open":
		reply, err = openCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "question":
		reply, err = questionCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "question_to":
		reply, err = questionToCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "list_questions":
		reply, err = listQuestionsCommandExec(store)
		if err != nil {
			break
		}
	case "list_questions_to_me":
		reply, err = listToMeQuestionsCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "answer":
		reply, err = answerCommandExec(update.Message, store, bot)
		if err != nil {
			break
		}
	case "list_answers":
		reply, err = listAnswersCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "delete_answer":
		reply, err = deleteAnswerCommandExec(update.Message, store)
		if err != nil {
			log.Println(err)
			break
		}
	case "delete_question":
		reply, err = deleteQuestionCommandExec(update.Message, store)
		if err != nil {
			log.Println(err)
			break
		}
	case "list_my_answers":
		reply = "Command is not implemented yet"
	case "list_my_questions":
		reply = "Command is not implemented yet"
	case "important":
		reply = "Command is not implemented yet"
	case "list_important":
		reply = "Command is not implemented yet"
	case "delete_important":
		reply = "Command is not implemented yet"
	default:
		log.Printf("%v is uknown command\n", update.Message.Command())
		reply = "Неверная команда"
	}
	return
}

func processUpdate(bot *tgbotapi.BotAPI, update *tgbotapi.Update, store *SQLStore) (err error) {
	if update.Message == nil {
		return
	}
	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

	reply, err := commandExec(bot, update, store)

	if reply != "" {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
		bot.Send(msg)
	}
	return
}

func parseSlashListAnswers(m *tgbotapi.Message) (questionID int, err error) {
	questionStrID := m.CommandArguments()
	if questionStrID == "" {
		err = WrongCommandFormat
		return
	}
	questionID, err = strconv.Atoi(questionStrID)
	if err != nil {
		return
	}
	return
}

func parseSlashAnswer(m *tgbotapi.Message) (answer *Answer, err error) {
	cmd_args := strings.SplitN(m.CommandArguments(), " ", 2)
	if len(cmd_args) < 2 {
		err = WrongCommandFormat
		return
	}
	log.Println(cmd_args)
	quest_id, err := strconv.Atoi(cmd_args[0])
	if err != nil {
		return
	}
	answer = &Answer{
		Text:       cmd_args[1],
		User:       m.From.UserName,
		QuestionID: quest_id,
	}
	answer.Text = cmd_args[1]
	answer.User = m.From.UserName
	answer.QuestionID = quest_id
	return
}

func makeAskerNotification(answer *Answer, question *Question) (msg tgbotapi.MessageConfig) {
	message_text := fmt.Sprintf(
		"На вопрос [%d], заданный @%s:\n        \"%s\"\n появился ответ от @%s:\n        \"%s\"",
		question.QuestionID, question.User, question.Text, answer.User, answer.Text)
	msg = tgbotapi.NewMessage(question.ChatID, message_text)
	return
}

func getStartReply() (reply string) {
	reply = "Привет, я телеграм бот, созданный для управления чатами групп ФББ"
	return
}

func listQuestions(lst QuestionList) (info string) {
	if len(lst) == 0 {
		info = "Нет вопросов"
		return
	}
	questionsInfo := []string{}
	for _, q := range lst {
		info := fmt.Sprintf("[%d] @%s cпросил в  %v:\n    %s", q.QuestionID, q.User, q.Date, q.Text)
		questionsInfo = append(questionsInfo, info)
	}
	info = strings.Join(questionsInfo, "\n")
	return
}

func listAnswers(q *Question, lst []*Answer) (info string) {
	if len(lst) == 0 {
		info = "Нет ответов"
		return
	}
	questionStr := fmt.Sprintf("На ваш вопрос от %v:\n    :%s\n",
		q.Date, q.Text)

	answersInfo := make([]string, len(lst))
	for ind, a := range lst {
		infStr := fmt.Sprintf("@%s ответил в %v:\n    %s", a.User, a.Date, a.Text)
		answersInfo[ind] = infStr
	}
	info = questionStr + strings.Join(answersInfo, "\n")
	return
}

func parseSlashClose(m *tgbotapi.Message) (qID int, err error) {
	if m.CommandArguments() == "" {
		err = WrongCommandFormat
		return
	}
	qID, err = strconv.Atoi(m.CommandArguments())
	if err != nil {
		err = WrongCommandFormat
		return
	}
	return
}

func parseSlashOpen(m *tgbotapi.Message) (qID int, err error) {
	if m.CommandArguments() == "" {
		err = WrongCommandFormat
		return
	}
	qID, err = strconv.Atoi(m.CommandArguments())
	if err != nil {
		err = WrongCommandFormat
		return
	}
	return
}
