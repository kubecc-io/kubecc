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
	"log"
	"time"

	"github.com/gizak/termui/v3"
	ui "github.com/gizak/termui/v3"
	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/types"
	"go.uber.org/atomic"
)

type StatusDisplay struct {
	agents     *Table
	consumerds *Table
	scheduler  *Table
	monitor    *Table
	cache      *Table
	routes     *Tree

	connected *atomic.Bool
}

func NewStatusDisplay(
	ctx context.Context,
	mon types.MonitorClient,
	sch types.SchedulerClient,
) *StatusDisplay {
	s := &StatusDisplay{
		agents:     NewTable(NewAgentDataSource(ctx, mon)),
		consumerds: NewTable(NewConsumerdDataSource(ctx, mon)),
		scheduler:  NewTable(NewSchedulerDataSource(ctx, mon)),
		monitor:    NewTable(NewMonitorDataSource(ctx, mon)),
		cache:      NewTable(NewCacheDataSource(ctx, mon)),
		routes:     NewTree(NewRoutesDataSource(ctx, sch)),
		connected:  atomic.NewBool(false),
	}
	go func() {
		avc := clients.NewAvailabilityChecker(clients.ComponentFilter(types.Monitor))
		clients.WatchAvailability(ctx, mon, avc)
		for {
			ctx := avc.EnsureAvailable()
			s.connected.Store(true)
			<-ctx.Done()
			s.connected.Store(false)
		}
	}()
	return s
}

func (s *StatusDisplay) Run() {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	grid := termui.NewGrid()
	w, h := ui.TerminalDimensions()
	grid.SetRect(0, 0, w, h)

	grid.Set(
		ui.NewRow(0.3,
			ui.NewCol(0.5, s.agents),
			ui.NewCol(0.5, s.consumerds),
		),
		ui.NewRow(0.7,
			ui.NewCol(0.4, s.routes),
			ui.NewCol(0.3, s.cache),
			ui.NewCol(0.3,
				ui.NewRow(0.5, s.monitor),
				ui.NewRow(0.5, s.scheduler),
			),
		),
	)

	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(time.Second / 4).C

	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return
			case "<Resize>":
				payload := e.Payload.(ui.Resize)
				grid.SetRect(0, 0, payload.Width, payload.Height)
				ui.Clear()
				ui.Render(grid)
			}
		case <-ticker:
			ui.Render(grid)
		}
	}
}
