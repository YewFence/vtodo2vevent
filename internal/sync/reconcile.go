package sync

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"baikal-vtodo-sync/internal/caldav"
	"baikal-vtodo-sync/internal/config"
	"baikal-vtodo-sync/internal/model"
	"baikal-vtodo-sync/internal/transform"
)

type Reconciler struct {
	client *caldav.Client
	cfg    config.Config
}

func NewReconciler(client *caldav.Client, cfg config.Config) *Reconciler {
	return &Reconciler{client: client, cfg: cfg}
}

func (r *Reconciler) Run(ctx context.Context) error {
	todos, err := r.client.ListTodos(ctx, r.cfg.Server.TodoCollection)
	if err != nil {
		return fmt.Errorf("list todos: %w", err)
	}

	events, err := r.client.ListProjectedEvents(ctx, r.cfg.Server.EventCollection)
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
	return r.client.PutEvent(ctx, r.cfg.Server.EventCollection, event, r.cfg.Sync.AlarmMinutesBefore)
}

func (r *Reconciler) updateEvent(ctx context.Context, event model.Event) error {
	if r.cfg.Sync.DryRun {
		log.Printf("[dry-run] update event uid=%s summary=%q", event.UID, event.Summary)
		return nil
	}
	return r.client.PutEvent(ctx, r.cfg.Server.EventCollection, event, r.cfg.Sync.AlarmMinutesBefore)
}

func (r *Reconciler) deleteEvent(ctx context.Context, event model.Event) error {
	if r.cfg.Sync.DryRun {
		log.Printf("[dry-run] delete event path=%s uid=%s", event.Path, event.UID)
		return nil
	}
	if strings.TrimSpace(event.Path) == "" {
		return fmt.Errorf("cannot delete event %s without path", event.UID)
	}
	return r.client.DeleteEvent(ctx, event.Path)
}

func eventsEqual(a, b model.Event) bool {
	return a.UID == b.UID &&
		a.Summary == b.Summary &&
		a.Description == b.Description &&
		a.SourceTodoUID == b.SourceTodoUID &&
		a.StartAt.Equal(b.StartAt) &&
		a.EndAt.Equal(b.EndAt)
}
