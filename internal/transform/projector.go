package transform

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"baikal-vtodo-sync/internal/model"
)

type ProjectOptions struct {
	DueOnlyDuration time.Duration
}

func ProjectTodo(todo model.Todo, opts ProjectOptions) (model.Event, bool, error) {
	if !todo.ShouldProject() {
		return model.Event{}, false, nil
	}
	if todo.UID == "" {
		return model.Event{}, false, fmt.Errorf("todo uid is empty")
	}

	var start time.Time
	var end time.Time

	switch {
	case todo.StartAt != nil && todo.DueAt != nil:
		start = *todo.StartAt
		end = *todo.DueAt
	case todo.DueAt != nil:
		start = *todo.DueAt
		end = start.Add(opts.DueOnlyDuration)
	default:
		return model.Event{}, false, nil
	}

	if !end.After(start) {
		return model.Event{}, false, fmt.Errorf("event end must be after start for todo %s", todo.UID)
	}

	description := strings.TrimSpace(todo.Description)
	if description != "" {
		description += "\n\n"
	}
	description += fmt.Sprintf("Source todo UID: %s", todo.UID)

	return model.Event{
		UID:           derivedEventUID(todo.UID),
		Summary:       todo.Summary,
		Description:   description,
		StartAt:       start,
		EndAt:         end,
		SourceTodoUID: todo.UID,
	}, true, nil
}

func derivedEventUID(todoUID string) string {
	sum := sha1.Sum([]byte(todoUID))
	return "vtodo-sync-" + hex.EncodeToString(sum[:])
}
