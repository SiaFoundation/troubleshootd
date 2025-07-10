package troubleshoot

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"go.sia.tech/core/types"
	"go.sia.tech/coreutils/chain"
	rhp4 "go.sia.tech/coreutils/rhp/v4"
	"go.sia.tech/coreutils/rhp/v4/quic"
	"go.sia.tech/coreutils/rhp/v4/siamux"
	"go.sia.tech/troubleshootd/internal/dns"
	"golang.org/x/exp/constraints"
)

const minContractDuration = 144 * 30 // 30 days

func delta[T constraints.Integer | constraints.Float](a, b T) T {
	if a < b {
		return b - a
	}
	return a - b
}

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

func dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{
		Timeout: 2 * time.Minute,
	}).DialContext(ctx, network, address)
	if err != nil {
		// return more user-friendly errors if possible
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			return nil, fmt.Errorf("failed to resolve host %q: check DNS setup", address)
		}

		var opErr *net.OpError
		if errors.As(err, &opErr) {
			if syscallErr, ok := opErr.Err.(*os.SyscallError); ok {
				if syscallErr.Err == syscall.ECONNREFUSED {
					return nil, fmt.Errorf("connection refused at %q: check if the service is running and port is forwarded", address)
				}
			}
		}

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, fmt.Errorf("timeout connecting to %q: check port forwarding or firewall", address)
		}

		return nil, fmt.Errorf("failed to connect to host at %q: %w", address, err)
	}
	return conn, nil
}

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

	if delta(settings.Prices.TipHeight, tip.Height) >= 3 {
		res.Errors = append(res.Errors, fmt.Sprintf("host's tip height %d is less than the current tip height %d", settings.Prices.TipHeight, tip.Height))
	}

	release, err := parseReleaseString(settings.Release)
	if err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("host is running an unknown version %q, which may not be stable", settings.Release))
	} else if release.Cmp(currentVersion) < 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("host is running an outdated version %q, latest is %q", release, currentVersion))
	}
}

func testRHP4SiaMux(ctx context.Context, currentVersion SemVer, tip types.ChainIndex, hostKey types.PublicKey, addr chain.NetAddress, res *RHP4Result) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()
	conn, err := dialContext(ctx, "tcp", addr.Address)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
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
}

func testRHP4Quic(ctx context.Context, currentVersion SemVer, tip types.ChainIndex, hostKey types.PublicKey, addr chain.NetAddress, res *RHP4Result) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()
	t, err := quic.Dial(ctx, addr.Address, hostKey)
	if err != nil {
		if strings.Contains(err.Error(), "no recent network activity") {
			_, port, _ := net.SplitHostPort(addr.Address)
			res.Errors = append(res.Errors, fmt.Sprintf("failed to connect to quic: check port forwarding and firewall settings for UDP port %q", port))
		} else {
			res.Errors = append(res.Errors, fmt.Sprintf("failed to connect to quic: %s", err))
		}
		return
	}
	defer t.Close()
	// dialing UDP is kind of annoying, so we don't have a singular dial time
	// for QUIC. we just assume it's instant.
	res.HandshakeTime = time.Since(start)
	res.Connected = true
	res.Handshake = true

	testRHP4Transport(ctx, t, currentVersion, tip, res)
}

func lookupIPs(ctx context.Context, addr string) ([]net.IP, error) {
	// try system resolver first
	ips, err := net.LookupIP(addr)
	if err == nil {
		return ips, nil
	}

	// fallback to DNS resolver
	ips, err = dns.LookupIP(ctx, "1.1.1.1:53", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host %q: %w", addr, err)
	}
	return ips, nil
}

func testRHP4(ctx context.Context, currentVersion SemVer, tip types.ChainIndex, hostKey types.PublicKey, netAddr chain.NetAddress, res *RHP4Result) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	res.NetAddress = netAddr
	addr, _, err := net.SplitHostPort(netAddr.Address)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to parse net address %q: %v", netAddr.Address, err))
		return
	}

	ips, err := lookupIPs(ctx, addr)
	if err != nil {
		if errors.Is(err, dns.ErrNotFound) {
			res.Errors = append(res.Errors, fmt.Sprintf("DNS lookup %q failed: check DNS records or wait for propagation", addr))
		} else {
			res.Errors = append(res.Errors, fmt.Sprintf("failed to resolve host %q: %s", addr, err))
		}
		return
	}
	for _, ip := range ips {
		res.ResolvedAddresses = append(res.ResolvedAddresses, ip.String())
	}

	switch netAddr.Protocol {
	case siamux.Protocol:
		testRHP4SiaMux(ctx, currentVersion, tip, hostKey, netAddr, res)
	case quic.Protocol:
		testRHP4Quic(ctx, currentVersion, tip, hostKey, netAddr, res)
	default:
		res.Errors = append(res.Errors, fmt.Sprintf("unknown protocol %q", netAddr.Protocol))
	}
}
