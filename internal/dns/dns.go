package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
)

var ErrNotFound = errors.New("no such host")

func queryRecord(ctx context.Context, server string, hostname string, recordType uint16) ([]string, error) {
	client := &dns.Client{
		Net:     "udp",
		Timeout: 5 * time.Second,
	}
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(hostname), recordType)
	resp, _, err := client.ExchangeContext(ctx, m, server)
	if err != nil {
		return nil, err
	}
	var results []string
	for _, answer := range resp.Answer {
		switch record := answer.(type) {
		case *dns.A:
			results = append(results, record.A.String())
		case *dns.AAAA:
			results = append(results, record.AAAA.String())
		case *dns.CNAME:
			results = append(results, record.Target)
		default:
			return nil, fmt.Errorf("unsupported record type: %T", answer)
		}
	}
	return results, nil
}

// QueryCNAME queries the DNS server for CNAME records of the given hostname.
// This function uses miekg/dns library to perform the query to bypass system
// cache and directly query the DNS server specified by the `server` parameter.
func QueryCNAME(ctx context.Context, server string, hostname string) ([]string, error) {
	resp, err := queryRecord(ctx, server, hostname, dns.TypeCNAME)
	if err != nil {
		return nil, err
	} else if len(resp) == 0 {
		return nil, ErrNotFound
	}
	return resp, nil
}

// QueryA queries the DNS server for A records of the given hostname.
func QueryA(ctx context.Context, server string, hostname string) ([]string, error) {
	resp, err := queryRecord(ctx, server, hostname, dns.TypeA)
	if err != nil {
		return nil, err
	} else if len(resp) == 0 {
		return nil, ErrNotFound
	}
	return resp, nil
}

// QueryAAAA queries the DNS server for AAAA records of the given hostname.
func QueryAAAA(ctx context.Context, server string, hostname string) ([]string, error) {
	resp, err := queryRecord(ctx, server, hostname, dns.TypeAAAA)
	if err != nil {
		return nil, err
	} else if len(resp) == 0 {
		return nil, ErrNotFound
	}
	return resp, nil
}

func LookupIP(ctx context.Context, server, hostname string) ([]net.IP, error) {
	a, err := queryRecord(ctx, server, hostname, dns.TypeA)
	if err != nil {
		return nil, fmt.Errorf("failed to query A records: %w", err)
	}

	aaaa, err := queryRecord(ctx, server, hostname, dns.TypeAAAA)
	if err != nil {
		return nil, fmt.Errorf("failed to query AAAA records: %w", err)
	}

	records := make([]net.IP, 0, len(a)+len(aaaa))
	for _, r := range a {
		ip := net.ParseIP(r)
		if ip == nil {
			continue
		}
		records = append(records, ip)
	}
	for _, r := range aaaa {
		ip := net.ParseIP(r)
		if ip == nil {
			continue
		}
		records = append(records, ip)
	}
	if len(records) == 0 {
		return nil, ErrNotFound
	}
	return records, nil
}
