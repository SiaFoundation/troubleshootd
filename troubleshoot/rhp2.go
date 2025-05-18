package troubleshoot

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	proto2 "go.sia.tech/core/rhp/v2"
	proto3 "go.sia.tech/core/rhp/v3"
	rhp2 "go.sia.tech/host-troubleshoot/internal/rhp/v2"
	rhp3 "go.sia.tech/host-troubleshoot/internal/rhp/v3"
)

const minContractDuration = 144 * 30 // 30 days

func parseReleaseString(versionStr string) (SemVer, error) {
	var version SemVer
	if parts := strings.Fields(versionStr); len(parts) > 1 {
		versionStr = parts[1] // remove the app prefix
	}
	if err := version.UnmarshalText([]byte(versionStr)); err != nil {
		return SemVer{}, err
	}
	return version, nil
}

func testRHP2(ctx context.Context, currentVersion SemVer, host Host, res *RHP2Result) {
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", host.RHP2NetAddress)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to connect to host: %v", err))
		return
	}
	defer conn.Close()
	res.DialTime = time.Since(start)
	res.Connected = true

	addr, _, err := net.SplitHostPort(host.RHP2NetAddress)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to parse net address %q: %v", host.RHP2NetAddress, err))
		return
	}

	ips, err := net.LookupIP(addr)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to resolve host %q: %v", addr, err))
		return
	}
	for _, ip := range ips {
		res.ResolvedAddresses = append(res.ResolvedAddresses, ip.String())
	}

	start = time.Now()
	t, err := proto2.NewRenterTransport(conn, host.PublicKey)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to create transport: %v", err))
		return
	}
	defer t.Close()
	res.HandshakeTime = time.Since(start)
	res.Handshake = true

	start = time.Now()
	settings, err := rhp2.RPCSettings(t)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to get settings: %v", err))
		return
	}
	res.ScanTime = time.Since(start)
	res.Scanned = true
	res.Settings = &settings

	// validate the settings
	if !settings.AcceptingContracts {
		res.Warnings = append(res.Warnings, "host is not accepting contracts")
	}

	if settings.NetAddress != host.RHP2NetAddress {
		res.Warnings = append(res.Warnings, "announced net address does not match settings")
	}

	if settings.MaxCollateral.IsZero() {
		res.Warnings = append(res.Warnings, "max collateral is zero")
	}

	if settings.Collateral.IsZero() {
		res.Warnings = append(res.Warnings, "collateral is zero")
	} else if settings.Collateral.Cmp(settings.StoragePrice) < 0 {
		res.Warnings = append(res.Warnings, "collateral should be greater than storage price")
	} else if settings.StoragePrice.Mul64(2).Cmp(settings.Collateral) > 0 {
		res.Warnings = append(res.Warnings, "collateral should be at least double the storage price")
	}

	if settings.MaxDuration < minContractDuration {
		res.Warnings = append(res.Warnings, "max duration is less than 30 days")
	}

	release, err := parseReleaseString(settings.Release)
	if err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("failed to parse release version: %v", err))
	} else if release.Cmp(currentVersion) > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("host is running an outdated version: %s", release))
	}
}

func testRHP3(ctx context.Context, rhp3Addr string, height uint64, host Host, res *RHP3Result) {
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", rhp3Addr)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to connect to host: %v", err))
		return
	}
	defer conn.Close()
	res.DialTime = time.Since(start)
	res.Connected = true

	start = time.Now()
	t, err := proto3.NewRenterTransport(conn, host.PublicKey)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to create transport: %v", err))
		return
	}
	defer t.Close()
	res.HandshakeTime = time.Since(start)
	res.Handshake = true

	start = time.Now()
	pt, err := rhp3.ScanPriceTable(ctx, t)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to scan price table: %v", err))
		return
	}
	res.ScanTime = time.Since(start)
	res.Scanned = true
	res.PriceTable = &pt

	// validate the price table
	if pt.MaxCollateral.IsZero() {
		res.Warnings = append(res.Warnings, "max collateral is zero")
	}
	if pt.CollateralCost.IsZero() {
		res.Warnings = append(res.Warnings, "collateral is zero")
	} else if pt.CollateralCost.Cmp(pt.WriteStoreCost) < 0 {
		res.Warnings = append(res.Warnings, "collateral should be greater than storage price")
	} else if pt.WriteStoreCost.Mul64(2).Cmp(pt.CollateralCost) > 0 {
		res.Warnings = append(res.Warnings, "collateral should be at least double the storage price")
	}

	if pt.HostBlockHeight < height {
		res.Warnings = append(res.Warnings, fmt.Sprintf("host is behind consensus by %d blocks", height-pt.HostBlockHeight))
	}
}
