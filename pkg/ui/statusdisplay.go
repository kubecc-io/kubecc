package ui

import (
	"context"
	"fmt"
	"log"
	"sync"

	mstat "github.com/cobalt77/kubecc/pkg/metrics/status"
	"github.com/cobalt77/kubecc/pkg/types"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type agent struct {
	ctx         context.Context
	uuid        string
	queueParams *mstat.QueueParams
	taskStatus  *mstat.TaskStatus
	queueStatus *mstat.QueueStatus
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
	header := []string{"ID", "Running Tasks", "Queued Tasks", "Queue Status"}
	rows = append(rows, header)
	for _, agent := range s.agents {
		row := []string{
			agent.uuid,
			fmt.Sprintf("%d/%d", agent.taskStatus.NumRunningProcesses, agent.queueParams.MaxRunningProcesses),
			fmt.Sprint(agent.taskStatus.NumQueuedProcesses),
			types.QueueStatus(agent.queueStatus.QueueStatus).String(),
		}
		rows = append(rows, row)
	}
	return rows
}

func (s *StatusDisplay) AddAgent(ctx context.Context, uuid string) {
	s.mutex.Lock()
	s.agents = append(s.agents, &agent{
		ctx:         ctx,
		uuid:        uuid,
		queueParams: &mstat.QueueParams{},
		taskStatus:  &mstat.TaskStatus{},
		queueStatus: &mstat.QueueStatus{},
	})
	s.mutex.Unlock()
	s.redraw()

	go func() {
		<-ctx.Done()
		s.mutex.Lock()
		for i, a := range s.agents {
			if a.uuid == uuid {
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
		if a.uuid == uuid {
			index = i
		}
	}
	switch p := params.(type) {
	case *mstat.QueueParams:
		s.agents[index].queueParams = p
	case *mstat.TaskStatus:
		s.agents[index].taskStatus = p
	case *mstat.QueueStatus:
		s.agents[index].queueStatus = p
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
