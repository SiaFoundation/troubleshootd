package rhp

import (
	"encoding/json"
	"fmt"

	rhp2 "go.sia.tech/core/rhp/v2"
)

const (
	// minMessageSize is the minimum size of an RPC message
	minMessageSize = 4096
)

// RPCSettings calls the Settings RPC, returning the host's reported settings.
func RPCSettings(t *rhp2.Transport) (settings rhp2.HostSettings, err error) {
	var resp rhp2.RPCSettingsResponse
	if err := t.Call(rhp2.RPCSettingsID, nil, &resp); err != nil {
		return rhp2.HostSettings{}, err
	} else if err := json.Unmarshal(resp.Settings, &settings); err != nil {
		return rhp2.HostSettings{}, fmt.Errorf("couldn't unmarshal json: %w", err)
	}

	return settings, nil
}
