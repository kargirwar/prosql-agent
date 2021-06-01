package main

import "time"

const PORT = 23890

//pool
const MAX_OPEN_CONNS = 500
const MAX_IDLE_CONNS = 100
const MAX_IDLE_CONNS_AT_START = 10

const BATCH_SIZE = 5000

const SESSION_CLEANUP_INTERVAL = 20 * time.Minute
const CURSOR_CLEANUP_INTERVAL = 1 * time.Minute

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
const CMD_FETCH_WS = "fetch-ws"
const CMD_CANCEL = "cancel"
const CMD_CLEANUP = "cleanup"
const CMD_SET_DB = "set-db"

//statuses
const SUCCESS = "success"
const ERROR = "error"
const CLEANUP_DONE = "cleanup-done"
