package cmd

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestDecodeCustomError(t *testing.T) {
	// Selectors confirmed against the live testnet registry.
	for sel, want := range map[string]string{
		"fa0d8bfd": "InsufficientSomiPayment",
		"5dcaf2d7": "OrderDoesNotExist",
	} {
		ce, ok := customErrors[sel]
		if !ok {
			t.Fatalf("selector %s not registered", sel)
		}
		if ce.name != want {
			t.Errorf("selector %s = %q, want %q", sel, ce.name, want)
		}
	}

	// No-arg error renders name + hint.
	if msg, ok := decodeCustomError(common.FromHex("0x5dcaf2d7")); !ok ||
		msg != "OrderDoesNotExist: the stop order has already triggered or been cancelled" {
		t.Errorf("OrderDoesNotExist: got %q ok=%v", msg, ok)
	}

	// Parametered error decodes and appends its arguments.
	data := append(selectorOf(t, "InsufficientBalance(uint256,uint256)"),
		append(common.LeftPadBytes(big.NewInt(5).Bytes(), 32),
			common.LeftPadBytes(big.NewInt(10).Bytes(), 32)...)...)
	want := "InsufficientBalance(5, 10): insufficient balance (available, required)"
	if msg, ok := decodeCustomError(data); !ok || msg != want {
		t.Errorf("InsufficientBalance: got %q ok=%v, want %q", msg, ok, want)
	}

	// Unknown selector is reported as such.
	if _, ok := decodeCustomError(common.FromHex("0xdeadbeef")); ok {
		t.Error("unknown selector should not decode")
	}
}

// selectorOf returns the 4-byte selector for a registered signature by scanning
// the populated map, failing the test if it isn't found.
func selectorOf(t *testing.T, sig string) []byte {
	t.Helper()
	name := sig[:len(sig)-len("(uint256,uint256)")]
	for sel, ce := range customErrors {
		if ce.name == name && len(ce.args) == 2 {
			return common.FromHex("0x" + sel)
		}
	}
	t.Fatalf("signature %s not registered", sig)
	return nil
}
