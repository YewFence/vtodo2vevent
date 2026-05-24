package sync_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"vtodo2vevent/internal/caldav"
	"vtodo2vevent/internal/config"
	"vtodo2vevent/internal/model"
	syncer "vtodo2vevent/internal/sync"
)

const (
	testUsername = "syncuser"
	testPassword = "syncpass"
)

func TestReconciler_RunE2EWithRadicale(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要 Docker 的 E2E 测试")
	}

	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	baseURL := startRadicale(t, ctx)
	todoCollection := "/" + testUsername + "/tasks/"
	eventCollection := "/" + testUsername + "/task-reminders/"

	warmUpAuthenticatedRoot(t, ctx, baseURL)
	createCalendarCollection(t, ctx, baseURL, todoCollection, "Tasks", "VTODO")
	createCalendarCollection(t, ctx, baseURL, eventCollection, "Task Reminders", "VEVENT")

	sourceClient := caldav.NewClient(baseURL, testUsername, testPassword, time.UTC)
	targetClient := caldav.NewClient(baseURL, testUsername, testPassword, time.UTC)
	reconciler := syncer.NewReconciler(sourceClient, targetClient, config.Config{
		Source: config.SourceConfig{
			Type:           "caldav",
			BaseURL:        baseURL,
			Username:       testUsername,
			Password:       testPassword,
			TodoCollection: todoCollection,
		},
		Target: config.TargetConfig{
			Type:            "caldav",
			BaseURL:         baseURL,
			Username:        testUsername,
			Password:        testPassword,
			EventCollection: eventCollection,
		},
		Sync: config.SyncConfig{
			Timezone:               "UTC",
			DueOnlyDurationMinutes: 5,
			AlarmMinutesBefore:     10,
		},
	})

	dueAt := time.Date(2026, 4, 8, 2, 0, 0, 0, time.UTC)
	startAt := time.Date(2026, 4, 8, 1, 30, 0, 0, time.UTC)
	todoPath := path.Join(todoCollection, "todo-1.ics")

	putCalendarObject(t, ctx, baseURL, todoPath, buildTodoICS(todoFixture{
		UID:         "todo-1",
		Summary:     "Pay bill",
		Description: "first description",
		DueAt:       &dueAt,
	}))

	runReconcile(t, ctx, reconciler)
	runReconcile(t, ctx, reconciler)

	events := listProjectedEvents(t, ctx, targetClient, eventCollection)
	if len(events) != 1 {
		t.Fatalf("expected 1 projected event after initial sync, got %d", len(events))
	}

	event := events[0]
	if event.SourceTodoUID != "todo-1" {
		t.Fatalf("expected source todo UID todo-1, got %q", event.SourceTodoUID)
	}
	if event.Summary != "Pay bill" {
		t.Fatalf("expected summary %q, got %q", "Pay bill", event.Summary)
	}
	if !event.StartAt.Equal(dueAt) {
		t.Fatalf("expected due-only event start %s, got %s", dueAt.Format(time.RFC3339), event.StartAt.Format(time.RFC3339))
	}
	if !event.EndAt.Equal(dueAt.Add(5 * time.Minute)) {
		t.Fatalf("expected due-only event end %s, got %s", dueAt.Add(5*time.Minute).Format(time.RFC3339), event.EndAt.Format(time.RFC3339))
	}
	if event.Description != "first description\n\nSource todo UID: todo-1" {
		t.Fatalf("unexpected initial description: %q", event.Description)
	}
	if !strings.Contains(event.Raw, "BEGIN:VALARM") || !strings.Contains(event.Raw, "TRIGGER:-PT10M") {
		t.Fatalf("expected projected event to contain VALARM, raw=%q", event.Raw)
	}

	putCalendarObject(t, ctx, baseURL, todoPath, buildTodoICS(todoFixture{
		UID:         "todo-1",
		Summary:     "Pay bill updated",
		Description: "updated description",
		StartAt:     &startAt,
		DueAt:       &dueAt,
	}))

	runReconcile(t, ctx, reconciler)

	events = listProjectedEvents(t, ctx, targetClient, eventCollection)
	if len(events) != 1 {
		t.Fatalf("expected 1 projected event after update, got %d", len(events))
	}

	event = events[0]
	if event.Summary != "Pay bill updated" {
		t.Fatalf("expected updated summary, got %q", event.Summary)
	}
	if !event.StartAt.Equal(startAt) {
		t.Fatalf("expected updated event start %s, got %s", startAt.Format(time.RFC3339), event.StartAt.Format(time.RFC3339))
	}
	if !event.EndAt.Equal(dueAt) {
		t.Fatalf("expected updated event end %s, got %s", dueAt.Format(time.RFC3339), event.EndAt.Format(time.RFC3339))
	}
	if event.Description != "updated description\n\nSource todo UID: todo-1" {
		t.Fatalf("unexpected updated description: %q", event.Description)
	}

	putCalendarObject(t, ctx, baseURL, todoPath, buildTodoICS(todoFixture{
		UID:         "todo-1",
		Summary:     "Pay bill updated",
		Description: "updated description",
		StartAt:     &startAt,
		DueAt:       &dueAt,
		Status:      "COMPLETED",
	}))

	runReconcile(t, ctx, reconciler)

	events = listProjectedEvents(t, ctx, targetClient, eventCollection)
	if len(events) != 0 {
		t.Fatalf("expected projected event to be deleted after todo completion, got %d event(s)", len(events))
	}
}

func runReconcile(t *testing.T, ctx context.Context, reconciler *syncer.Reconciler) {
	t.Helper()

	if err := reconciler.Run(ctx); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
}

func listProjectedEvents(t *testing.T, ctx context.Context, client *caldav.Client, eventCollection string) []model.Event {
	t.Helper()

	events, err := client.ListProjectedEvents(ctx, eventCollection)
	if err != nil {
		t.Fatalf("list projected events failed: %v", err)
	}
	return events
}

func startRadicale(t *testing.T, ctx context.Context) string {
	t.Helper()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "radicale.conf")
	usersPath := filepath.Join(tempDir, "users")

	configBody := `[server]
hosts = 0.0.0.0:5232

[auth]
type = htpasswd
htpasswd_filename = /etc/radicale/users
htpasswd_encryption = plain

[rights]
type = owner_only

[storage]
filesystem_folder = /var/lib/radicale/collections
`
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write radicale config: %v", err)
	}
	if err := os.WriteFile(usersPath, []byte(testUsername+":"+testPassword+"\n"), 0o644); err != nil {
		t.Fatalf("write radicale users: %v", err)
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "ghcr.io/kozea/radicale:stable",
			ExposedPorts: []string{"5232/tcp"},
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      configPath,
					ContainerFilePath: "/etc/radicale/config",
					FileMode:          0o644,
				},
				{
					HostFilePath:      usersPath,
					ContainerFilePath: "/etc/radicale/users",
					FileMode:          0o644,
				},
			},
			WaitingFor: wait.ForMappedPort("5232/tcp").WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start radicale container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get radicale host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5232/tcp")
	if err != nil {
		t.Fatalf("get radicale port: %v", err)
	}

	return fmt.Sprintf("http://%s:%s", host, port.Port())
}

func warmUpAuthenticatedRoot(t *testing.T, ctx context.Context, baseURL string) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, joinURL(baseURL, "/"), nil)
		if err != nil {
			t.Fatalf("build warm-up request: %v", err)
		}
		req.SetBasicAuth(testUsername, testPassword)

		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return
			}
			lastErr = fmt.Errorf("unexpected status: %s", resp.Status)
		} else {
			lastErr = err
		}

		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("warm up authenticated root: %v", lastErr)
}

func createCalendarCollection(t *testing.T, ctx context.Context, baseURL, collectionPath, displayName, component string) {
	t.Helper()

	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set>
    <D:prop>
      <D:displayname>%s</D:displayname>
      <C:supported-calendar-component-set>
        <C:comp name="%s" />
      </C:supported-calendar-component-set>
    </D:prop>
  </D:set>
</C:mkcalendar>`, displayName, component)

	req, err := http.NewRequestWithContext(ctx, "MKCALENDAR", joinURL(baseURL, collectionPath), strings.NewReader(body))
	if err != nil {
		t.Fatalf("build mkcalendar request: %v", err)
	}
	req.SetBasicAuth(testUsername, testPassword)
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create calendar collection %s: %v", collectionPath, err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("create calendar collection %s unexpected status: %s body=%s", collectionPath, resp.Status, string(responseBody))
	}
}

func putCalendarObject(t *testing.T, ctx context.Context, baseURL, objectPath, raw string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, joinURL(baseURL, objectPath), strings.NewReader(raw))
	if err != nil {
		t.Fatalf("build put request: %v", err)
	}
	req.SetBasicAuth(testUsername, testPassword)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put calendar object %s: %v", objectPath, err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("put calendar object %s unexpected status: %s body=%s", objectPath, resp.Status, string(responseBody))
	}
}

func joinURL(baseURL, resourcePath string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(resourcePath, "/")
}

type todoFixture struct {
	UID         string
	Summary     string
	Description string
	Status      string
	StartAt     *time.Time
	DueAt       *time.Time
}

func buildTodoICS(todo todoFixture) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//vtodo2vevent-e2e//EN\r\n")
	b.WriteString("BEGIN:VTODO\r\n")
	b.WriteString("UID:" + todo.UID + "\r\n")
	b.WriteString("DTSTAMP:20260408T000000Z\r\n")
	b.WriteString("SUMMARY:" + escapeICSValue(todo.Summary) + "\r\n")
	b.WriteString("DESCRIPTION:" + escapeICSValue(todo.Description) + "\r\n")
	if todo.Status != "" {
		b.WriteString("STATUS:" + todo.Status + "\r\n")
	}
	if todo.StartAt != nil {
		b.WriteString("DTSTART:" + todo.StartAt.UTC().Format("20060102T150405Z") + "\r\n")
	}
	if todo.DueAt != nil {
		b.WriteString("DUE:" + todo.DueAt.UTC().Format("20060102T150405Z") + "\r\n")
	}
	b.WriteString("END:VTODO\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

func escapeICSValue(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		";", `\\;`,
		",", `\\,`,
		"\n", `\\n`,
	)
	return replacer.Replace(value)
}
