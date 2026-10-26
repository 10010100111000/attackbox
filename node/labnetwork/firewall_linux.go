//go:build linux

package labnetwork

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// An early INPUT drop cannot be bypassed by an accept/established rule in the
// legacy Worker table. Docker internal networks own FORWARD isolation; we do
// not duplicate Docker's forwarding rules or filter same-bridge lab traffic.
func ProtectionRules(node string) (string, error) {
	if !identityPattern.MatchString(node) {
		return "", fmt.Errorf("invalid node identity")
	}
	table := "attackbox_labs_" + shortHash(node, 8)
	return fmt.Sprintf(`table inet %s {
    comment "attackbox:lab:%s"
    chain input {
        type filter hook input priority -10; policy accept;
        iifname "bt_*" drop
    }
}
`, table, node), nil
}

type protection interface{ Ensure(context.Context) error }
type nftProtection struct{ node string }

func nft(ctx context.Context, input string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "nft", args...)
	if input != "" {
		cmd.Stdin = bytes.NewBufferString(input)
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lab nftables command failed: %w", err)
	}
	return out, nil
}

// ownedTable returns false only after a successful inventory, never on EPERM.
func (p nftProtection) ownedTable(ctx context.Context) (bool, error) {
	out, err := nft(ctx, "", "-j", "list", "tables")
	if err != nil {
		return false, err
	}
	var data struct {
		NFTables []struct {
			Table *struct{ Family, Name string }
		} `json:"nftables"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return false, err
	}
	name := "attackbox_labs_" + shortHash(p.node, 8)
	found := false
	for _, x := range data.NFTables {
		if x.Table != nil && x.Table.Family == "inet" && x.Table.Name == name {
			found = true
		}
	}
	if !found {
		return false, nil
	}
	out, err = nft(ctx, "", "-j", "list", "table", "inet", name)
	if err != nil {
		return false, err
	}
	var table struct {
		NFTables []struct{ Table *struct{ Comment string } } `json:"nftables"`
	}
	if err := json.Unmarshal(out, &table); err != nil {
		return false, err
	}
	for _, x := range table.NFTables {
		if x.Table != nil && x.Table.Comment == "attackbox:lab:"+p.node {
			return true, nil
		}
	}
	return false, fmt.Errorf("refusing unowned nftables table")
}

func (p nftProtection) Ensure(ctx context.Context) error {
	rules, err := ProtectionRules(p.node)
	if err != nil {
		return err
	}
	exists, err := p.ownedTable(ctx)
	if err != nil {
		return err
	}
	if exists {
		rules = "delete table inet attackbox_labs_" + shortHash(p.node, 8) + "\n" + rules
	}
	_, err = nft(ctx, rules, "-f", "-")
	return err
}
