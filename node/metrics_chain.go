package node

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
)

type statsProvider interface {
	ChainCtx() chainctx.IChainCtx
	Stats() map[string]float64
}

type chainMetrics struct {
	sync.RWMutex
	metricsByName map[string]prometheus.Gauge
	logger        zerolog.Logger

	chainID   string
	chainName string
	netName   string

	chain statsProvider
}

func MetricsOfChain(chain statsProvider, logger zerolog.Logger) (res IMetric) {
	res = &chainMetrics{
		chain:         chain,
		logger:        logger.With().Str("ctx", "metrics").Str("chain", chain.ChainCtx().Name()).Logger(),
		chainID:       strconv.Itoa(int(chain.ChainCtx().ShardID())),
		chainName:     chain.ChainCtx().Name(),
		netName:       chain.ChainCtx().Params().Net.String(),
		metricsByName: make(map[string]prometheus.Gauge),
	}
	return res
}

func (s *chainMetrics) Read() {
	stats := s.chain.Stats()
	for name, value := range stats {
		s.updateGauge(prometheus.BuildFQName("jax_core", "chain", name), value)
	}
}

func (s *chainMetrics) updateGauge(name string, value float64) {
	s.RLock()
	m, ok := s.metricsByName[name]
	if !ok {
		m = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: name,
			ConstLabels: map[string]string{
				"chain_name": s.chainName,
				"chain_id":   s.chainID,
				"net_name":   s.netName,
			},
		})
		err := prometheus.Register(m)
		if err != nil {
			s.logger.Error().Err(err).Msg("can't register metric")
		}
	}
	m.Set(value)
	s.metricsByName[name] = m
}
