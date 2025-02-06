package runtime

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/nyaruka/gocommon/aws/cwatch"
)

type Stats struct {
	HandlerTaskCount    int           // number of contact tasks handled
	HandlerTaskDuration time.Duration // total time spent handling contact tasks
	HandlerTaskLatency  time.Duration // total time spent queuing and handling contact tasks

	CronTaskCount    map[string]int           // number of cron tasks run by type
	CronTaskDuration map[string]time.Duration // total time spent running cron tasks
}

func newStats() *Stats {
	return &Stats{
		CronTaskCount:    make(map[string]int),
		CronTaskDuration: make(map[string]time.Duration),
	}
}

func (s *Stats) ToMetrics() []types.MetricDatum {
	metrics := make([]types.MetricDatum, 0, 20)

	// convert handler task timings to averages
	avgHandlerTaskDuration, avgHandlerTaskLatency := time.Duration(0), time.Duration(0)
	if s.HandlerTaskCount > 0 {
		avgHandlerTaskDuration = s.HandlerTaskDuration / time.Duration(s.HandlerTaskCount)
		avgHandlerTaskLatency = s.HandlerTaskLatency / time.Duration(s.HandlerTaskCount)
	}

	metrics = append(metrics,
		cwatch.Datum("HandlerTaskCount", float64(s.HandlerTaskCount), types.StandardUnitCount),
		cwatch.Datum("HandlerTaskDuration", float64(avgHandlerTaskDuration)/float64(time.Second), types.StandardUnitCount),
		cwatch.Datum("HandlerTaskLatency", float64(avgHandlerTaskLatency)/float64(time.Second), types.StandardUnitCount),
	)

	for name, count := range s.CronTaskCount {
		avgTime := s.CronTaskDuration[name] / time.Duration(count)

		metrics = append(metrics,
			cwatch.Datum("CronTaskCount", float64(count), types.StandardUnitCount, cwatch.Dimension("TaskName", name)),
			cwatch.Datum("CronTaskDuration", float64(avgTime)/float64(time.Second), types.StandardUnitSeconds, cwatch.Dimension("TaskName", name)),
		)
	}

	return metrics
}

// StatsCollector provides threadsafe stats collection
type StatsCollector struct {
	mutex sync.Mutex
	stats *Stats
}

// NewStatsCollector creates a new stats collector
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{stats: newStats()}
}

func (c *StatsCollector) RecordHandlerTask(d, l time.Duration) {
	c.mutex.Lock()
	c.stats.HandlerTaskCount++
	c.stats.HandlerTaskDuration += d
	c.stats.HandlerTaskLatency += l
	c.mutex.Unlock()
}

func (c *StatsCollector) RecordCronTask(name string, d time.Duration) {
	c.mutex.Lock()
	c.stats.CronTaskCount[name]++
	c.stats.CronTaskDuration[name] += d
	c.mutex.Unlock()
}

// Extract returns the stats for the period since the last call
func (c *StatsCollector) Extract() *Stats {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	s := c.stats
	c.stats = newStats()
	return s
}
