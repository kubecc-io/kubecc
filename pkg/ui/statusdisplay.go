/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package ui

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type agent struct {
	ctx         context.Context
	info        *types.WhoisResponse
	usageLimits *metrics.UsageLimits
	taskStatus  *metrics.TaskStatus
}

type StatusDisplay struct {
	agents []*agent
	table  *widgets.Table
	mutex  *sync.RWMutex
}

func NewStatusDisplay() *StatusDisplay {
	return &StatusDisplay{
		agents: []*agent{},
		mutex:  &sync.RWMutex{},
	}
}

func (s *StatusDisplay) makeRows() [][]string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rows := make([][]string, 0)
	header := []string{"ID", "Running Tasks", "Queued Tasks"}
	rows = append(rows, header)
	for _, agent := range s.agents {
		row := []string{
			fmt.Sprintf("[%s] %s", agent.info.Component.Name(), agent.info.Address),
			fmt.Sprintf("%d/%d", agent.taskStatus.NumRunning, agent.usageLimits.ConcurrentProcessLimit),
			fmt.Sprint(agent.taskStatus.NumQueued),
		}
		rows = append(rows, row)
	}
	return rows
}

func (s *StatusDisplay) AddAgent(ctx context.Context, info *types.WhoisResponse) {
	s.mutex.Lock()
	s.agents = append(s.agents, &agent{
		ctx:         ctx,
		info:        info,
		usageLimits: &metrics.UsageLimits{},
		taskStatus:  &metrics.TaskStatus{},
	})
	s.mutex.Unlock()
	s.redraw()

	go func() {
		<-ctx.Done()
		s.mutex.Lock()
		for i, a := range s.agents {
			if a.info.UUID == info.UUID {
				s.agents = append(s.agents[:i], s.agents[i+1:]...)
			}
		}
		s.mutex.Unlock()
		s.redraw()
	}()
}

func (s *StatusDisplay) Update(uuid string, params interface{}) {
	s.mutex.Lock()
	var index int
	for i, a := range s.agents {
		if a.info.UUID == uuid {
			index = i
		}
	}
	switch p := params.(type) {
	case *metrics.UsageLimits:
		s.agents[index].usageLimits = p
	case *metrics.TaskStatus:
		s.agents[index].taskStatus = p
	}
	s.mutex.Unlock()
	s.redraw()
}

func (s *StatusDisplay) redraw() {
	s.table.Rows = s.makeRows()
	ui.Render(s.table)
}

func (s *StatusDisplay) Run() {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	s.table = widgets.NewTable()
	termWidth, termHeight := ui.TerminalDimensions()
	s.table.SetRect(0, 0, termWidth, termHeight)

	s.redraw()

	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "q", "<C-c>":
			return
		}
	}
}
