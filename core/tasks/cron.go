package tasks

import (
	"context"
	"sync"
	"time"

	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/utils/crons"
)

// Cron is a task to be repeated on a schedule
type Cron interface {
	// Next returns the next schedule time
	Next(time.Time) time.Time

	// Run performs the task
	Run(context.Context, *runtime.Runtime) (map[string]any, error)

	// AllInstances returns whether cron runs on all instances - i.e. locking is instance specific. This is for crons
	// like analytics which report instance specific stats. Other crons are synchronized across all instances.
	AllInstances() bool
}

var registeredCrons = map[string]Cron{}

// RegisterCron registers a new cron job
func RegisterCron(name string, c Cron) {
	registeredCrons[name] = c
}

// StartCrons starts all registered cron jobs
func StartCrons(rt *runtime.Runtime, wg *sync.WaitGroup, quit chan bool) {
	for name, c := range registeredCrons {
		crons.Start(rt, wg, name, c.AllInstances(), c.Run, c.Next, time.Minute*5, quit)
	}
}

// CronNext returns the next time we should fire based on the passed in time and interval
func CronNext(last time.Time, interval time.Duration) time.Time {
	if interval >= time.Second && interval < time.Minute {
		return last.Add(interval - ((time.Duration(last.Second()) * time.Second) % interval))
	} else if interval == time.Minute {
		seconds := time.Duration(60-last.Second()) + 1
		return last.Add(seconds * time.Second)
	} else {
		// no special treatment for other things
		return last.Add(interval)
	}
}
