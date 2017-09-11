package main

import "errors"

var appConfig *AppConfig

var QuestionDoesntExist = errors.New("Question with such ID doesn't exist")
var AnswerDoesntExist = errors.New("Answer with such ID doesn't exist")
var WrongCommandFormat = errors.New("Wrong command format")
var NotEnoughPermissions = errors.New("User has not enough permission for that action")
var NoteDoesntExist = errors.New("Note with such ID doesn't exist")
var UknownCommand = errors.New("Unknown command")

const appConfigPath string = "config.json"
const AllGroupName string = "all"
const BotMessageSign = "----------------------------------------"
const MaxShownMessageLength = 64
const EmptyMessage = "__ Пустое сообщение"
const NotExistsMessage = "Данной команды не существует"
