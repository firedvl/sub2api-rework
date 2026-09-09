package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type imageDestinationResolver struct {
	mu      sync.Mutex
	answers map[string][]net.IPAddr
	calls   []string
}

func (r *imageDestinationResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, host)
	return r.answers[host], nil
}

func imageDestinationNetwork(t *testing.T, answers map[string][]net.IPAddr, respond func(*http.Request) *http.Response) (openAIImageBackfillNetwork, *imageDestinationResolver, *[]string) {
	t.Helper()
	resolver := &imageDestinationResolver{answers: answers}
	addresses := []string{}
	network := openAIImageBackfillNetwork{resolver: resolver}
	network.dial = func(_ context.Context, _, address string) (net.Conn, error) {
		addresses = append(addresses, address)
		client, server := net.Pipe()
		go func() {
			defer func() { _ = server.Close() }()
			request, err := http.ReadRequest(bufio.NewReader(server))
			if err != nil {
				return
			}
			defer func() { _ = request.Body.Close() }()
			response := respond(request)
			response.Close = true
			_ = response.Write(server)
		}()
		return client, nil
	}
	return network, resolver, &addresses
}

func imageDestinationResponse(code int, data []byte) *http.Response {
	return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data))}
}

func TestImageBackfillDestinationPublicAndRedirects(t *testing.T) {
	for _, redirect := range []string{"", "http://second.example/image", "http://10.0.0.1/private", "http://private.example/private", "http://user:password@second.example/image"} {
		t.Run(redirect, func(t *testing.T) {
			var requests []*http.Request
			network, _, addresses := imageDestinationNetwork(t, map[string][]net.IPAddr{
				"public.example":  {{IP: net.ParseIP("8.8.8.8")}},
				"second.example":  {{IP: net.ParseIP("1.1.1.1")}},
				"private.example": {{IP: net.ParseIP("192.168.1.1")}},
			}, func(request *http.Request) *http.Response {
				requests = append(requests, request)
				if len(requests) == 1 && redirect != "" {
					response := imageDestinationResponse(http.StatusFound, nil)
					response.Header.Set("Location", redirect)
					response.Header.Set("Set-Cookie", "private=value")
					return response
				}
				return imageDestinationResponse(http.StatusOK, b64BackfillPNGBytes)
			})
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			service := &OpenAIGatewayService{cfg: cfg}
			got, err := service.fetchOpenAIImageURLBase64WithNetwork(context.Background(), b64BackfillAccount(true), "http://public.example/signed%2Fpath/?sig=synthetic", network)
			if redirect == "" || redirect == "http://second.example/image" {
				require.NoError(t, err)
				require.Equal(t, base64.StdEncoding.EncodeToString(b64BackfillPNGBytes), got)
			} else {
				require.Error(t, err)
				require.Empty(t, got)
				require.NotContains(t, err.Error(), "sig=")
			}
			require.Equal(t, "8.8.8.8:80", (*addresses)[0])
			require.Equal(t, "public.example", requests[0].Host)
			require.Equal(t, "/signed%2Fpath/?sig=synthetic", requests[0].RequestURI)
			if redirect == "http://second.example/image" {
				require.Equal(t, []string{"8.8.8.8:80", "1.1.1.1:80"}, *addresses)
			} else {
				require.Len(t, *addresses, 1)
			}
			for _, request := range requests {
				for _, header := range []string{"Authorization", "Cookie", "Referer"} {
					require.Empty(t, request.Header.Get(header))
				}
			}
		})
	}
}

func TestImageBackfillDestinationRejectsNonPublicDNSAndLiterals(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "224.0.0.1", "::1", "fc00::1", "fe80::1", "ff02::1", "::ffff:127.0.0.1", "64:ff9b::a00:1"} {
		t.Run(address, func(t *testing.T) {
			for _, host := range []string{address, "public.example"} {
				resolver := &imageDestinationResolver{answers: map[string][]net.IPAddr{"public.example": {{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP(address)}}}}
				dials := 0
				network := openAIImageBackfillNetwork{resolver: resolver, dial: func(context.Context, string, string) (net.Conn, error) {
					dials++
					return nil, errors.New("unexpected dial")
				}}
				conn, err := network.publicDialContext(context.Background(), "tcp", net.JoinHostPort(host, "80"))
				require.Error(t, err)
				require.Nil(t, conn)
				require.Zero(t, dials)
			}
		})
	}
}

func TestImageBackfillDestinationProxySkipsBeforeDNS(t *testing.T) {
	proxyID := int64(1)
	for _, scheme := range []string{"http", "https", "socks5", "socks5h", "unknown", "missing", "orphan"} {
		t.Run(scheme, func(t *testing.T) {
			account := b64BackfillAccount(true)
			account.ProxyID = &proxyID
			account.Proxy = &Proxy{Protocol: scheme, Host: "proxy.example", Port: 1080}
			if scheme == "missing" {
				account.Proxy = nil
			}
			if scheme == "orphan" {
				account.ProxyID = nil
			}
			network, resolver, addresses := imageDestinationNetwork(t, nil, func(*http.Request) *http.Response {
				t.Error("proxy or direct request reached network")
				return imageDestinationResponse(500, nil)
			})
			service := &OpenAIGatewayService{cfg: &config.Config{}}
			got, err := service.fetchOpenAIImageURLBase64WithNetwork(context.Background(), account, "https://public.example/image", network)
			require.Error(t, err)
			require.Empty(t, got)
			require.Empty(t, resolver.calls)
			require.Empty(t, *addresses)
			dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(b64BackfillPNGBytes)
			got, err = service.fetchOpenAIImageURLBase64WithNetwork(context.Background(), account, dataURL, network)
			require.NoError(t, err)
			require.Equal(t, strings.TrimPrefix(dataURL, "data:image/png;base64,"), got)
			require.Empty(t, resolver.calls)
		})
	}
}

func TestImageBackfillDestinationTLSPreservesSNIAndVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_, _ = w.Write(b64BackfillPNGBytes)
	}))
	defer server.Close()
	cert := server.Certificate()
	require.NotEmpty(t, cert.DNSNames)
	host := cert.DNSNames[0]
	for _, trusted := range []bool{true, false} {
		t.Run(map[bool]string{true: "trusted", false: "untrusted"}[trusted], func(t *testing.T) {
			sni := make(chan string, 1)
			requests := make(chan string, 1)
			network := openAIImageBackfillNetwork{
				resolver: &imageDestinationResolver{answers: map[string][]net.IPAddr{host: {{IP: net.ParseIP("8.8.8.8")}}}},
				dial: func(_ context.Context, _, address string) (net.Conn, error) {
					require.Equal(t, "8.8.8.8:443", address)
					client, peer := net.Pipe()
					go func() {
						defer func() { _ = peer.Close() }()
						conn := tls.Server(peer, &tls.Config{Certificates: server.TLS.Certificates, GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) { sni <- hello.ServerName; return nil, nil }})
						if err := conn.Handshake(); err != nil {
							return
						}
						request, err := http.ReadRequest(bufio.NewReader(conn))
						if err != nil {
							return
						}
						defer func() { _ = request.Body.Close() }()
						requests <- request.Host
						response := imageDestinationResponse(http.StatusOK, b64BackfillPNGBytes)
						response.Close = true
						_ = response.Write(conn)
					}()
					return client, nil
				},
			}
			service := &OpenAIGatewayService{cfg: &config.Config{}}
			client := newOpenAIImageBackfillHTTPClient(network, service.validateOpenAIImageBackfillURL)
			transport, ok := client.Transport.(*http.Transport)
			require.True(t, ok)
			require.Nil(t, transport.DialTLSContext)
			require.Nil(t, transport.TLSClientConfig)
			if trusted {
				roots := x509.NewCertPool()
				roots.AddCert(cert)
				transport.TLSClientConfig = &tls.Config{RootCAs: roots}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/image", nil)
			require.NoError(t, err)
			response, err := client.Do(request)
			if trusted {
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())
				require.Equal(t, host, <-requests)
			} else {
				require.Error(t, err)
				require.Nil(t, response)
			}
			require.Equal(t, host, <-sni)
			client.CloseIdleConnections()
		})
	}
}

func TestImageBackfillDestinationExactSizeLimit(t *testing.T) {
	for _, size := range []int{openAIImageMaxDownloadBytes, openAIImageMaxDownloadBytes + 1} {
		data := make([]byte, size)
		copy(data, b64BackfillPNGBytes)
		encoded := base64.StdEncoding.EncodeToString(data)
		got, err := validateOpenAIImageDataURL("data:image/png;base64," + encoded)
		if size == openAIImageMaxDownloadBytes {
			require.NoError(t, err)
			require.Equal(t, encoded, got)
		} else {
			require.Error(t, err)
			require.Empty(t, got)
		}
		network, _, _ := imageDestinationNetwork(t, map[string][]net.IPAddr{"public.example": {{IP: net.ParseIP("8.8.8.8")}}}, func(*http.Request) *http.Response {
			return imageDestinationResponse(http.StatusOK, data)
		})
		cfg := &config.Config{}
		cfg.Security.URLAllowlist.AllowInsecureHTTP = true
		got, err = (&OpenAIGatewayService{cfg: cfg}).fetchOpenAIImageURLBase64WithNetwork(context.Background(), b64BackfillAccount(true), "http://public.example/image", network)
		if size == openAIImageMaxDownloadBytes {
			require.NoError(t, err)
			require.Equal(t, encoded, got)
		} else {
			require.Error(t, err)
			require.Empty(t, got)
		}
	}
}
