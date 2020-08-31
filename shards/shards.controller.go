package shards

type chainController struct {
	cfg *Config
}

func Controller(cfg *Config) *chainController {
	res := &chainController{}
	return res
}

func (c *chainController) runBeacon() error {
	return nil
}

func (c *chainController) runShard() error {
	return nil
}

func (c *chainController) Run() error {
	return nil
}
