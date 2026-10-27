//go:build linux

package labnetwork

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"
)

type creationLinks struct {
	link             netlink.Link
	addErr, aliasErr error
	sets, deletes    int
	ignoreAlias      bool
}

func (f *creationLinks) LinkByName(string) (netlink.Link, error) { return f.link, nil }
func (f *creationLinks) LinkAdd(netlink.Link) error              { return f.addErr }
func (f *creationLinks) LinkSetAlias(l netlink.Link, a string) error {
	f.sets++
	if f.aliasErr != nil {
		return f.aliasErr
	}
	if !f.ignoreAlias {
		l.Attrs().Alias = a
	}
	return nil
}
func (f *creationLinks) LinkDel(netlink.Link) error { f.deletes++; return nil }

func creationFixture() (*creationLinks, *creationLinks, *netlink.Veth, Attachment) {
	a := Attachment{HostName: "vlh_test", PeerName: "vlg_test", Alias: "attackbox:lab:node:owned"}
	h := &creationLinks{link: &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: a.HostName, Index: 20, ParentIndex: 30}}}
	p := &creationLinks{link: &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: a.PeerName, Index: 30, ParentIndex: 20}}}
	v := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: a.HostName, Alias: a.Alias}, PeerName: a.PeerName}
	return h, p, v, a
}

func TestNewVethExplicitAliasWithoutCreationTimeAlias(t *testing.T) {
	h, p, v, a := creationFixture()
	link, err := CreateOwnedVeth(h, p, v, a.Alias)
	if err != nil || link.Attrs().Alias != a.Alias || h.sets != 1 || h.deletes != 0 {
		t.Fatal(link, err, h)
	}
}

func TestNewVethAliasFailureRollsBackVerifiedPair(t *testing.T) {
	for _, ignored := range []bool{false, true} {
		h, p, v, a := creationFixture()
		h.ignoreAlias = ignored
		if !ignored {
			h.aliasErr = errors.New("SETLINK failed")
		}
		if _, err := CreateOwnedVeth(h, p, v, a.Alias); err == nil || h.deletes != 1 {
			t.Fatal("new pair not rolled back", err, h.deletes)
		}
	}
}

func TestNewVethNeverClaimsCollisionOrWrongPeer(t *testing.T) {
	for _, mutate := range []func(*creationLinks, *creationLinks){
		func(h, p *creationLinks) { h.addErr = errors.New("EEXIST") },
		func(h, p *creationLinks) { p.link.Attrs().ParentIndex = 99 },
		func(h, p *creationLinks) { h.link.Attrs().Alias = "foreign" },
	} {
		h, p, v, a := creationFixture()
		mutate(h, p)
		if _, err := CreateOwnedVeth(h, p, v, a.Alias); err == nil || h.sets != 0 || h.deletes != 0 {
			t.Fatal("unverified interface modified", err, h)
		}
	}
}
