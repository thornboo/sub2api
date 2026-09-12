package service

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

func TestSupplierBalanceDialHonorsPrivateHostPolicy(t *testing.T) {
	for _, tc := range []struct {
		name         string
		allowlist    bool
		allowPrivate bool
		wantBlocked  bool
	}{
		{"reject rebinding to private IP", true, false, true},
		{"explicitly allow private upstream", true, true, false},
		{"allowlist disabled", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			var authenticated atomic.Bool
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				authenticated.Store(r.Header.Get("Authorization") == "Bearer synthetic-balance-test-token")
				_, _ = w.Write([]byte(`{"balance":9,"unit":"USD"}`))
			}))
			t.Cleanup(server.Close)
			parsed, err := url.Parse(server.URL)
			require.NoError(t, err)
			host := server.Certificate().DNSNames[0]
			queries := supplierBalanceRebindingDNS(t, tc.wantBlocked)
			cfg := newSupplierBalanceQueryTestConfig()
			cfg.Security.URLAllowlist.Enabled = tc.allowlist
			cfg.Security.URLAllowlist.AllowPrivateHosts = tc.allowPrivate
			cfg.Security.URLAllowlist.UpstreamHosts = []string{host}
			endpoint, err := cnValidateProbeURL(cfg, "https://"+net.JoinHostPort(host, parsed.Port())+"/v1/usage")
			require.NoError(t, err)
			client := newUpstreamSupplierBalanceHTTPClient()
			// Trust the fixture certificate without disabling TLS hostname verification.
			transport, ok := client.Transport.(*http.Transport)
			require.True(t, ok)
			fixtureTransport, ok := server.Client().Transport.(*http.Transport)
			require.True(t, ok)
			transport.TLSClientConfig = fixtureTransport.TLSClientConfig.Clone()
			require.False(t, transport.TLSClientConfig.InsecureSkipVerify)
			t.Cleanup(client.CloseIdleConnections)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if shouldValidateNewAPISupplierResolvedIP(cfg) {
				ctx = WithHTTPUpstreamPublicHostsOnly(ctx)
			}
			headers := http.Header{"Authorization": []string{"Bearer synthetic-balance-test-token"}}
			_, err = requestSupplierBalanceJSON(ctx, client, endpoint, headers)
			if tc.wantBlocked {
				require.Error(t, err)
				require.Zero(t, requests.Load(), "a private server must not receive the request or credential")
				require.GreaterOrEqual(t, queries.Load(), int32(2), "exercise changed DNS answers between validation and dialing")
				return
			}
			require.NoError(t, err)
			require.Equal(t, int32(1), requests.Load())
			require.True(t, authenticated.Load())
		})
	}
}

// The public answer is a documentation-only address and is never dialed. The
// second lookup returns loopback. All DNS and HTTPS traffic stays in this fixture.
// Tests replacing net.DefaultResolver must remain non-parallel.
func supplierBalanceRebindingDNS(t *testing.T, publicFirst bool) *atomic.Int32 {
	t.Helper()
	queries := &atomic.Int32{}
	dns, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = dns.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := dns.ReadFrom(buf)
			if err != nil {
				return
			}
			var query dnsmessage.Message
			if query.Unpack(buf[:n]) != nil {
				continue
			}
			answer := dnsmessage.Message{
				Header: dnsmessage.Header{
					ID: query.ID, Response: true, Authoritative: true,
					RecursionDesired: true, RecursionAvailable: true,
				},
				Questions: query.Questions,
			}
			for _, question := range query.Questions {
				if question.Type != dnsmessage.TypeA {
					continue
				}
				ip := [4]byte{127, 0, 0, 1}
				if queries.Add(1) == 1 && publicFirst {
					ip = [4]byte{203, 0, 113, 1}
				}
				answer.Answers = append(answer.Answers, dnsmessage.Resource{
					Header: dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET},
					Body:   &dnsmessage.AResource{A: ip},
				})
			}
			data, err := answer.Pack()
			if err == nil {
				_, _ = dns.WriteTo(data, addr)
			}
		}
	}()
	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", dns.LocalAddr().String())
	}}
	t.Cleanup(func() { net.DefaultResolver = previous })
	return queries
}
