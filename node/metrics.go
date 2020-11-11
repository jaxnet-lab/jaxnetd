package node

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//metricsManager metrics manager
type metricsManager struct {
	metrics  []IMetric
	interval time.Duration
	closing  bool
}

//IMetric metric reader
type IMetric interface {
	Read()
}

//IMetricManager metric manager
type IMetricManager interface {
	Add(metrics ...IMetric)
	Listen(route string, port uint16) error
}

//Metrics creates metric instance
func Metrics(ctx context.Context, interval time.Duration) IMetricManager {
	res := &metricsManager{
		interval: interval,
	}

	go res.collector(ctx)
	return res
}

func (m *metricsManager) Add(metrics ...IMetric) {
	for _, metric := range metrics {
		m.metrics = append(m.metrics, metric)
	}
}

func (m *metricsManager) collector(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.interval):
			for _, v := range m.metrics {
				v.Read()
			}
		}
	}
}

func (m *metricsManager) Listen(route string, port uint16) error {
	http.Handle(route, promhttp.Handler())
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
