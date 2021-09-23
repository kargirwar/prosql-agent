/* Copyright (C) 2021 Pankaj Kargirwar <kargirwar@protonmail.com>

   This file is part of prosql-agent

   prosql-agent is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   prosql-agent is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with prosql-agent.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import "time"

const APP_NAME = "prosql-agent"
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
const CMD_QUERY = "query"
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
