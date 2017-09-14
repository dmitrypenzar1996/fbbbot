package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"time"
)

// creates new SQLStore instance, connects to sqldatabase and creates, if not exists, required tables
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
	err = store.createUsersTable()
	if err != nil {
		return
	}
	return
}

// stop connection to sql database
func (s *SQLStore) Close() {
	err := s.db.Close()
	if err != nil {
		log.Printf("Error while closing sqlstore: %v", err)
	}
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

func (s *SQLStore) createUsersTable() (err error) {
	creationQuery := `
	CREATE TABLE IF NOT EXISTS Users(
	    id integer,
	    name text primary key,
	    chatID
	)`
	_, err = s.db.Exec(creationQuery)
	if err != nil {
		return
	}
	return
}

type User struct {
	ID     int
	Name   string
	ChatID int64
}

func (s *SQLStore) addUser(user *User) (err error) {
	s.Lock()
	defer s.Unlock()
	tx, err := s.db.Begin()
	defer tx.Commit()
	_, err = tx.Exec(`INSERT INTO Users (id, name, chatID)
	                VALUES (?, ?, ?)`, user.ID, user.Name, user.ChatID)
	if err != nil {
		return
	}
	return
}

func (s *SQLStore) getUserChatID(userName string) (chatID int64, err error) {
	row := s.db.QueryRow(`SELECT chatID FROM Users
                                      WHERE name = ?`, userName)
	err = row.Scan(&chatID)
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

func (s *SQLStore) findQuestionsTo(receiver string,
	limit int, offset int) (questions []*Question, err error) {

	rows, err := s.db.Query(`SELECT id, user, content, time,  receiver, isClosed, chatID
                            FROM Questions
                                WHERE receiver = ? AND isClosed = 0
                            ORDER BY time DESC
                            LIMIT ?
                            OFFSET ?`,
		receiver, limit, offset)
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

func (s *SQLStore) findAnswersFor(questionID int,
	limit int, offset int) (answers []*Answer, err error) {
	rows, err := s.db.Query(`SELECT id, user, content, time, questionID
                                   FROM Answers
                                   WHERE questionID = ?
                                   ORDER BY time DESC
                                   LIMIT ?
                                   OFFSET ?`,
		questionID, limit, offset)
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
		log.Println(unixTime)
		a.Date = time.Unix(unixTime, 0)
		answers = append(answers, &a)
	}
	return
}

func (s *SQLStore) findAllQuestionsTo(receiver string) (questions []*Question, err error) {
	rows, err := s.db.Query(`SELECT id, user, content, time,  receiver, isClosed, chatID
                            FROM Questions
                                WHERE receiver = ? AND isClosed = 0
                            ORDER BY time DESC`, receiver)
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
	rows, err := s.db.Query(`SELECT id, user, content, time, questionID
                                   FROM Answers
                                   WHERE questionID = ?
                                   ORDER BY time DESC`,
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

func (s *SQLStore) findQuestionsFrom(user string,
	limit int, offset int) (questions []*Question, err error) {

	rows, err := s.db.Query(`SELECT id, user, content, time,  receiver, isClosed, chatID
                            FROM Questions WHERE user = ? AND isClosed = 0
                            ORDER BY time DESC
                            LIMIT ?
                            OFFSET ?`,
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
		                  WHERE user = ?)
		              ORDER BY time DESC
		              LIMIT ?
		              OFFSET ?`, user, limit, offset)
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
