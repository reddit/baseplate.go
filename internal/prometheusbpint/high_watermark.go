package prometheusbpint

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// HighWatermarkValue implements an int64 gauge with high watermark value.
type HighWatermarkValue struct {
	lock sync.RWMutex
	curr int64
	max  int64
}

// Inc increases the gauge value by 1.
func (hwv *HighWatermarkValue) Inc() {
	hwv.lock.Lock()
	defer hwv.lock.Unlock()

	hwv.curr++
	if hwv.curr > hwv.max {
		hwv.max = hwv.curr
	}
}

// Dec decreases the gauge value by 1.
func (hwv *HighWatermarkValue) Dec() {
	hwv.lock.Lock()
	defer hwv.lock.Unlock()

	hwv.curr--
}

// Set sets the current value, and updates high watermark if needed.
func (hwv *HighWatermarkValue) Set(v int64) {
	hwv.lock.Lock()
	defer hwv.lock.Unlock()

	hwv.curr = v
	if hwv.curr > hwv.max {
		hwv.max = hwv.curr
	}
}

// Get gets the current gauge value.
func (hwv *HighWatermarkValue) Get() int64 {
	hwv.lock.RLock()
	defer hwv.lock.RUnlock()

	return hwv.curr
}

// Max returns the max gauge value (the high watermark).
func (hwv *HighWatermarkValue) Max() int64 {
	hwv.lock.RLock()
	defer hwv.lock.RUnlock()

	return hwv.max
}

func (hwv *HighWatermarkValue) getBoth() (curr, max int64) {
	hwv.lock.RLock()
	defer hwv.lock.RUnlock()

	return hwv.curr, hwv.max
}

// HighWatermarkGauge implements a prometheus.Collector that reports up to 2
// gauges backed by a HighWatermarkValue.
type HighWatermarkGauge struct {
	*HighWatermarkValue

	// Optional gauge to report the current value when scraped
	CurrGauge            *prometheus.Desc
	CurrGaugeLabelValues []string

	// Optional gauge to report the max value when scraped
	MaxGauge            *prometheus.Desc
	MaxGaugeLabelValues []string
}

// Describe implements prometheus.Collector.
func (hwg HighWatermarkGauge) Describe(ch chan<- *prometheus.Desc) {
	// All metrics are described dynamically.
}

// Collect implements prometheus.Collector.
func (hwg HighWatermarkGauge) Collect(ch chan<- prometheus.Metric) {
	curr, max := hwg.HighWatermarkValue.getBoth()

	if hwg.CurrGauge != nil {
		ch <- prometheus.MustNewConstMetric(
			hwg.CurrGauge,
			prometheus.GaugeValue,
			float64(curr),
			hwg.CurrGaugeLabelValues...,
		)
	}

	if hwg.MaxGauge != nil {
		ch <- prometheus.MustNewConstMetric(
			hwg.MaxGauge,
			prometheus.GaugeValue,
			float64(max),
			hwg.MaxGaugeLabelValues...,
		)
	}
}

var _ prometheus.Collector = HighWatermarkGauge{}
