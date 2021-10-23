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

import (
	"context"
	"fmt"
	"time"

	"github.com/kargirwar/prosql-agent/utils"
)

type accessTimeInterface interface {
	setAccessTime()
	getId() string
}

const UPDATE_TIME = 10 //seconds

func startTimer(ctx context.Context, p accessTimeInterface) {
	ticker := time.NewTicker(UPDATE_TIME * time.Second)
	defer ticker.Stop()
	utils.Dbg(ctx, fmt.Sprintf("Starting accesstimer for %s", p.getId()))
loop:
	for {
		select {
		case <-ctx.Done():
			utils.Dbg(ctx, fmt.Sprintf("Stopping accesstimer for %s", p.getId()))
			break loop

		case <-ticker.C:
			utils.Dbg(ctx, fmt.Sprintf("Setting accesstime for %s", p.getId()))
			p.setAccessTime()
		}
	}
}
