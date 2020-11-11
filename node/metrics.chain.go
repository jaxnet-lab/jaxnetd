package node

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type chainMetrics struct {
	sync.RWMutex
	metricsByName map[string]prometheus.Gauge
	chain         *blockchain.BlockChain
	logger        zerolog.Logger
	name          string
}

func ChainMetrics(chain *blockchain.BlockChain, name string, logger zerolog.Logger) (res IMetric) {
	res = &chainMetrics{
		chain:         chain,
		logger:        logger.With().Str("ctx", "metrics").Str("chain", name).Logger(),
		name:          name,
		metricsByName: make(map[string]prometheus.Gauge),
	}
	return res
}

func (s *chainMetrics) Read() {
	snapshot := s.chain.BestSnapshot()
	s.updateGauge(prometheus.BuildFQName("chain", s.name, "height"), float64(snapshot.Height))
	s.updateGauge(prometheus.BuildFQName("chain", s.name, "size"), float64(snapshot.BlockSize))
	s.updateGauge(prometheus.BuildFQName("chain", s.name, "transactions"), float64(snapshot.NumTxns))
	s.updateGauge(prometheus.BuildFQName("chain", s.name, "median_time"), float64(snapshot.MedianTime.Unix()))
	s.updateGauge(prometheus.BuildFQName("chain", s.name, "bits"), float64(snapshot.Bits))
	s.updateGauge(prometheus.BuildFQName("chain", s.name, "total_transactions"), float64(snapshot.TotalTxns))
	block, err := s.chain.BlockByHeight(snapshot.Height)
	if err != nil {
		s.logger.Error().Err(err).Msg("can't get block height")
		return
	}
	if !block.MsgBlock().ShardBlock {
		h := block.MsgBlock().Header.(*wire.BeaconHeader)
		s.updateGauge(prometheus.BuildFQName("chain", s.name, "shards"), float64(h.Shards()))
	}
}

func (s *chainMetrics) updateGauge(name string, value float64) {
	s.RLock()
	m, ok := s.metricsByName[name]
	if !ok {
		m = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        name,
			ConstLabels: map[string]string{"chain": s.name},
		})
		err := prometheus.Register(m)
		if err != nil {
			s.logger.Error().Err(err).Msg("can't register metric")
		}
	}
	m.Set(value)
	s.metricsByName[name] = m
}
