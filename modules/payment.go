package modules

import (
	"io"

	"gitlab.com/NebulousLabs/Sia/build"
	"gitlab.com/NebulousLabs/Sia/crypto"
	"gitlab.com/NebulousLabs/Sia/encoding"
	"gitlab.com/NebulousLabs/Sia/types"
	"gitlab.com/NebulousLabs/errors"
	"gitlab.com/NebulousLabs/siamux"
)

const (
	// WithdrawalNonceSize is the size of the nonce in the WithdralMessage
	WithdrawalNonceSize = 8
)

var (
	// ErrUnknownPaymentMethod occurs when the payment method specified in the
	// PaymentRequest object is unknown. The possible options are outlined below
	// under "Payment identifiers".
	ErrUnknownPaymentMethod = errors.New("unknown payment method")

	// ErrInvalidPaymentMethod occurs when the payment method is not accepted
	// for a specific RPC.
	ErrInvalidPaymentMethod = errors.New("invalid payment method")

	// ErrInsufficientPaymentForRPC is returned when the provided payment was
	// lower than the cost of the RPC.
	ErrInsufficientPaymentForRPC = errors.New("Insufficient payment, the provided payment did not cover the cost of the RPC.")

	// ErrExpiredRPCPriceTable is returned when the renter performs an RPC call
	// and the current block height exceeds the expiry block height of the RPC
	// price table.
	ErrExpiredRPCPriceTable = errors.New("Expired RPC price table, ensure you have the latest prices by calling the updatePriceTable RPC.")

	// ErrWithdrawalsInactive occurs when the host is not synced yet. If that is
	// the case the account manager does not allow trading money from the
	// ephemeral accounts.
	ErrWithdrawalsInactive = errors.New("ephemeral account withdrawals are inactive because the host is not synced")

	// ErrWithdrawalExpired occurs when the withdrawal message's expiry block
	// height is in the past.
	ErrWithdrawalExpired = errors.New("ephemeral account withdrawal message expired")

	// ErrWithdrawalExtremeFuture occurs when the withdrawal message's expiry
	// block height is too far into the future.
	ErrWithdrawalExtremeFuture = errors.New("ephemeral account withdrawal message expires too far into the future")

	// ErrWithdrawalInvalidSignature occurs when the signature provided with the
	// withdrawal message was invalid.
	ErrWithdrawalInvalidSignature = errors.New("ephemeral account withdrawal message signature is invalid")
)

// PaymentProcessor is the interface implemented when receiving payment for an
// RPC.
type PaymentProcessor interface {
	// ProcessPayment takes a stream and handles the payment request objects
	// sent by the caller. Returns an object that implements the PaymentDetails
	// interface, or an error in case of failure.
	ProcessPayment(stream siamux.Stream) (PaymentDetails, error)
}

// PaymentDetails is an interface that defines method that give more information
// about the details of a processed payment.
type PaymentDetails interface {
	AccountID() AccountID
	Amount() types.Currency
	AddedCollateral() types.Currency
}

// Payment identifiers
var (
	PayByContract         = types.NewSpecifier("PayByContract")
	PayByEphemeralAccount = types.NewSpecifier("PayByEphemAcc")
)

// ZeroAccountID is the only account id that is allowed to be invalid.
var ZeroAccountID = AccountID("")

type (
	// AccountID is the unique identifier of an ephemeral account on the host.
	// It should always be a valid representation of types.SiaPublicKey or an
	// empty string.
	AccountID string

	// PaymentRequest identifies the payment method. This can be either
	// PayByContract or PayByEphemeralAccount
	PaymentRequest struct {
		Type types.Specifier
	}

	// PayByEphemeralAccountRequest holds all payment details to pay from an
	// ephemeral account.
	PayByEphemeralAccountRequest struct {
		Message   WithdrawalMessage
		Signature crypto.Signature
		Priority  int64
	}

	// PayByEphemeralAccountResponse is the object sent in response to the
	// PayByEphemeralAccountRequest
	PayByEphemeralAccountResponse struct {
		Balance types.Currency // balance of the account before withdrawal
	}

	// PayByContractRequest holds all payment details to pay from a file
	// contract.
	PayByContractRequest struct {
		ContractID           types.FileContractID
		NewRevisionNumber    uint64
		NewValidProofValues  []types.Currency
		NewMissedProofValues []types.Currency
		RefundAccount        AccountID
		Signature            []byte
	}

	// PayByContractResponse is the object sent in response to the
	// PayByContractRequest
	PayByContractResponse struct {
		Balance   types.Currency // balance of the refund account before withdrawal
		Signature crypto.Signature
	}

	// WithdrawalMessage contains all details to spend from an ephemeral account
	WithdrawalMessage struct {
		Account AccountID
		Expiry  types.BlockHeight
		Amount  types.Currency
		Nonce   [WithdrawalNonceSize]byte
	}

	// Receipt is returned by the host after a successful deposit into an
	// ephemeral account and can be used as proof of payment.
	Receipt struct {
		Host      types.SiaPublicKey
		Account   AccountID
		Amount    types.Currency
		Timestamp int64
	}
)

// FromSPK creates an AccountID from a SiaPublicKey. This assumes that the
// provided key is valid and won't perform additional checks.
func (aid *AccountID) FromSPK(spk types.SiaPublicKey) {
	*aid = AccountID(spk.String())
}

// IsZeroAccount returns whether or not the account id matche the empty string.
func (aid AccountID) IsZeroAccount() bool {
	return aid == ZeroAccountID
}

// LoadString loads an account id from a string.
func (aid *AccountID) LoadString(s string) error {
	var spk types.SiaPublicKey
	err := spk.LoadString(s)
	if err != nil {
		return errors.AddContext(err, "failed to load account id from string")
	}
	aid.FromSPK(spk)
	return nil
}

// MarshalSia implements the SiaMarshaler interface.
func (aid AccountID) MarshalSia(w io.Writer) error {
	e := encoding.NewEncoder(w)
	_ = e.WritePrefixedBytes([]byte(aid))
	return e.Err()
}

// UnmarshalSia implements the SiaMarshaler interface.
func (aid *AccountID) UnmarshalSia(r io.Reader) error {
	d := encoding.NewDecoder(r, encoding.DefaultAllocLimit)
	idBytes := d.ReadPrefixedBytes()
	if d.Err() != nil {
		return d.Err()
	}
	if len(idBytes) == 0 {
		*aid = ZeroAccountID
		return nil
	}
	return aid.LoadString(string(idBytes))
}

// PK returns the id as a crypto.PublicKey.
func (aid AccountID) PK() (pk crypto.PublicKey) {
	spk := aid.SPK()
	if len(spk.Key) != len(pk) {
		panic("key len mismatch between crypto.Publickey and types.SiaPublicKey")
	}
	copy(pk[:], spk.Key)
	return
}

// SPK returns the account id as a types.SiaPublicKey.
func (aid AccountID) SPK() (spk types.SiaPublicKey) {
	if aid.IsZeroAccount() {
		build.Critical("should never use the zero account")
	}
	err := spk.LoadString(string(aid))
	if err != nil {
		build.Critical("account id should never fail to be loaded as a SiaPublicKey")
	}
	return
}

// Validate checks the WithdrawalMessage's expiry and signature. If the
// signature is invalid, or if the WithdrawlMessage is already expired, or it
// expires too far into the future, an error is returned.
func (wm *WithdrawalMessage) Validate(blockHeight, expiry types.BlockHeight, hash crypto.Hash, sig crypto.Signature) error {
	return errors.Compose(
		wm.ValidateExpiry(blockHeight, expiry),
		wm.ValidateSignature(hash, sig),
	)
}

// ValidateExpiry returns an error if the withdrawal message is either already
// expired or if it expires too far into the future
func (wm *WithdrawalMessage) ValidateExpiry(blockHeight, expiry types.BlockHeight) error {
	// Verify the current blockheight does not exceed the expiry
	if blockHeight > wm.Expiry {
		return ErrWithdrawalExpired
	}
	// Verify the withdrawal is not too far into the future
	if wm.Expiry > expiry {
		return ErrWithdrawalExtremeFuture
	}
	return nil
}

// ValidateSignature returns an error if the provided signature is invalid
func (wm *WithdrawalMessage) ValidateSignature(hash crypto.Hash, sig crypto.Signature) error {
	err := crypto.VerifyHash(hash, wm.Account.PK(), sig)
	if err != nil {
		return errors.Extend(err, ErrWithdrawalInvalidSignature)
	}
	return nil
}
