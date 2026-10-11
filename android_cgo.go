// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

//go:build android && cgo && !ts_omit_androidbin

package tailcat

import (
	"errors"
	"net"
	"net/netip"
	"time"

	"tailscale.com/feature"
	"tailscale.com/net/netmon"
)

// Tailscale's androidbin fallback is not registered in android+cgo builds,
// which includes Go Mobile libraries. Android app UIDs cannot enumerate
// interfaces through net.Interfaces, so install the same last-resort
// synthetic-interface fallback for Tailcat's embedded use case.
func init() {
	if feature.IsRegistered("androidbin") {
		return
	}
	if !feature.Register("androidbin") {
		return
	}
	netmon.HookInterfacesFallback.Set(androidCgoInterfaces)
}

func androidCgoInterfaces() ([]netmon.Interface, error) {
	var addrs []net.Addr
	if ip, ok := androidCgoOutboundIP("udp4", "8.8.8.8:53"); ok {
		addrs = append(addrs, &net.IPNet{IP: ip.AsSlice(), Mask: net.CIDRMask(32, 32)})
	}
	if ip, ok := androidCgoOutboundIP("udp6", "[2001:4860:4860::8888]:53"); ok {
		addrs = append(addrs, &net.IPNet{IP: ip.AsSlice(), Mask: net.CIDRMask(128, 128)})
	}
	if len(addrs) == 0 {
		return nil, errors.New("tailcat: no outbound Android routes found")
	}
	return []netmon.Interface{{
		Interface: &net.Interface{
			MTU:   1500,
			Name:  "android",
			Flags: net.FlagUp | net.FlagRunning,
		},
		AltAddrs: addrs,
	}}, nil
}

func androidCgoOutboundIP(network, addr string) (netip.Addr, bool) {
	c, err := (&net.Dialer{Timeout: 2 * time.Second}).Dial(network, addr)
	if err != nil {
		return netip.Addr{}, false
	}
	defer c.Close()
	ua, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, false
	}
	ip, ok := netip.AddrFromSlice(ua.IP)
	if !ok {
		return netip.Addr{}, false
	}
	ip = ip.Unmap()
	if ip.IsLoopback() || ip.IsUnspecified() {
		return netip.Addr{}, false
	}
	return ip, true
}
