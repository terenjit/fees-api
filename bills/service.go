package bills

import (
	"context"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

//encore:service
type Service struct {
	tc client.Client
	w  worker.Worker
}

func initService() (*Service, error) {
	tc, err := client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
	})
	if err != nil {
		return nil, err
	}

	w := worker.New(tc, TaskQueue, worker.Options{})
	w.RegisterWorkflow(BillWorkflow)

	if err := w.Start(); err != nil {
		tc.Close()
		return nil, err
	}

	return &Service{tc: tc, w: w}, nil
}

func (s *Service) Shutdown(_ context.Context) {
	s.w.Stop()
	s.tc.Close()
}
