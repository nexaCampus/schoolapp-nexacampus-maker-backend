package security

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	ErrDisallowedHost   = errors.New("outbound target domain is not in the allowed cloud provider whitelist")
	ErrPrivateIPBlocked = errors.New("outbound requests to private, loopback, or metadata addresses are prohibited")
	ErrInvalidURL       = errors.New("target URL is malformed or not HTTPS")

	// WhitelistedCloudDomains lists verified third-party API hosts
	WhitelistedCloudDomains = []string{
		"api.render.com",
		"api.vercel.com",
		"amazonaws.com",
		"api.hostinger.com",
		"api.cloudflare.com",
	}
)

// ValidateOutboundURL ensures that outbound requests are HTTPS, target whitelisted cloud providers,
// and do not resolve to local, internal, or cloud metadata IP addresses.
func ValidateOutboundURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return nil, ErrInvalidURL
	}

	if strings.ToLower(parsed.Scheme) != "https" {
		return nil, fmt.Errorf("%w: outbound scheme must be https", ErrInvalidURL)
	}

	host := strings.ToLower(parsed.Hostname())

	// Verify whitelisted domain
	isAllowed := false
	for _, domain := range WhitelistedCloudDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		return nil, fmt.Errorf("%w: %s", ErrDisallowedHost, host)
	}

	// Resolve IP to check for private or metadata addresses
	ips, err := net.LookupIP(host)
	if err == nil {
		for _, ip := range ips {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || isCloudMetadata(ip) {
				return nil, ErrPrivateIPBlocked
			}
		}
	}

	return parsed, nil
}

func isCloudMetadata(ip net.IP) bool {
	// 169.254.169.254 AWS/GCP/Azure link-local metadata address
	metadataIP := net.ParseIP("169.254.169.254")
	return ip.Equal(metadataIP)
}
