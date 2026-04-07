package sync

import (
	"context"
	"log"
	"time"
)

type Service struct {
	reconciler *Reconciler
	interval   time.Duration
}

func NewService(reconciler *Reconciler, interval time.Duration) *Service {
	return &Service{
		reconciler: reconciler,
		interval:   interval,
	}
}

func (s *Service) Run(ctx context.Context) error {
	if s.interval <= 0 {
		return s.reconciler.Run(ctx)
	}

	log.Printf("continuous sync enabled, poll interval=%s", s.interval)
	s.runCycle(ctx)
	if ctx.Err() != nil {
		return nil
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.runCycle(ctx)
		}
	}
}

func (s *Service) runCycle(ctx context.Context) {
	startedAt := time.Now()
	log.Printf("sync cycle started")

	if err := s.reconciler.Run(ctx); err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Printf("sync cycle failed after %s: %v", time.Since(startedAt).Round(time.Millisecond), err)
		return
	}

	log.Printf("sync cycle completed in %s", time.Since(startedAt).Round(time.Millisecond))
}
