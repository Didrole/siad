package renter

import (
	"errors"
	"fmt"

	"go.sia.tech/core/net/mux"
	"go.sia.tech/core/net/rhp"
	"go.sia.tech/core/net/rpc"
	"go.sia.tech/core/types"
	"lukechampine.com/frand"
)

type (
	// A PaymentMethod pays for RPC usage during a renter-host session.
	PaymentMethod interface {
		isPayment()
	}

	payByEphemeralAccount struct {
		accountID types.PublicKey
		privkey   types.PrivateKey
		expiry    uint64
	}

	payByContract struct {
		contract        *rhp.Contract
		privkey         types.PrivateKey
		hostKey         types.PublicKey
		refundAccountID types.PublicKey
	}
)

// implements the PaymentMethod interface.
func (p *payByEphemeralAccount) isPayment() {}
func (p *payByContract) isPayment()         {}

func (s *Session) payByContract(stream *mux.Stream, payment *payByContract, amount types.Currency) error {
	// verify the contract has enough funds to pay the amount.
	switch {
	case payment.contract.Revision.ValidRenterOutput.Value.Cmp(amount) < 0:
		return errors.New("insufficient renter funds")
	case payment.contract.Revision.MissedRenterOutput.Value.Cmp(amount) < 0:
		return errors.New("insufficient renter funds")
	}

	vc, err := s.cm.TipContext()
	if err != nil {
		return fmt.Errorf("failed to get current validation context: %w", err)
	}

	// update the revision to pay for the usage.
	revision := payment.contract.Revision
	revision.RevisionNumber++
	revision.ValidRenterOutput.Value = revision.ValidRenterOutput.Value.Sub(amount)
	revision.MissedRenterOutput.Value = revision.MissedRenterOutput.Value.Sub(amount)
	revision.ValidHostOutput.Value = revision.ValidHostOutput.Value.Add(amount)
	revision.MissedHostOutput.Value = revision.MissedHostOutput.Value.Add(amount)
	revisionHash := vc.ContractSigHash(revision)

	req := &rhp.PayByContractRequest{
		RefundAccount: payment.refundAccountID,

		ContractID:        payment.contract.ID,
		NewRevisionNumber: revision.RevisionNumber,
		NewOutputs: rhp.ContractOutputs{
			MissedHostValue:   revision.MissedHostOutput.Value,
			MissedRenterValue: revision.MissedRenterOutput.Value,
			ValidHostValue:    revision.ValidHostOutput.Value,
			ValidRenterValue:  revision.ValidRenterOutput.Value,
		},
		Signature: payment.privkey.SignHash(revisionHash),
	}

	// write the payment request.
	if err := rpc.WriteRequest(stream, rhp.PayByContract, req); err != nil {
		return fmt.Errorf("failed to write contract payment request specifier: %w", err)
	}

	// read the payment response.
	var resp rhp.RPCRevisionSigningResponse
	if err := rpc.ReadResponse(stream, &resp); err != nil {
		return fmt.Errorf("failed to read contract payment response: %w", err)
	}

	// verify the host's signature.
	if !payment.hostKey.VerifyHash(revisionHash, resp.Signature) {
		return errors.New("could not verify host signature")
	}

	// update the contract to reflect the payment and new signatures
	payment.contract.Revision = revision
	payment.contract.RenterSignature = req.Signature
	payment.contract.HostSignature = resp.Signature
	return nil
}

func (s *Session) payByEphemeralAccount(stream *mux.Stream, payment *payByEphemeralAccount, amount types.Currency) error {
	var nonce [8]byte
	frand.Read(nonce[:])

	req := &rhp.PayByEphemeralAccountRequest{
		Message: rhp.WithdrawalMessage{
			AccountID: payment.accountID,
			Amount:    amount,
			Expiry:    payment.expiry,
			Nonce:     nonce,
		},
	}

	req.Signature = payment.privkey.SignHash(req.Message.SigHash())
	if err := rpc.WriteRequest(stream, rhp.PayByEphemeralAccount, req); err != nil {
		return fmt.Errorf("failed to write ephemeral account payment request specifier: %w", err)
	}

	return nil
}

func (s *Session) pay(stream *mux.Stream, payment PaymentMethod, amount types.Currency) error {
	switch p := payment.(type) {
	case *payByEphemeralAccount:
		return s.payByEphemeralAccount(stream, p, amount)
	case *payByContract:
		return s.payByContract(stream, p, amount)
	default:
		return fmt.Errorf("unrecognized payment method: %T", payment)
	}
}

// PayByContract creates a new contract payment method.
func (s *Session) PayByContract(contract *rhp.Contract, priv types.PrivateKey, refundAccountID types.PublicKey) PaymentMethod {
	return &payByContract{
		contract:        contract,
		privkey:         priv,
		hostKey:         s.hostKey,
		refundAccountID: refundAccountID,
	}
}

// PayByEphemeralAccount creates a new ephemeral account payment method.
func (s *Session) PayByEphemeralAccount(accountID types.PublicKey, priv types.PrivateKey, expiry uint64) PaymentMethod {
	return &payByEphemeralAccount{
		accountID: accountID,
		privkey:   priv,
		expiry:    expiry,
	}
}
