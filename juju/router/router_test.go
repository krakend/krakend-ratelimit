package router

import (
	"encoding/json"
	"testing"

	"github.com/devopsfaith/krakend/config"
)

func TestConfigGetter(t *testing.T) {
	serializedCfg := []byte(`{
		"github.com/devopsfaith/krakend-ratelimit/juju/router": {
			"maxRate":10
		}
	}`)
	var dat config.ExtraConfig
	if err := json.Unmarshal(serializedCfg, &dat); err != nil {
		t.Error(err.Error())
	}
	cfg := ConfigGetter(dat).(Config)
	if cfg.MaxRate != 10 {
		t.Errorf("wrong value for MaxRate. Want: 10, have: %d", cfg.MaxRate)
	}
	if cfg.ClientMaxRate != 0 {
		t.Errorf("wrong value for ClientMaxRate. Want: 0, have: %d", cfg.ClientMaxRate)
	}
	if cfg.Strategy != "" {
		t.Errorf("wrong value for Strategy. Want: '', have: %s", cfg.Strategy)
	}
	if cfg.Key != "" {
		t.Errorf("wrong value for Key. Want: '', have: %s", cfg.Key)
	}
}
