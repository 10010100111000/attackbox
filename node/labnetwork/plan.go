package labnetwork

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
)

var identityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

func shortHash(value string, size int) string {
	s := sha256.Sum256([]byte(value))
	return hex.EncodeToString(s[:])[:size]
}

func networkID(s NetworkSpec) string { return s.TargetID + "/" + s.Key }

func validateSpec(s NetworkSpec) error {
	if !identityPattern.MatchString(s.TargetID) || !identityPattern.MatchString(s.OwnerID) || !keyPattern.MatchString(s.Key) {
		return fmt.Errorf("invalid network identity")
	}
	if s.PrefixBits < 24 || s.PrefixBits > 28 {
		return fmt.Errorf("lab subnet must be /24 through /28")
	}
	return nil
}

// Plan allocates the first available aligned subnet. occupied must include
// existing Docker networks, non-default host routes and durable reservations.
// /28 is the default, larger labs may request /27 through /24 before creation.
func Plan(pool netip.Prefix, spec NetworkSpec, occupied []netip.Prefix) (Network, error) {
	if spec.PrefixBits == 0 {
		spec.PrefixBits = 28
	}
	if err := validateSpec(spec); err != nil {
		return Network{}, err
	}
	if !pool.IsValid() || !pool.Addr().Is4() || pool != pool.Masked() || pool.Bits() < 16 || pool.Bits() > spec.PrefixBits {
		return Network{}, fmt.Errorf("pool must be a canonical IPv4 /16 or smaller containing requested subnet")
	}
	for _, p := range occupied {
		if !p.IsValid() {
			return Network{}, fmt.Errorf("invalid occupied prefix")
		}
	}
	step := uint32(1) << (32 - spec.PrefixBits)
	for a := pool.Addr(); pool.Contains(a); a = addIPv4(a, step) {
		p := netip.PrefixFrom(a, spec.PrefixBits)
		conflict := false
		for _, used := range occupied {
			if p.Overlaps(used) {
				conflict = true
				break
			}
		}
		if conflict {
			continue
		}
		id := networkID(spec)
		return Network{Spec: spec, ID: id, Name: "attackbox-lab-" + shortHash(id, 24), Bridge: "bt_" + shortHash(id, 8), Subnet: p, Gateway: a.Next(), SandboxIP: addIPv4(a, step-2), Desired: true}, nil
	}
	return Network{}, fmt.Errorf("lab address pool exhausted or overlaps existing networks")
}

func addIPv4(a netip.Addr, n uint32) netip.Addr {
	b := a.As4()
	v := uint64(uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3])) + uint64(n)
	if v > 0xffffffff {
		return netip.IPv4Unspecified()
	}
	return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
}

func makeAttachment(node string, n Network, s Sandbox, slot int) Attachment {
	id := shortHash(node+"/"+n.ID+"/"+s.ID+"/"+strconv.FormatUint(s.Namespace.Device, 10)+"/"+strconv.FormatUint(s.Namespace.Inode, 10), 24)
	return Attachment{ID: id, NetworkID: n.ID, Sandbox: s, HostName: "vlh_" + id[:10], GuestName: "lab" + strconv.Itoa(slot), PeerName: "vlg_" + id[:10], Alias: "attackbox:lab:" + node + ":" + id, Address: netip.PrefixFrom(n.SandboxIP, n.Subnet.Bits()), Desired: true}
}
