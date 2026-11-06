package sub

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// subscriptionClientIP is a trust boundary: only a peer inside
// trustedProxyCIDRs may name the visitor via forwarded headers (lucx.344).
func TestSubscriptionClientIP(t *testing.T) {
	initSubDB(t)
	setTrustedProxyCIDRs(t, "127.0.0.1/32,::1/128,192.168.11.0/24")

	t.Run("untrusted peer cannot name its own IP", func(t *testing.T) {
		c := requestFrom(t, "203.0.113.9:51000", map[string]string{
			"X-Real-IP":       "6.6.6.6",
			"X-Forwarded-For": "6.6.6.6",
		})
		if got := subscriptionClientIP(c); got != "203.0.113.9" {
			t.Errorf("subscriptionClientIP = %q, want the socket peer 203.0.113.9", got)
		}
	})

	t.Run("nginx LAN proxy reports the visitor", func(t *testing.T) {
		c := requestFrom(t, "192.168.11.1:39000", map[string]string{
			"X-Real-IP":       "198.51.100.7",
			"X-Forwarded-For": "198.51.100.7",
		})
		if got := subscriptionClientIP(c); got != "198.51.100.7" {
			t.Errorf("subscriptionClientIP = %q, want 198.51.100.7", got)
		}
	})

	t.Run("chained XFF picks the address behind the trusted proxies", func(t *testing.T) {
		c := requestFrom(t, "192.168.11.1:39000", map[string]string{
			"X-Forwarded-For": "198.51.100.9, 203.0.113.9",
		})
		if got := subscriptionClientIP(c); got != "203.0.113.9" {
			t.Errorf("subscriptionClientIP = %q, want 203.0.113.9", got)
		}
	})

	t.Run("client-sent XFF entry before the proxy is dropped", func(t *testing.T) {
		c := requestFrom(t, "192.168.11.1:39000", map[string]string{
			"X-Forwarded-For": "6.6.6.6, 198.51.100.7",
		})
		if got := subscriptionClientIP(c); got != "198.51.100.7" {
			t.Errorf("subscriptionClientIP = %q, want the proxy-seen 198.51.100.7", got)
		}
	})

	t.Run("mismatched headers fall back to the proxy", func(t *testing.T) {
		c := requestFrom(t, "192.168.11.1:39000", map[string]string{
			"X-Real-IP":       "1.2.3.4",
			"X-Forwarded-For": "5.5.5.5",
		})
		if got := subscriptionClientIP(c); got != "192.168.11.1" {
			t.Errorf("subscriptionClientIP = %q, want the proxy socket 192.168.11.1", got)
		}
	})

	t.Run("non-IP header values are ignored", func(t *testing.T) {
		c := requestFrom(t, "192.168.11.1:39000", map[string]string{
			"X-Real-IP":       "not-an-ip",
			"X-Forwarded-For": "also, garbage",
		})
		if got := subscriptionClientIP(c); got != "192.168.11.1" {
			t.Errorf("subscriptionClientIP = %q, want the proxy socket 192.168.11.1", got)
		}
	})

	t.Run("trusted proxy without headers keeps its socket address", func(t *testing.T) {
		c := requestFrom(t, "192.168.11.1:39000", nil)
		if got := subscriptionClientIP(c); got != "192.168.11.1" {
			t.Errorf("subscriptionClientIP = %q, want 192.168.11.1", got)
		}
	})
}

// Empty / shipped-default CIDRs: only loopback proxies are trusted.
func TestSubscriptionClientIPDefaultCIDRs(t *testing.T) {
	initSubDB(t)
	setTrustedProxyCIDRs(t, service.DefaultTrustedProxyCIDRs)

	c := requestFrom(t, "127.0.0.1:39000", map[string]string{
		"X-Real-IP": "198.51.100.7",
	})
	if got := subscriptionClientIP(c); got != "198.51.100.7" {
		t.Errorf("subscriptionClientIP = %q, want 198.51.100.7", got)
	}

	c = requestFrom(t, "203.0.113.9:51000", map[string]string{
		"X-Real-IP": "6.6.6.6",
	})
	if got := subscriptionClientIP(c); got != "203.0.113.9" {
		t.Errorf("subscriptionClientIP = %q, want the socket peer 203.0.113.9", got)
	}
}
