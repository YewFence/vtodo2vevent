package caldav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"vtodo2vevent/internal/ics"
	"vtodo2vevent/internal/model"
)

type Client struct {
	baseURL         string
	username        string
	password        string
	defaultLocation *time.Location
	httpClient      *http.Client
}

func NewClient(baseURL, username, password string, defaultLocation *time.Location) *Client {
	if defaultLocation == nil {
		defaultLocation = time.UTC
	}
	return &Client{
		baseURL:         strings.TrimRight(baseURL, "/"),
		username:        username,
		password:        password,
		defaultLocation: defaultLocation,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) ListTodos(ctx context.Context, collectionPath string) ([]model.Todo, error) {
	resources, err := c.calendarQuery(ctx, collectionPath, todoQueryBody())
	if err != nil {
		return nil, err
	}

	todos := make([]model.Todo, 0, len(resources))
	for _, resource := range resources {
		if strings.TrimSpace(resource.CalendarData) == "" {
			continue
		}
		todo, ok, err := parseTodoResource(resource.Href, resource.ETag, resource.CalendarData, c.defaultLocation)
		if err != nil {
			return nil, err
		}
		if ok {
			todos = append(todos, todo)
		}
	}

	sort.Slice(todos, func(i, j int) bool {
		return todos[i].UID < todos[j].UID
	})
	return todos, nil
}

func (c *Client) ListProjectedEvents(ctx context.Context, collectionPath string) ([]model.Event, error) {
	resources, err := c.calendarQuery(ctx, collectionPath, eventQueryBody())
	if err != nil {
		return nil, err
	}

	events := make([]model.Event, 0, len(resources))
	for _, resource := range resources {
		if strings.TrimSpace(resource.CalendarData) == "" {
			continue
		}
		event, ok, err := parseEventResource(resource.Href, resource.ETag, resource.CalendarData, c.defaultLocation)
		if err != nil {
			return nil, err
		}
		if ok {
			events = append(events, event)
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].UID < events[j].UID
	})
	return events, nil
}

func (c *Client) PutEvent(ctx context.Context, collectionPath string, event model.Event, alarmMinutesBefore int) error {
	resourcePath := joinCollectionResource(collectionPath, event.UID+".ics")
	body := buildEventICS(event, alarmMinutesBefore)
	return c.putICS(ctx, resourcePath, body)
}

func (c *Client) DeleteEvent(ctx context.Context, eventPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.absURL(eventPath), nil)
	if err != nil {
		return fmt.Errorf("build delete request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete event unexpected status: %s", resp.Status)
	}
	return nil
}

func (c *Client) putICS(ctx context.Context, resourcePath string, body string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.absURL(resourcePath), bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("build put request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("put event: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("put event unexpected status: %s", resp.Status)
	}
	return nil
}

func (c *Client) absURL(resourcePath string) string {
	if strings.HasPrefix(resourcePath, "http://") || strings.HasPrefix(resourcePath, "https://") {
		return resourcePath
	}
	if !strings.HasPrefix(resourcePath, "/") {
		resourcePath = "/" + resourcePath
	}
	return c.baseURL + resourcePath
}

func joinCollectionResource(collectionPath, filename string) string {
	clean := strings.TrimSuffix(collectionPath, "/")
	return path.Clean(clean + "/" + filename)
}

func (c *Client) calendarQuery(ctx context.Context, collectionPath string, body string) ([]calendarResource, error) {
	req, err := http.NewRequestWithContext(ctx, "REPORT", c.absURL(collectionPath), bytes.NewBufferString(body))
	if err != nil {
		return nil, fmt.Errorf("build calendar-query request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)
	req.Header.Set("Depth", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calendar-query request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("calendar-query unexpected status: %s body=%s", resp.Status, string(bodyBytes))
	}

	var multi multistatusResponse
	if err := xml.NewDecoder(resp.Body).Decode(&multi); err != nil {
		return nil, fmt.Errorf("decode multistatus: %w", err)
	}
	return extractCalendarResources(multi), nil
}

func todoQueryBody() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
    <d:getetag />
    <c:calendar-data />
  </d:prop>
  <c:filter>
    <c:comp-filter name="VCALENDAR">
      <c:comp-filter name="VTODO" />
    </c:comp-filter>
  </c:filter>
</c:calendar-query>`
}

func eventQueryBody() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
    <d:getetag />
    <c:calendar-data />
  </d:prop>
  <c:filter>
    <c:comp-filter name="VCALENDAR">
      <c:comp-filter name="VEVENT" />
    </c:comp-filter>
  </c:filter>
</c:calendar-query>`
}

func extractCalendarResources(multi multistatusResponse) []calendarResource {
	resources := make([]calendarResource, 0)
	for _, response := range multi.Responses {
		for _, propstat := range response.PropStats {
			if !strings.Contains(propstat.Status, " 200 ") {
				continue
			}
			resources = append(resources, calendarResource{
				Href:         response.Href,
				ETag:         strings.TrimSpace(propstat.Prop.GetETag),
				CalendarData: propstat.Prop.CalendarData,
			})
		}
	}
	return resources
}

func parseTodoResource(pathValue, etag, raw string, defaultLocation *time.Location) (model.Todo, bool, error) {
	props := parseICSProperties(raw)
	uid := firstPropValue(props, "UID")
	if uid == "" {
		return model.Todo{}, false, nil
	}

	status := strings.ToUpper(firstPropValue(props, "STATUS"))
	completed := status == "COMPLETED" || firstPropValue(props, "COMPLETED") != ""
	startProp, _ := firstProp(props, "DTSTART")
	startAt, startHasTime, err := parseICSDateTime(startProp, defaultLocation)
	if err != nil {
		return model.Todo{}, false, fmt.Errorf("parse todo DTSTART for %s: %w", uid, err)
	}
	dueProp, _ := firstProp(props, "DUE")
	dueAt, dueHasTime, err := parseICSDateTime(dueProp, defaultLocation)
	if err != nil {
		return model.Todo{}, false, fmt.Errorf("parse todo DUE for %s: %w", uid, err)
	}

	return model.Todo{
		Path:         pathValue,
		ETag:         etag,
		UID:          uid,
		Summary:      firstPropValue(props, "SUMMARY"),
		Description:  firstPropValue(props, "DESCRIPTION"),
		Status:       status,
		Completed:    completed,
		StartAt:      startAt,
		StartHasTime: startHasTime,
		DueAt:        dueAt,
		DueHasTime:   dueHasTime,
		Raw:          raw,
	}, true, nil
}

func parseEventResource(pathValue, etag, raw string, defaultLocation *time.Location) (model.Event, bool, error) {
	props := parseICSProperties(raw)
	uid := firstPropValue(props, "UID")
	if uid == "" {
		return model.Event{}, false, nil
	}

	sourceTodoUID := firstPropValue(props, "X-SOURCE-TODO-UID")
	if sourceTodoUID == "" {
		return model.Event{}, false, nil
	}

	startProp, _ := firstProp(props, "DTSTART")
	startAt, _, err := parseICSDateTime(startProp, defaultLocation)
	if err != nil {
		return model.Event{}, false, fmt.Errorf("parse event DTSTART for %s: %w", uid, err)
	}
	endProp, _ := firstProp(props, "DTEND")
	endAt, _, err := parseICSDateTime(endProp, defaultLocation)
	if err != nil {
		return model.Event{}, false, fmt.Errorf("parse event DTEND for %s: %w", uid, err)
	}
	if startAt == nil || endAt == nil {
		return model.Event{}, false, nil
	}

	return model.Event{
		Path:          pathValue,
		ETag:          etag,
		UID:           uid,
		Summary:       firstPropValue(props, "SUMMARY"),
		Description:   firstPropValue(props, "DESCRIPTION"),
		StartAt:       *startAt,
		EndAt:         *endAt,
		SourceTodoUID: sourceTodoUID,
		Raw:           raw,
	}, true, nil
}

func parseICSProperties(raw string) map[string][]icsProperty {
	lines := unfoldICSLines(raw)
	props := make(map[string][]icsProperty)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		namePart := line[:idx]
		value := strings.TrimSpace(line[idx+1:])
		name, params := parseICSPropertyName(namePart)
		props[name] = append(props[name], icsProperty{
			Name:   name,
			Params: params,
			Value:  unescapeText(value),
		})
	}
	return props
}

func unfoldICSLines(raw string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	parts := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(lines) > 0 && (strings.HasPrefix(part, " ") || strings.HasPrefix(part, "\t")) {
			lines[len(lines)-1] += strings.TrimLeft(part, " \t")
			continue
		}
		lines = append(lines, part)
	}
	return lines
}

func firstProp(props map[string][]icsProperty, name string) (icsProperty, bool) {
	values := props[strings.ToUpper(name)]
	if len(values) == 0 {
		return icsProperty{}, false
	}
	return values[0], true
}

func firstPropValue(props map[string][]icsProperty, name string) string {
	prop, ok := firstProp(props, name)
	if !ok {
		return ""
	}
	return prop.Value
}

func parseICSPropertyName(value string) (string, map[string]string) {
	parts := strings.Split(value, ";")
	name := strings.ToUpper(strings.TrimSpace(parts[0]))
	params := make(map[string]string, max(len(parts)-1, 0))
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, rawValue, found := strings.Cut(part, "=")
		if !found {
			params[strings.ToUpper(part)] = ""
			continue
		}
		params[strings.ToUpper(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(rawValue), `"`)
	}
	return name, params
}

func parseICSDateTime(prop icsProperty, defaultLocation *time.Location) (*time.Time, bool, error) {
	value := strings.TrimSpace(prop.Value)
	if value == "" {
		return nil, false, nil
	}

	if defaultLocation == nil {
		defaultLocation = time.UTC
	}

	valueType := strings.ToUpper(strings.TrimSpace(prop.Params["VALUE"]))
	hasTime := strings.Contains(value, "T") && valueType != "DATE"
	if !hasTime {
		parsed, err := time.ParseInLocation("20060102", value, defaultLocation)
		if err != nil {
			return nil, false, fmt.Errorf("unsupported datetime format: %s", value)
		}
		return &parsed, false, nil
	}

	if strings.HasSuffix(value, "Z") {
		parsed, err := time.Parse("20060102T150405Z", value)
		if err != nil {
			return nil, true, fmt.Errorf("unsupported datetime format: %s", value)
		}
		return &parsed, true, nil
	}

	location := defaultLocation
	if tzid := strings.TrimSpace(prop.Params["TZID"]); tzid != "" {
		var err error
		location, err = time.LoadLocation(tzid)
		if err != nil {
			return nil, true, fmt.Errorf("load tzid %q: %w", tzid, err)
		}
	}

	parsed, err := time.ParseInLocation("20060102T150405", value, location)
	if err != nil {
		return nil, true, fmt.Errorf("unsupported datetime format: %s", value)
	}
	return &parsed, true, nil
}

func unescapeText(v string) string {
	replacer := strings.NewReplacer(
		`\\n`, "\n",
		`\\N`, "\n",
		`\\,`, ",",
		`\\;`, ";",
		`\\\\`, `\\`,
	)
	return replacer.Replace(v)
}

type multistatusResponse struct {
	XMLName   xml.Name           `xml:"multistatus"`
	Responses []multistatusEntry `xml:"response"`
}

type multistatusEntry struct {
	Href      string                `xml:"href"`
	PropStats []multistatusPropStat `xml:"propstat"`
}

type multistatusPropStat struct {
	Status string          `xml:"status"`
	Prop   multistatusProp `xml:"prop"`
}

type multistatusProp struct {
	GetETag      string `xml:"getetag"`
	CalendarData string `xml:"calendar-data"`
}

type calendarResource struct {
	Href         string
	ETag         string
	CalendarData string
}

type icsProperty struct {
	Name   string
	Params map[string]string
	Value  string
}

func buildEventICS(event model.Event, alarmMinutesBefore int) string {
	start := event.StartAt.UTC().Format("20060102T150405Z")
	end := event.EndAt.UTC().Format("20060102T150405Z")
	dtstamp := time.Now().UTC().Format("20060102T150405Z")
	summary := escapeText(event.Summary)
	description := escapeText(event.Description)
	xSource := escapeText(event.SourceTodoUID)

	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//vtodo2vevent//EN\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")
	b.WriteString("UID:" + event.UID + "\r\n")
	b.WriteString("DTSTAMP:" + dtstamp + "\r\n")
	b.WriteString("DTSTART:" + start + "\r\n")
	b.WriteString("DTEND:" + end + "\r\n")
	b.WriteString("SUMMARY:" + summary + "\r\n")
	b.WriteString("DESCRIPTION:" + description + "\r\n")
	b.WriteString("X-SOURCE-TODO-UID:" + xSource + "\r\n")
	if alarmMinutesBefore > 0 {
		b.WriteString(ics.BuildDisplayAlarm(alarmMinutesBefore))
	}
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

func escapeText(v string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		";", `\\;`,
		",", `\\,`,
		"\n", `\\n`,
	)
	return replacer.Replace(v)
}
