package main

//error codes
const ERR_INVALID_USER_INPUT = "invalid-user-input"
const ERR_INVALID_SESSION_ID = "invalid-session-id"
const ERR_DB_ERROR = "db-error"
const ERR_UNRECOVERABLE = "unrecoverable-error"
const ERR_INVALID_CURSOR_ID = "invalid-cursor-id"
const ERR_NO_DATA = "no-data"
const ERR_INVALID_CURSOR_CMD = "invalid-cursor-cmd"
const EOF = "eof"

//commands
const CMD_EXECUTE = "execute"
const CMD_FETCH = "fetch"

//statuses
const SUCCESS = "success"
const ERROR = "error"
