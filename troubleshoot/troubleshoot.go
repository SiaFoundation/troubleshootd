package troubleshoot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.sia.tech/core/consensus"
	proto4 "go.sia.tech/core/rhp/v4"
	"go.sia.tech/core/types"
	"go.sia.tech/coreutils/chain"
	"go.sia.tech/coreutils/threadgroup"
	"go.sia.tech/troubleshootd/github"
	"go.uber.org/zap"
)

type (
	// A Host is a host on the Sia network. It contains the public key of the
	// host, the address of the host's RHP2 endpoint, and a list of addresses for
	// RHP4.
	Host struct {
		PublicKey        types.PublicKey    `json:"publicKey"`
		RHP4NetAddresses []chain.NetAddress `json:"rhp4NetAddresses"`
	}

	// RHP4Result is the result of testing a host's RHP4 endpoint. It contains
	// the results of the connection, handshake, and scan, as well as any errors
	// or warnings that occurred during the test.
	RHP4Result struct {
		NetAddress        chain.NetAddress `json:"netAddress"`
		ResolvedAddresses []string         `json:"resolvedAddresses"`

		Connected bool          `json:"connected"`
		DialTime  time.Duration `json:"dialTime"`

		Handshake     bool          `json:"handshake"`
		HandshakeTime time.Duration `json:"handshakeTime"`

		Scanned  bool          `json:"scanned"`
		ScanTime time.Duration `json:"scanTime"`

		Settings *proto4.HostSettings `json:"settings"`

		Errors   []string `json:"errors"`
		Warnings []string `json:"warnings"`
	}

	// A Result is the result of testing a host. It contains the public key of the
	// host, the version of the host, and the results of the RHP2, RHP3, and RHP4
	Result struct {
		PublicKey types.PublicKey `json:"publicKey"`
		Version   string          `json:"version"`

		RHP4 []RHP4Result `json:"rhp4"`
	}

	// An Explorer is an interface that defines the methods required to
	// query state from the Sia blockchain.
	Explorer interface {
		ConsensusState() (consensus.State, error)
	}

	// A Manager manages the testing of hosts.
	Manager struct {
		tg       *threadgroup.ThreadGroup
		log      *zap.Logger
		explorer Explorer

		mu            sync.Mutex // protects the fields below
		latestRelease SemVer
		state         consensus.State

		// cooldown protects hosts from being spammed too frequently
		cooldown map[types.PublicKey]time.Time
	}
)

// TestHost tests a host by connecting to its RHP2, RHP3, and RHP4 endpoints.
// It returns a Result struct containing the results of the tests.
func (m *Manager) TestHost(ctx context.Context, host Host) (Result, error) {
	ctx, cancel, err := m.tg.AddContext(ctx)
	if err != nil {
		return Result{}, err
	}
	defer cancel()

	m.mu.Lock()
	// check if the host is on cooldown
	if n := time.Until(m.cooldown[host.PublicKey]); n > 0 {
		m.mu.Unlock()
		return Result{}, fmt.Errorf("host is on cooldown, please try again in %s", n)
	}
	m.cooldown[host.PublicKey] = time.Now().Add(15 * time.Second)
	// grab the latest state
	latestRelease := m.latestRelease
	cs := m.state
	m.mu.Unlock()

	start := time.Now()
	log := m.log.With(zap.Stringer("host", host.PublicKey))
	log.Debug("starting host test")

	resp := Result{
		PublicKey: host.PublicKey,
	}
	var wg sync.WaitGroup

	resp.RHP4 = make([]RHP4Result, len(host.RHP4NetAddresses))
	rhp4Protos := make(map[chain.Protocol]bool)
	var rhp4VersionSet sync.Once
	var rhp4Version string
	for i, addr := range host.RHP4NetAddresses {
		if rhp4Protos[addr.Protocol] {
			// skip duplicate protocols
			resp.RHP4[i].Errors = append(resp.RHP4[i].Errors, fmt.Sprintf("duplicate protocol %q", addr.Protocol))
			continue
		}

		wg.Add(1)
		go func(i int, addr chain.NetAddress) {
			defer wg.Done()

			log := log.With(zap.String("addr", addr.Address), zap.String("protocol", string(addr.Protocol)))
			log.Debug("starting RHP4 test")
			start := time.Now()
			testRHP4(ctx, latestRelease, cs.Index, host.PublicKey, addr, &resp.RHP4[i])
			log.Debug("finished RHP4 test", zap.Bool("successful", resp.RHP4[i].Scanned), zap.Duration("elapsed", time.Since(start)))
			if resp.RHP4[i].Settings != nil {
				// sticky version check
				rhp4VersionSet.Do(func() {
					rhp4Version = resp.RHP4[i].Settings.Release
				})

				if resp.RHP4[i].Settings.Release != rhp4Version {
					resp.RHP4[i].Errors = append(resp.RHP4[i].Errors, fmt.Sprintf("host is reporting multiple versions %q and %q", rhp4Version, resp.RHP4[i].Settings.Release))
				}
			}
		}(i, addr)
	}
	wg.Wait()
	if len(resp.RHP4) != 0 {
		for _, r := range resp.RHP4 {
			if r.Settings != nil {
				resp.Version = r.Settings.Release
				break
			}
		}
	}
	log.Info("host tested", zap.String("version", resp.Version), zap.Duration("elapsed", time.Since(start)))
	return resp, nil
}

// Close stops the manager and releases any resources it holds.
func (m *Manager) Close() error {
	m.tg.Stop()
	return nil
}

// NewManager creates a new Manager instance. It fetches the latest release
// from GitHub and initializes the manager with the provided Explorer and logger.
func NewManager(explorer Explorer, log *zap.Logger) (*Manager, error) {
	latestRelease, err := github.LatestRelease("SiaFoundation", "hostd")
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}

	m := &Manager{
		tg:       threadgroup.New(),
		log:      log,
		explorer: explorer,

		cooldown: make(map[types.PublicKey]time.Time),
	}

	if err := m.latestRelease.UnmarshalText([]byte(latestRelease)); err != nil {
		return nil, fmt.Errorf("failed to unmarshal latest release: %w", err)
	}

	cs, err := explorer.ConsensusState()
	if err != nil {
		return nil, fmt.Errorf("failed to get tip state: %w", err)
	}
	m.state = cs

	ctx, cancel, err := m.tg.AddContext(context.Background())
	if err != nil {
		return nil, err
	}

	go func() {
		defer cancel()

		versionTicker := time.NewTicker(15 * time.Minute)
		defer versionTicker.Stop()

		// tip state changes more frequently than the
		// latest release, poll it every minute.
		stateTicker := time.NewTicker(time.Minute)
		defer stateTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-stateTicker.C:
				cs, err := explorer.ConsensusState()
				if err != nil {
					log.Warn("failed to update tip state", zap.Error(err))
					continue
				}
				m.mu.Lock()
				m.state = cs
				m.mu.Unlock()
			case <-versionTicker.C:
				releaseStr, err := github.LatestRelease("SiaFoundation", "hostd")
				if err != nil {
					log.Warn("failed to update latest release", zap.Error(err))
					continue
				}
				var release SemVer
				if err := release.UnmarshalText([]byte(releaseStr)); err != nil {
					log.Warn("failed to unmarshal latest release", zap.Error(err))
					continue
				}
				m.mu.Lock()
				m.latestRelease = release
				m.mu.Unlock()
			}
		}
	}()

	return m, nil
}
