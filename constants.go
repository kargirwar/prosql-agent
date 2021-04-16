package main

import "time"

const SESSION_CLEANUP_INTERVAL = 20 * time.Second
const CURSOR_CLEANUP_INTERVAL = 20 * time.Second

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
const CMD_CANCEL = "cancel"
const CMD_CLEANUP = "cleanup"

//statuses
const SUCCESS = "success"
const ERROR = "error"
const CLEANUP_DONE = "cleanup-done"
