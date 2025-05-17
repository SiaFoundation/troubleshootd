package rhp

import (
	"context"
	"encoding/json"
	"fmt"

	proto3 "go.sia.tech/core/rhp/v3"
)

// ScanPriceTable retrieves the host's current price table
func ScanPriceTable(ctx context.Context, t *proto3.Transport) (proto3.HostPriceTable, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream := t.DialStream()
	defer stream.Close()

	go func() {
		// tie the stream's lifetime to the context
		<-ctx.Done()
		stream.Close()
	}()

	if deadline, ok := ctx.Deadline(); ok {
		stream.SetDeadline(deadline)
	}

	if err := stream.WriteRequest(proto3.RPCUpdatePriceTableID, nil); err != nil {
		return proto3.HostPriceTable{}, fmt.Errorf("failed to write request: %w", err)
	}
	var resp proto3.RPCUpdatePriceTableResponse
	if err := stream.ReadResponse(&resp, 4096); err != nil {
		return proto3.HostPriceTable{}, fmt.Errorf("failed to read response: %w", err)
	}

	var pt proto3.HostPriceTable
	if err := json.Unmarshal(resp.PriceTableJSON, &pt); err != nil {
		return proto3.HostPriceTable{}, fmt.Errorf("failed to unmarshal price table: %w", err)
	}
	return pt, nil
}
