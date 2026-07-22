package time

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	Nanosecond  time.Duration = 1
	Microsecond               = 1000 * Nanosecond
	Millisecond               = 1000 * Microsecond
	Second                    = 1000 * Millisecond
	Minute                    = 60 * Second
	Hour                      = 60 * Minute
)

type Time struct {
	time time.Time
}

func Now() *Time {
	t := Time{}

	t.time = time.Now().UTC().Add(9 * time.Hour)
	return &t
}

func Parse(str string) *Time {
	layout := "2006-01-02 15:04:05"
	t, err := time.Parse(layout, str)
	if err == nil {
		return &Time{time: t}
	}

	return nil
}

func (c *Time) Clone() *Time {
	t := Time{}
	t.time = c.time
	return &t
}

func (c *Time) Timestamp() int64 {
	return c.time.Unix()
}

func (c *Time) String() string {
	return c.Datetime()
}

func (c *Time) Datetime() string {
	t := c.time
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

func (c *Time) StartDatetime() string {
	t := c.time
	return fmt.Sprintf("%04d-%02d-%02d 00:00:00", t.Year(), t.Month(), t.Day())
}

func (c *Time) EndDatetime() string {
	t := c.time
	return fmt.Sprintf("%04d-%02d-%02d 23:59:59", t.Year(), t.Month(), t.Day())
}

func (c *Time) Date() string {
	t := c.time
	return fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
}

func (c *Time) DateWithEmptyTime() string {
	t := c.time
	return fmt.Sprintf("%04d-%02d-%02d 00:00:00", t.Year(), t.Month(), t.Day())
}

func (c *Time) DateAsOnlyNumber() string {
	t := c.time
	return fmt.Sprintf("%04d%02d%02d", t.Year(), t.Month(), t.Day())
}

func (c *Time) ToTime() time.Time {
	return c.time
}

func (c *Time) Time() string {
	t := c.time
	return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
}

func (c *Time) MonthStartDatetime() string {
	t := c.time
	return fmt.Sprintf("%04d-%02d-01 00:00:00", t.Year(), t.Month())
}

func (c *Time) Humandate() string {
	target := c.time

	t := time.Now().UTC().Add(9 * time.Hour)
	diff := t.Sub(target)

	if math.Floor(diff.Hours()/24) > 0 {
		if math.Floor(diff.Hours()/24) > 30 {
			return strings.ReplaceAll(c.Date(), "-", ".")
		} else {
			return fmt.Sprintf("%v일전", math.Floor(diff.Hours()/24))
		}
	}

	if math.Floor(diff.Hours()/24) > 0 {
		return strings.ReplaceAll(c.Date(), "-", ".")
	}

	if math.Floor(diff.Hours()) > 0 {
		return fmt.Sprintf("%v시간전", math.Floor(diff.Hours()))
	}

	m := math.Floor(diff.Minutes())

	if m == 0 {
		return "방금전"
	} else {
		return fmt.Sprintf("%v분전", m)
	}
}

func (c *Time) Year() int {
	return c.time.Year()
}

func (c *Time) Month() int {
	return int(c.time.Month())
}

func (c *Time) Day() int {
	return c.time.Day()
}

func (c *Time) Hour() int {
	return c.time.Hour()
}

func (c *Time) Minute() int {
	return c.time.Minute()
}

func (c *Time) Second() int {
	return c.time.Second()
}

func (c *Time) Nanosecond() int {
	return c.time.Nanosecond()
}

func (c *Time) Add(duration time.Duration) *Time {
	t := Time{}
	t.time = c.time.Add(duration)

	return &t
}

func (c *Time) AddDate(years int, months int, days int) *Time {
	t := Time{}
	t.time = c.time.AddDate(years, months, days)

	return &t
}

func (c *Time) GetDuration() (string, string) {
	return fmt.Sprintf("%v 00:00:00", c.Date()), fmt.Sprintf("%v 23:59:59", c.Date())
}

func (c *Time) GetDurationArray() [2]string {
	return [2]string{fmt.Sprintf("%v 00:00:00", c.Date()), fmt.Sprintf("%v 23:59:59", c.Date())}
}

func (c *Time) GMTDate() string {
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d+0900", c.Year(), c.Month(), c.Day(), c.Hour(), c.Minute(), c.Second())
}

func (c *Time) After(value *Time) bool {
	return c.time.After(value.ToTime())
}

func (c *Time) Before(value *Time) bool {
	return c.time.Before(value.time)
}

func (c *Time) Firstday() string {
	return fmt.Sprintf("%04d-%02d-01", c.Year(), c.Month())
}

func (c *Time) Lastday() string {
	d := time.Date(c.Year(), time.Month(c.Month()+1), 1, 0, 0, 0, 0, time.UTC)
	lastday := d.AddDate(0, 0, -1)

	return fmt.Sprintf("%04d-%02d-%02d", lastday.Year(), lastday.Month(), lastday.Day())
}

func (c *Time) FirstdayDatetime() string {
	return c.Firstday() + " 00:00:00"
}

func (c *Time) LastdayDatetime() string {
	return c.Lastday() + " 23:59:59"
}

func Sleep(d time.Duration) {
	time.Sleep(d)
}

func After(d time.Duration) <-chan time.Time {
	return time.After(d)
}
