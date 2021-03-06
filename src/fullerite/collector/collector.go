package collector

import (
	"fullerite/metric"

	"github.com/Sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{"app": "fullerite", "pkg": "collector"})

// Collector defines the interface of a generic collector.
type Collector interface {
	Collect()
	Name() string
	Interval() int
	SetInterval(int)
	Channel() chan metric.Metric
}

// New creates a new Collector based on the requested collector name.
func New(name string) Collector {
	var collector Collector
	switch name {
	case "Test":
		collector = NewTest()
	case "CPU":
		collector = NewCPU()
	case "Diamond":
		collector = NewDiamond()
	default:
		log.Fatal("Cannot create collector", name)
		return nil
	}
	return collector
}
