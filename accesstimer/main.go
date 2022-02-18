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

package accesstimer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kargirwar/prosql-agent/utils"
)

type timer struct {
	id         string
	accessTime time.Time
	cancel     context.CancelFunc
}

type timers struct {
	store map[string]*timer
	mutex sync.Mutex
}

func (pt *timers) start(id string) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	t := timer{
		id:         id,
		accessTime: time.Now(),
		cancel:     cancel,
	}
	pt.store[id] = &t

	go start(ctx, &t)
}

func (pt *timers) cancel(id string) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	t, present := pt.store[id]
	if !present {
		utils.Dbg(context.Background(), fmt.Sprintf("Timer %s not found", id))
		return
	}

	t.cancel()
	delete(pt.store, id)
}

func (pt *timers) getAccessTime(id string) time.Time {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	t, present := pt.store[id]
	if !present {
		utils.Dbg(context.Background(), fmt.Sprintf("Timer %s not found", id))
		return time.Time{}
	}

	return t.accessTime
}

var timerStore *timers

const UPDATE_TIME = 10 //seconds

func init() {
	store := make(map[string]*timer)
	timerStore = &timers{
		store: store,
	}
}

func Start(id string) {
	timerStore.start(id)
}

func Cancel(id string) {
	timerStore.cancel(id)
}

func GetAccessTime(id string) time.Time {
	return timerStore.getAccessTime(id)
}

func start(ctx context.Context, t *timer) {
	ticker := time.NewTicker(UPDATE_TIME * time.Second)
	defer ticker.Stop()
	utils.Dbg(ctx, fmt.Sprintf("Starting accesstimer for %s", t.id))
loop:
	for {
		select {
		case <-ctx.Done():
			utils.Dbg(ctx, fmt.Sprintf("Stopping accesstimer for %s", t.id))
			break loop

		case <-ticker.C:
			utils.Dbg(ctx, fmt.Sprintf("Setting accesstime for %s", t.id))
			t.accessTime = time.Now()
		}
	}
}
