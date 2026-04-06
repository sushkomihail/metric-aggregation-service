package service

import (
	"context"
	"fmt"
	"time"
)

type Processor struct {
	timeWindow time.Duration
}

func NewProcessor(timeWindow time.Duration) *Processor {
	return &Processor{
		timeWindow: timeWindow,
	}
}

func (p *Processor) Run(ctx context.Context) {
	ticker := time.NewTicker(p.timeWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			fmt.Println("tick at: ", t)
			processMetrics()
		}
	}
}

func processMetrics() {
	fmt.Println("metrics processed")
}
