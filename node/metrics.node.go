package node

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type nodeMetrics struct {
	sync.RWMutex
	cfg   *Config
	stats nodeStatsProvider
	name  string

	logger        zerolog.Logger
	metricsByName map[string]prometheus.Gauge
}
type nodeStatsProvider interface {
	Stats() map[string]float64
}

func NodeMetrics(cfg *Config, stats nodeStatsProvider, logger zerolog.Logger) (res IMetric) {
	res = &nodeMetrics{
		logger:        logger,
		cfg:           cfg,
		stats:         stats,
		metricsByName: make(map[string]prometheus.Gauge),
	}
	return res
}

func (s *nodeMetrics) Read() {
	for name, value := range s.stats.Stats() {
		s.updateGauge(prometheus.BuildFQName(metricsNamespace, "node", name), value)
	}

	dSize, err := dirSize(s.cfg.DataDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("can't calculate data dir size")
		return
	}

	s.updateGauge(prometheus.BuildFQName(metricsNamespace, "node", "data_size"), float64(dSize))
	logSize, err := dirSize(s.cfg.LogDir)
	if err != nil && !os.IsNotExist(err) {
		s.logger.Error().Err(err).Msg("can't calculate log dir size")
	}
	s.updateGauge(prometheus.BuildFQName(metricsNamespace, "node", "log_size"), float64(logSize))
}

func (s *nodeMetrics) updateGauge(name string, value float64) {
	s.RLock()
	m, ok := s.metricsByName[name]
	if !ok {
		m = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: name,
			// Help: "Beacon Height",
		})
		err := prometheus.Register(m)
		if err != nil {
			s.logger.Error().Err(err).Msg("can't register metric")
		}
	}
	m.Set(value)
	s.metricsByName[name] = m
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
