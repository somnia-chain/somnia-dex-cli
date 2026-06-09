package cmd

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

// customError decodes a protocol custom-error revert: its name, an optional
// plain-language hint, and the ABI types of any arguments.
type customError struct {
	name string
	hint string
	args abi.Arguments
}

// customErrors maps a 4-byte error selector (lowercase hex, no 0x prefix) to its
// decoder. Populated from canonical signatures so the selectors can never drift.
var customErrors = map[string]customError{}

// registerError computes a custom error's selector from its canonical Solidity
// signature (e.g. "InsufficientBalance(uint256,uint256)") and records its decoder.
func registerError(sig, hint string) {
	open := strings.IndexByte(sig, '(')
	sel := fmt.Sprintf("%x", crypto.Keccak256([]byte(sig))[:4])

	var args abi.Arguments
	if inner := sig[open+1 : len(sig)-1]; inner != "" {
		for _, typeName := range strings.Split(inner, ",") {
			t, err := abi.NewType(typeName, "", nil)
			if err != nil {
				return // skip errors whose types we can't model rather than mis-decode
			}
			args = append(args, abi.Argument{Type: t})
		}
	}

	customErrors[sel] = customError{name: sig[:open], hint: hint, args: args}
}

// decodeCustomError returns a human-readable message for a 4-byte custom-error
// revert payload, or ("", false) if the selector is unknown. Known argument
// values are appended; an unparseable argument tail is omitted, not fatal.
func decodeCustomError(data []byte) (string, bool) {
	if len(data) < 4 {
		return "", false
	}
	ce, ok := customErrors[fmt.Sprintf("%x", data[:4])]
	if !ok {
		return "", false
	}

	msg := ce.name
	if len(ce.args) > 0 {
		if vals, err := ce.args.UnpackValues(data[4:]); err == nil && len(vals) > 0 {
			parts := make([]string, len(vals))
			for i, v := range vals {
				parts[i] = fmt.Sprint(v)
			}
			msg += "(" + strings.Join(parts, ", ") + ")"
		}
	}
	if ce.hint != "" {
		msg += ": " + ce.hint
	}
	return msg, true
}

func init() {
	// SpotStopOrderRegistry (stop / take-profit orders).
	registerError("InsufficientSomiPayment()", "wrong native payment attached; the registry requires an exact amount")
	registerError("OrderDoesNotExist()", "the stop order has already triggered or been cancelled")
	registerError("OrderIdMismatch()", "stop order id no longer matches the stored order (triggered, cancelled, or reused)")
	registerError("InvalidOrderOwner()", "you are not the owner of this stop order")
	registerError("NoActiveSubscription()", "the stop-order registry is not currently active")
	registerError("TriggerTooCloseToEma()", "trigger price is too close to the current mark price")
	registerError("LimitPriceIncompatibleWithTrigger()", "limit price is on the wrong side of the trigger price")
	registerError("QuantityNotAlignedToLotSize()", "quantity must be a multiple of the market lot size")
	registerError("PriceNotAlignedToTickSize()", "price must be a multiple of the market tick size")
	registerError("InsufficientVaultBalance()", "insufficient vault balance to back the order at trigger time")
	registerError("InvalidTriggerPrice()", "")
	registerError("InvalidLimitPrice()", "")
	registerError("ExceedsWithdrawableBalance()", "")
	registerError("NothingToClaim()", "")
	registerError("WithdrawalFailed()", "")

	// SpotPool order book and vault.
	registerError("InsufficientBalance(uint256,uint256)", "insufficient balance (available, required)")
	registerError("InvalidPrice(uint256,uint256)", "price not aligned to tick size (price, tickSize)")
	registerError("InvalidQuantity(uint256,uint256)", "quantity not aligned to lot size (quantity, constraint)")
	registerError("QuantityBelowMinimum(uint256,uint256)", "quantity below the market minimum (quantity, minimum)")
	registerError("QuantityBelowMinimum()", "quantity is below the market minimum")
	registerError("ExpiredOrderMustBeCancelled(uint128)", "order has expired; cancel it instead of reducing")
	registerError("IncorrectSender(address,address)", "caller is not the order owner (sender, expected)")
	registerError("IncorrectOrder()", "order no longer exists (filled, cancelled, or swept)")
	registerError("FillOrKillNotFillable()", "fill-or-kill order could not be filled in full")
	registerError("OnlyApprovedContracts()", "operator is not approved for this action")
	registerError("UseDepositNative()", "use the native deposit path for the native token")
	registerError("UnexpectedNativeDeposit()", "native value sent to a pool side that is not native")
	registerError("NativeTokenTransferFailed()", "native token transfer failed")
	registerError("InvalidDepositOrWithdrawal()", "token is not the pool's base or quote token")
	registerError("CircuitBreakerTriggered()", "trading is halted by the circuit breaker")
	registerError("BuilderCodesNotSupported()", "builder codes are disabled on this pool")
	registerError("BuilderNotApproved()", "builder has not been approved by the order owner")
	registerError("InvalidBuilder()", "")
}
