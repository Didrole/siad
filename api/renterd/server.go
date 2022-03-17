package renterd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.sia.tech/core/consensus"
	"go.sia.tech/core/net/rhp"
	"go.sia.tech/core/types"
	"go.sia.tech/siad/v2/api"
	"go.sia.tech/siad/v2/internal/walletutil"
	"go.sia.tech/siad/v2/renter"
	"go.sia.tech/siad/v2/wallet"
)

type (
	// A Wallet can spend and receive siacoins.
	Wallet interface {
		Balance() types.Currency
		Address() types.Address
		NewAddress() types.Address
		Addresses() ([]types.Address, error)
		Transactions(since time.Time, max int) ([]wallet.Transaction, error)
		FundTransaction(txn *types.Transaction, amount types.Currency, pool []types.Transaction) ([]types.ElementID, func(), error)
		SignTransaction(vc consensus.ValidationContext, txn *types.Transaction, toSign []types.ElementID) error
	}

	// A Syncer can connect to other peers and synchronize the blockchain.
	Syncer interface {
		Addr() string
		Peers() []string
		Connect(addr string) error
		BroadcastTransaction(txn types.Transaction, dependsOn []types.Transaction)
	}

	// A TransactionPool can validate and relay unconfirmed transactions.
	TransactionPool interface {
		RecommendedFee() types.Currency
		Transactions() []types.Transaction
		AddTransaction(txn types.Transaction) error
		AddTransactionSet(txns []types.Transaction) error
	}

	// A ChainManager manages blockchain state.
	ChainManager interface {
		TipContext() consensus.ValidationContext
	}
)

type server struct {
	w  Wallet
	s  Syncer
	cm ChainManager
	tp TransactionPool
}

func (s *server) walletBalanceHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	api.WriteJSON(w, WalletBalanceResponse{
		Siacoins: s.w.Balance(),
	})
}

func (s *server) walletAddressHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	api.WriteJSON(w, s.w.NewAddress())
}

func (s *server) walletTransactionsHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var since time.Time
	if v := req.FormValue("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			http.Error(w, "invalid since value: "+err.Error(), http.StatusBadRequest)
			return
		}
		since = t
	}
	max := -1
	if v := req.FormValue("max"); v != "" {
		t, err := strconv.Atoi(v)
		if err != nil {
			http.Error(w, "invalid max value: "+err.Error(), http.StatusBadRequest)
			return
		}
		max = t
	}
	txns, err := s.w.Transactions(since, max)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	api.WriteJSON(w, txns)
}

func (s *server) syncerPeersHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	ps := s.s.Peers()
	sps := make([]SyncerPeerResponse, len(ps))
	for i, peer := range ps {
		sps[i] = SyncerPeerResponse{
			NetAddress: peer,
		}
	}
	api.WriteJSON(w, sps)
}

func (s *server) syncerConnectHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var scr SyncerConnectRequest
	if err := json.NewDecoder(req.Body).Decode(&scr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.s.Connect(scr.NetAddress); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *server) rhpScanHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var rsr RHPScanRequest
	if err := json.NewDecoder(req.Body).Decode(&rsr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := renter.NewSession(rsr.NetAddress, rsr.HostKey, s.cm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()

	settings, err := session.ScanSettings(req.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	api.WriteJSON(w, settings)
}

func (s *server) rhpFormHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var rfr RHPFormRequest
	if err := json.NewDecoder(req.Body).Decode(&rfr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := renter.NewSession(rfr.NetAddress, rfr.HostKey, s.cm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()

	contract, parent, err := session.FormContract(rfr.RenterKey, rfr.HostFunds, rfr.RenterFunds, rfr.EndHeight, rfr.HostSettings, s.w, s.tp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	api.WriteJSON(w, RHPFormResponse{contract, parent})
}

func (s *server) rhpRenewHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var rrr RHPRenewRequest
	if err := json.NewDecoder(req.Body).Decode(&rrr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := renter.NewSession(rrr.NetAddress, rrr.HostKey, s.cm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()

	settings, err := session.RegisterSettings(req.Context(), session.PayByContract(rrr.RenterKey.PublicKey()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if settings.StoragePrice.Cmp(rrr.HostSettings.StoragePrice) > 0 ||
		settings.Collateral.Cmp(rrr.HostSettings.Collateral) < 0 ||
		settings.ContractFee.Cmp(rrr.HostSettings.ContractFee) < 0 {
		http.Error(w, "current host settings are unacceptable", http.StatusBadRequest)
		return
	}

	contract, parent, err := session.RenewContract(rrr.RenterKey, rrr.Contract, rrr.RenterFunds, rrr.HostCollateral, rrr.Extension, s.w, s.tp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	api.WriteJSON(w, RHPRenewResponse{contract, parent})
}

func (s *server) rhpReadHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var rrr RHPReadRequest
	if err := json.NewDecoder(req.Body).Decode(&rrr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := renter.NewSession(rrr.NetAddress, rrr.HostKey, s.cm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()
	if err := session.Lock(req.Context(), rrr.ContractID, rrr.RenterKey); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	settings, err := session.RegisterSettings(req.Context(), session.PayByContract(rrr.RenterKey.PublicKey()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	price := rhp.RPCReadRenterCost(settings, rrr.Sections)
	if price.Cmp(rrr.MaxPrice) > 0 {
		http.Error(w, fmt.Sprintf("host price (%v) is unacceptable", price), http.StatusBadRequest)
		return
	}
	if err := session.Read(req.Context(), w, rrr.Sections); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *server) rhpAppendHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var rar RHPAppendRequest
	if err := json.Unmarshal([]byte(req.PostFormValue("meta")), &rar); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sectorFile, hdr, err := req.FormFile("sector")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	} else if hdr.Size != rhp.SectorSize {
		http.Error(w, fmt.Sprintf("wrong sector size (%v)", hdr.Size), http.StatusBadRequest)
		return
	}
	var sector [rhp.SectorSize]byte
	if _, err := io.ReadFull(sectorFile, sector[:]); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := renter.NewSession(rar.NetAddress, rar.HostKey, s.cm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()
	if err := session.Lock(req.Context(), rar.ContractID, rar.RenterKey); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	settings, err := session.RegisterSettings(req.Context(), session.PayByContract(rar.RenterKey.PublicKey()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	price := rhp.RPCWriteRenterCost(settings, session.LockedContract().Revision, []rhp.RPCWriteAction{{Type: rhp.RPCWriteActionAppend, Data: sector[:]}})
	if price.Cmp(rar.MaxPrice) > 0 {
		http.Error(w, fmt.Sprintf("host price (%v) is unacceptable", price), http.StatusBadRequest)
		return
	}
	root, err := session.Append(req.Context(), &sector)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	api.WriteJSON(w, RHPAppendResponse{root})
}

// NewServer returns an HTTP handler that serves the renterd API.
func NewServer(cm ChainManager, s Syncer, w *walletutil.TestingWallet, tp TransactionPool) http.Handler {
	srv := server{
		cm: cm,
		s:  s,
		w:  w,
		tp: tp,
	}
	mux := httprouter.New()

	mux.GET("/api/wallet/balance", srv.walletBalanceHandler)
	mux.GET("/api/wallet/address", srv.walletAddressHandler)
	mux.GET("/api/wallet/transactions", srv.walletTransactionsHandler)

	mux.GET("/api/syncer/peers", srv.syncerPeersHandler)
	mux.POST("/api/syncer/connect", srv.syncerConnectHandler)

	mux.POST("/api/rhp/scan", srv.rhpScanHandler)
	mux.POST("/api/rhp/form", srv.rhpFormHandler)
	mux.POST("/api/rhp/renew", srv.rhpRenewHandler)
	mux.POST("/api/rhp/read", srv.rhpReadHandler)
	mux.POST("/api/rhp/append", srv.rhpAppendHandler)

	return mux
}
