package node

import (
	"fmt"
	"net/netip"
	"regexp"
)

var sandboxIDForNetwork = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func validateSandboxNetwork(cfg NetworkConfig) error {
	if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,14}$`).MatchString(cfg.BridgeName) {
		return fmt.Errorf("invalid sandbox bridge name")
	}
	if cfg.Interface != "" && !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,14}$`).MatchString(cfg.Interface) {
		return fmt.Errorf("invalid sandbox egress interface")
	}
	subnet, err := netip.ParsePrefix(cfg.SubnetCIDR)
	if err != nil || !subnet.Addr().Is4() || subnet != subnet.Masked() || subnet.Bits() > 29 {
		return fmt.Errorf("invalid sandbox subnet")
	}
	gateway, err := netip.ParseAddr(cfg.GatewayIP)
	if err != nil || !subnet.Contains(gateway) || gateway == subnet.Addr() || !subnet.Contains(gateway.Next()) {
		return fmt.Errorf("invalid sandbox gateway")
	}
	return nil
}
