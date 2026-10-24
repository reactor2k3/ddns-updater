package update

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/reactor2k3/ddns-updater/pkg/publicip/ipversion"
)

// Discovery is serialized by run, but does not inspect or update DNS records.
func (s *Service) discoverPublicIPs(ctx context.Context) []error {
	s.statsMu.Lock()
	s.stats.IPDiscoveryRunning = true
	s.statsMu.Unlock()
	discoveryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	type result struct {
		version ipversion.IPVersion
		ip      netip.Addr
		err     error
	}
	results := make(chan result, 2)
	for _, version := range []ipversion.IPVersion{ipversion.IP4, ipversion.IP6} {
		go func(version ipversion.IPVersion) {
			getter := s.ipGetter.IP4
			if version == ipversion.IP6 {
				getter = s.ipGetter.IP6
			}
			ip, err := tryAndRepeatGettingIP(discoveryCtx, getter, s.logger, version)
			results <- result{version: version, ip: ip, err: err}
		}(version)
	}
	ip4, ip6 := netip.Addr{}, netip.Addr{}
	var errs []error
	for range 2 {
		select {
		case result := <-results:
			if result.err != nil {
				errs = append(errs, errors.New("public IP discovery unavailable"))
				continue
			}
			if result.version == ipversion.IP4 && result.ip.Is4() {
				ip4 = result.ip
			} else if result.version == ipversion.IP6 && result.ip.Is6() {
				ip6 = result.ip
			} else {
				errs = append(errs, errors.New("public IP discovery returned wrong address family"))
			}
		case <-discoveryCtx.Done():
			errs = append(errs, discoveryCtx.Err())
		}
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.IPv4 = ""
	s.stats.IPv6 = ""
	s.stats.IPv4Status = "unavailable"
	s.stats.IPv6Status = "unavailable"
	if ip4.IsValid() {
		s.stats.IPv4 = ip4.String()
		s.stats.IPv4Status = "available"
	}
	if ip6.IsValid() {
		s.stats.IPv6 = ip6.String()
		s.stats.IPv6Status = "available"
	}
	s.stats.IPCheckedAt = s.timeNow()
	s.stats.IPDiscoveryRunning = false
	return errs
}

// Cached discovery avoids duplicate lookups when an update follows startup or a
// manual IP refresh. Missing unused address families do not fail DNS updates.
func (s *Service) currentIPs(ctx context.Context, doIP, do4, do6 bool) (ip, ip4, ip6 netip.Addr, errs []error) {
	state := s.Status()
	maxAge := min(s.period, 30*time.Second)
	if state.IPCheckedAt.IsZero() || s.timeNow().Sub(state.IPCheckedAt) >= maxAge {
		_ = s.discoverPublicIPs(ctx)
		state = s.Status()
	}
	ip4, _ = netip.ParseAddr(state.IPv4)
	ip6, _ = netip.ParseAddr(state.IPv6)
	if do4 && !ip4.IsValid() {
		errs = append(errs, errors.New("public IPv4 address not available"))
	}
	if do6 && !ip6.IsValid() {
		errs = append(errs, errors.New("public IPv6 address not available"))
	}
	if doIP {
		// Automatic records use the generic fetcher, which honors
		// PUBLICIP_HTTP_PROVIDERS instead of the family-specific lists.
		s.statsMu.RLock()
		ip, checkedAt := s.automaticIP, s.automaticIPCheckedAt
		s.statsMu.RUnlock()
		if checkedAt.IsZero() || s.timeNow().Sub(checkedAt) >= maxAge {
			automaticCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			value, err := tryAndRepeatGettingIP(automaticCtx, s.ipGetter.IP, s.logger, ipversion.IP4or6)
			cancel()
			if err != nil {
				value = netip.Addr{}
			}
			ip = value
			s.statsMu.Lock()
			s.automaticIP, s.automaticIPCheckedAt = value, s.timeNow()
			s.statsMu.Unlock()
		}
		if !ip.IsValid() {
			errs = append(errs, errors.New("public IP address not available"))
		}
		return ip, ip4, ip6, errs
	}
	return ip, ip4, ip6, errs
}

func (s *Service) RefreshPublicIPs(ctx context.Context) []error {
	result := make(chan []error, 1)
	select {
	case s.requests <- serviceRequest{publicIPsOnly: true, result: result}:
	case <-ctx.Done():
		return []error{ctx.Err()}
	}
	select {
	case errs := <-result:
		return errs
	case <-ctx.Done():
		return []error{ctx.Err()}
	}
}
