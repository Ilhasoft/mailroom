package tasks

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/mailroom/utils/crons"
)

const (
	cronStatsExpires       = 60 * 60 * 48 // 2 days
	cronStatsKeyBase       = "cron_stats"
	cronStatsLastStartKey  = cronStatsKeyBase + ":last_start"
	cronStatsLastTimeKey   = cronStatsKeyBase + ":last_time"
	cronStatsLastResultKey = cronStatsKeyBase + ":last_result"
	cronStatsCallCountKey  = cronStatsKeyBase + ":call_count"
	cronStatsTotalTimeKey  = cronStatsKeyBase + ":total_time"
)

var statsKeys = []string{
	cronStatsLastStartKey,
	cronStatsLastTimeKey,
	cronStatsLastResultKey,
	cronStatsCallCountKey,
	cronStatsTotalTimeKey,
}

// Cron is a task to be repeated on a schedule
type Cron interface {
	// Next returns the next schedule time
	Next(time.Time) time.Time

	// Run performs the task
	Run(context.Context, *runtime.Runtime) (map[string]any, error)

	// AllInstances returns whether cron runs on all instances - i.e. locking is instance specific. This is for crons
	// like metrics which report instance specific stats. Other crons are synchronized across all instances.
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
		crons.Start(rt, wg, name, c.AllInstances(), recordCronExecution(name, c.Run), c.Next, time.Minute*5, quit)
	}
}

func recordCronExecution(name string, r func(context.Context, *runtime.Runtime) (map[string]any, error)) func(context.Context, *runtime.Runtime) error {
	return func(ctx context.Context, rt *runtime.Runtime) error {
		log := slog.With("cron", name)
		started := time.Now()

		results, err := r(ctx, rt)

		elapsed := time.Since(started)
		elapsedSeconds := elapsed.Seconds()

		rt.Stats.RecordCronTask(name, elapsed)

		rc := rt.RP.Get()
		defer rc.Close()

		rc.Send("HSET", cronStatsLastStartKey, name, started.Format(time.RFC3339))
		rc.Send("HSET", cronStatsLastTimeKey, name, elapsedSeconds)
		rc.Send("HSET", cronStatsLastResultKey, name, jsonx.MustMarshal(results))
		rc.Send("HINCRBY", cronStatsCallCountKey, name, 1)
		rc.Send("HINCRBYFLOAT", cronStatsTotalTimeKey, name, elapsedSeconds)
		for _, key := range statsKeys {
			rc.Send("EXPIRE", key, cronStatsExpires)
		}

		if err := rc.Flush(); err != nil {
			log.Error("error writing cron results to redis")
		}

		logResults := make([]any, 0, len(results)*2)
		for k, v := range results {
			logResults = append(logResults, k, v)
		}
		log = log.With("elapsed", elapsed, slog.Group("results", logResults...))

		// if cron too longer than a minute, log as error
		if elapsed > time.Minute {
			log.Error("cron took too long")
		} else {
			log.Info("cron completed")
		}

		return err
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
