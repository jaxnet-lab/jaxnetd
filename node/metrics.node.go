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
	metricsByName map[string]prometheus.Gauge
	index         *Index
	logger        zerolog.Logger
	name          string
	cfg           *Config
}

func NodeMetrics(cfg *Config, index *Index, logger zerolog.Logger) (res IMetric) {
	res = &nodeMetrics{
		logger:        logger,
		cfg:           cfg,
		index:         index,
		metricsByName: make(map[string]prometheus.Gauge),
	}
	return res
}

func (s *nodeMetrics) Read() {
	s.updateGauge(prometheus.BuildFQName("node", "status", "shards"), float64(len(s.index.Shards)))
	dSize, err := dirSize(s.cfg.DataDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("can't calculate data dir size")
		return
	}

	s.updateGauge(prometheus.BuildFQName("node", "status", "data_size"), float64(dSize))
	logSize, err := dirSize(s.cfg.LogDir)
	if err != nil && !os.IsNotExist(err) {
		s.logger.Error().Err(err).Msg("can't calculate log dir size")
	}
	s.updateGauge(prometheus.BuildFQName("node", "status", "log_size"), float64(logSize))
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
