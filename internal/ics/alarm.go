package ics

import "fmt"

func BuildDisplayAlarm(minutesBefore int) string {
	return fmt.Sprintf("BEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nTRIGGER:-PT%dM\r\nEND:VALARM\r\n", minutesBefore)
}
