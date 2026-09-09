package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

type openAIImageBackfillResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type openAIImageBackfillNetwork struct {
	resolver openAIImageBackfillResolver
	dial     func(context.Context, string, string) (net.Conn, error)
}

var openAIImageNonPublicPrefixes = mustParseOpenAIImagePrefixes([]string{
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
	"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
	"192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
	"224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64",
	"2001::/23", "2001:db8::/32", "2002::/16", "fc00::/7", "fe80::/10", "ff00::/8",
})

func mustParseOpenAIImagePrefixes(raw []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(raw))
	for _, value := range raw {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}

func isOpenAIImagePublicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range openAIImageNonPublicPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

func (s *OpenAIGatewayService) validateOpenAIImageBackfillURL(raw string) (string, error) {
	if _, err := s.validateOutboundURL(raw); err != nil {
		return "", errors.New("image URL is not allowed by outbound policy")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("image URL is invalid")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", errors.New("image URL scheme is not allowed")
	}
	if urlvalidator.IsBlockedHost(parsed.Hostname()) || isBlockedHostname(parsed.Hostname()) {
		return "", errors.New("image URL host is not allowed")
	}
	return raw, nil
}

func newOpenAIImageBackfillHTTPClient(network openAIImageBackfillNetwork, validate func(string) (string, error)) *http.Client {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           network.publicDialContext,
		ForceAttemptHTTP2:     true,
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("image redirect limit exceeded")
			}
			if _, err := validate(req.URL.String()); err != nil {
				return err
			}
			stripOpenAIImageSensitiveHeaders(req.Header)
			return nil
		},
	}
}

func (n openAIImageBackfillNetwork) publicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("image destination address is invalid")
	}
	var addrs []net.IPAddr
	if ip := net.ParseIP(host); ip != nil {
		addrs = []net.IPAddr{{IP: ip}}
	} else {
		if isBlockedHostname(host) {
			return nil, errors.New("image destination host is not allowed")
		}
		addrs, err = n.resolver.LookupIPAddr(ctx, host)
		if err != nil || len(addrs) == 0 {
			return nil, errors.New("image destination DNS resolution failed")
		}
	}
	for _, addr := range addrs {
		if !isOpenAIImagePublicIP(addr.IP) {
			return nil, errors.New("image destination resolved to a non-public address")
		}
	}
	var lastErr error
	for _, addr := range addrs {
		literal := net.JoinHostPort(addr.IP.String(), port)
		conn, dialErr := n.dial(ctx, network, literal)
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, errors.New("image destination connection failed")
	}
	return nil, errors.New("image destination has no usable public address")
}

func stripOpenAIImageSensitiveHeaders(header http.Header) {
	for _, name := range []string{"Authorization", "Cookie", "Referer"} {
		header.Del(name)
	}
}
