package sync

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"vtodo2vevent/internal/caldav"
	"vtodo2vevent/internal/config"
	"vtodo2vevent/internal/model"
	"vtodo2vevent/internal/transform"
)

type Reconciler struct {
	sourceClient *caldav.Client
	targetClient *caldav.Client
	cfg          config.Config
}

func NewReconciler(sourceClient, targetClient *caldav.Client, cfg config.Config) *Reconciler {
	return &Reconciler{
		sourceClient: sourceClient,
		targetClient: targetClient,
		cfg:          cfg,
	}
}

func (r *Reconciler) Run(ctx context.Context) error {
	todos, err := r.sourceClient.ListTodos(ctx, r.cfg.Source.TodoCollection)
	if err != nil {
		return fmt.Errorf("list todos: %w", err)
	}

	events, err := r.targetClient.ListProjectedEvents(ctx, r.cfg.Target.EventCollection)
	if err != nil {
		return fmt.Errorf("list projected events: %w", err)
	}

	existingBySourceUID := make(map[string]model.Event, len(events))
	for _, event := range events {
		if event.SourceTodoUID == "" {
			continue
		}
		existingBySourceUID[event.SourceTodoUID] = event
	}

	expectedBySourceUID := make(map[string]model.Event)
	for _, todo := range todos {
		event, ok, err := transform.ProjectTodo(todo, transform.ProjectOptions{
			DueOnlyDuration: time.Duration(r.cfg.Sync.DueOnlyDurationMinutes) * time.Minute,
		})
		if err != nil {
			return fmt.Errorf("project todo %s: %w", todo.UID, err)
		}
		if !ok {
			continue
		}
		expectedBySourceUID[todo.UID] = event
	}

	for sourceUID, expected := range expectedBySourceUID {
		existing, found := existingBySourceUID[sourceUID]
		if !found {
			if err := r.createEvent(ctx, expected); err != nil {
				return err
			}
			continue
		}
		if eventsEqual(existing, expected) {
			continue
		}
		expected.Path = existing.Path
		if err := r.updateEvent(ctx, expected); err != nil {
			return err
		}
	}

	for sourceUID, existing := range existingBySourceUID {
		if _, ok := expectedBySourceUID[sourceUID]; ok {
			continue
		}
		if err := r.deleteEvent(ctx, existing); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) createEvent(ctx context.Context, event model.Event) error {
	if r.cfg.Sync.DryRun {
		log.Printf("[dry-run] create event uid=%s summary=%q", event.UID, event.Summary)
		return nil
	}
	return r.targetClient.PutEvent(ctx, r.cfg.Target.EventCollection, event, r.cfg.Sync.AlarmMinutesBefore)
}

func (r *Reconciler) updateEvent(ctx context.Context, event model.Event) error {
	if r.cfg.Sync.DryRun {
		log.Printf("[dry-run] update event uid=%s summary=%q", event.UID, event.Summary)
		return nil
	}
	return r.targetClient.PutEvent(ctx, r.cfg.Target.EventCollection, event, r.cfg.Sync.AlarmMinutesBefore)
}

func (r *Reconciler) deleteEvent(ctx context.Context, event model.Event) error {
	if r.cfg.Sync.DryRun {
		log.Printf("[dry-run] delete event path=%s uid=%s", event.Path, event.UID)
		return nil
	}
	if strings.TrimSpace(event.Path) == "" {
		return fmt.Errorf("cannot delete event %s without path", event.UID)
	}
	return r.targetClient.DeleteEvent(ctx, event.Path)
}

func eventsEqual(a, b model.Event) bool {
	return a.UID == b.UID &&
		a.Summary == b.Summary &&
		a.Description == b.Description &&
		a.SourceTodoUID == b.SourceTodoUID &&
		a.StartAt.Equal(b.StartAt) &&
		a.EndAt.Equal(b.EndAt)
}
