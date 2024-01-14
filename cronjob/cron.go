package cronjob

import (
	"fmt"
	"github.com/robfig/cron"
	"time"
)

var c *cron.Cron

func NewCron(timeZone string) *cron.Cron {
	if c == nil {
		if currentZone, err := time.LoadLocation(timeZone); err != nil {
			fmt.Printf("Failed to load %s timezone. Reason: %s", timeZone, err)
			c = cron.New()
		} else {
			c = cron.NewWithLocation(currentZone)
		}
	}
	return c
}
