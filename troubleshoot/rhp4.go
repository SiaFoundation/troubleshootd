package troubleshoot

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.sia.tech/core/types"
	"go.sia.tech/coreutils/chain"
	rhp4 "go.sia.tech/coreutils/rhp/v4"
	"go.sia.tech/coreutils/rhp/v4/quic"
	"go.sia.tech/coreutils/rhp/v4/siamux"
)

func testRHP4Transport(ctx context.Context, t rhp4.TransportClient, currentVersion SemVer, tip types.ChainIndex, res *RHP4Result) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	start := time.Now()
	settings, err := rhp4.RPCSettings(ctx, t)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to get settings: %s", err))
	}
	res.ScanTime = time.Since(start)
	res.Scanned = true
	res.Settings = &settings

	if !settings.AcceptingContracts {
		res.Warnings = append(res.Warnings, "host is not accepting contracts")
	}

	if settings.MaxCollateral.IsZero() {
		res.Warnings = append(res.Warnings, "host has no max collateral")
	}

	if settings.MaxContractDuration < minContractDuration {
		res.Warnings = append(res.Warnings, "host has a max contract duration less than 1 month")
	}

	if settings.Prices.Collateral.IsZero() {
		res.Warnings = append(res.Warnings, "host has no collateral price")
	} else if settings.Prices.Collateral.Cmp(settings.Prices.StoragePrice) < 0 {
		res.Warnings = append(res.Warnings, "host's collateral price is less than storage price")
	} else if settings.Prices.StoragePrice.Mul64(2).Cmp(settings.Prices.Collateral) > 0 {
		res.Warnings = append(res.Warnings, "host's collateral price is less than double the storage price")
	}

	if settings.Prices.TipHeight < tip.Height {
		res.Warnings = append(res.Warnings, fmt.Sprintf("host's tip height %d is less than the current tip height %d", settings.Prices.TipHeight, tip.Height))
	}

	release, err := parseReleaseString(settings.Release)
	if err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("failed to parse release version: %v", err))
	} else if release.Cmp(currentVersion) > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("host is running an outdated version: %s", release))
	}
}

func testRHP4SiaMux(ctx context.Context, currentVersion SemVer, tip types.ChainIndex, hostKey types.PublicKey, addr chain.NetAddress, res *RHP4Result) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr.Address)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to connect to host: %s", err))
		return
	}
	defer conn.Close()
	res.DialTime = time.Since(start)
	res.Connected = true

	start = time.Now()
	t, err := siamux.Upgrade(ctx, conn, hostKey)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to connect to siamux: %s", err))
		return
	}
	defer t.Close()
	res.HandshakeTime = time.Since(start)
	res.Handshake = true

	testRHP4Transport(ctx, t, currentVersion, tip, res)
	return
}

func testRHP4Quic(ctx context.Context, currentVersion SemVer, tip types.ChainIndex, hostKey types.PublicKey, addr chain.NetAddress, res *RHP4Result) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()
	t, err := quic.Dial(ctx, addr.Address, hostKey)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to connect to quic: %s", err))
		return
	}
	defer t.Close()
	// dialing UDP is kind of annoying, so we don't have a singular dial time
	// for QUIC. we just assume it's instant.
	res.HandshakeTime = time.Since(start)
	res.Connected = true
	res.Handshake = true

	testRHP4Transport(ctx, t, currentVersion, tip, res)
	return
}
