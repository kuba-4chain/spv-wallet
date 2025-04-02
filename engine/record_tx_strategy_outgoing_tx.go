package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/4chain-AG/gateway-overlay/pkg/token_engine/specifications"
	trx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/spv-wallet/engine/spverrors"
)

type outgoingTx struct {
	SDKTx          *trx.Transaction
	RelatedDraftID string
	XPubKey        string
	txID           string
	isExtended     bool
}

func (strategy *outgoingTx) Name() string {
	return "outgoing_tx"
}

func (strategy *outgoingTx) Execute(ctx context.Context, c ClientInterface, opts []ModelOps) (*Transaction, error) {
	logger := c.Logger()

	var transaction *Transaction
	var err error

	if transaction, err = strategy.createOutgoingTxToRecord(ctx, c, opts); err != nil {
		return nil, spverrors.ErrCreateOutgoingTxFailed.Wrap(err)
	}

	if err = transaction.processUtxos(ctx); err != nil {
		return nil, err
	}

	if err = transaction.Save(ctx); err != nil {
		return nil, spverrors.ErrDuringSaveTx
	}

	if _shouldNotifyP2P(transaction) {
		if err = processP2PTransaction(ctx, transaction); err != nil {
			return nil, _handleNotifyP2PError(ctx, c, transaction, err)
		}
	}

	// transaction can be updated by internal_incoming_tx
	transaction, err = getTransactionByID(ctx, transaction.XPubID, transaction.ID, WithClient(c))
	if transaction == nil || err != nil {
		return nil, spverrors.ErrInternal.Wrap(err)
	}

	if transaction.TxStatus == TxStatusBroadcasted {
		// no need to broadcast twice
		// this also means that if the transaction contained the token - it was validated in overlay
		return transaction, nil
	}

	if _isTokenTransaction(transaction.parsedTx) {
		logger.Info().Str("strategy", "outgoing").Msg("Token transaction FOUND")
		err = c.Tokens().VerifyAndSaveTokenTransfer(ctx, transaction.Hex)
		// TODO: should we ignore the error and broadcast anyway if the receiver accepted?
		if err != nil {
			return nil, spverrors.ErrTokenValidationFailed.Wrap(err)
		}
		logger.Info().Str("strategy", "outgoing").Msg("Token transaction successfully VALIDATED")
	}

	if err = broadcastTransaction(ctx, transaction); err != nil {
		logger.Warn().Str("txID", transaction.ID).Msgf("broadcasting failed in outgoingTx strategy")
		// ignore error, transaction most likely is successfully broadcasted by payment receiver
		// TODO: return a Warning to a client
	} else {
		transaction.TxStatus = TxStatusBroadcasted
	}

	if err = transaction.Save(ctx); err != nil {
		logger.Error().Str("txID", transaction.ID).Err(err).Msg("Outgoing transaction has been processed but failed save to db")
	}

	return transaction, nil
}

func (strategy *outgoingTx) Validate() error {
	if strategy.SDKTx == nil {
		return ErrMissingFieldHex
	}

	if strategy.RelatedDraftID == "" {
		return spverrors.ErrEmptyRelatedDraftID
	}

	if strategy.XPubKey == "" {
		return spverrors.ErrEmptyXpubKey
	}

	return nil // is valid
}

func (strategy *outgoingTx) TxID() string {
	if strategy.txID == "" {
		strategy.txID = strategy.SDKTx.TxID().String()
	}
	return strategy.txID
}

func (strategy *outgoingTx) LockKey() string {
	return fmt.Sprintf("outgoing-%s", strategy.TxID())
}

func (strategy *outgoingTx) createOutgoingTxToRecord(ctx context.Context, c ClientInterface, opts []ModelOps) (*Transaction, error) {
	// Create NEW transaction model
	newOpts := c.DefaultModelOptions(append(opts, WithXPub(strategy.XPubKey), New())...)
	tx := txFromSDKTx(strategy.SDKTx, strategy.isExtended, newOpts...)
	tx.DraftID = strategy.RelatedDraftID

	if err := _hydrateOutgoingWithDraft(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func _hydrateOutgoingWithDraft(ctx context.Context, tx *Transaction) error {
	draft, err := getDraftTransactionID(ctx, tx.XPubID, tx.DraftID, tx.GetOptions(false)...)
	if err != nil {
		return err
	}

	if draft == nil {
		return spverrors.ErrCouldNotFindDraftTx
	}

	if len(draft.Configuration.Outputs) == 0 {
		return spverrors.ErrDraftTxHasNoOutputs
	}

	if draft.Configuration.Sync == nil {
		draft.Configuration.Sync = &SyncConfig{
			Broadcast:        true,
			BroadcastInstant: true,
			PaymailP2P:       true,
			SyncOnChain:      true,
		}
	}

	tx.draftTransaction = draft

	return nil // success
}

func _shouldNotifyP2P(tx *Transaction) bool {
	return slices.ContainsFunc(tx.draftTransaction.Configuration.Outputs, func(o *TransactionOutput) bool {
		return o.PaymailP4 != nil && o.PaymailP4.ResolutionType == ResolutionTypeP2P
	})
}

func _handleNotifyP2PError(ctx context.Context, c ClientInterface, transaction *Transaction, cause error) error {
	logger := c.Logger()

	p2pError := spverrors.ErrProcessP2PTx.Wrap(cause)

	saveAsProblematic := func() {
		transaction.TxStatus = TxStatusProblematic
		if err := transaction.Save(ctx); err != nil {
			logger.Error().Str("txID", transaction.ID).Err(err).Msg("Error saving transaction after notifyP2P failed")
		}
	}

	txInfo, err := c.Chain().QueryTransaction(ctx, transaction.ID)
	if err != nil {
		saveAsProblematic()
		return p2pError.Wrap(err)
	}

	if txInfo.Found() {
		saveAsProblematic()
		return p2pError.Wrap(spverrors.ErrTxRevertFoundOnChain)
	}

	if err := c.RevertTransaction(ctx, transaction.ID); err != nil {
		saveAsProblematic()
		return p2pError.Wrap(err)
	}

	// RevertTransaction saves the transaction itself as REVERTED
	return p2pError
}

func _isTokenTransaction(tx *trx.Transaction) bool {
	for _, in := range tx.Inputs {
		if in.SourceTxOutput() == nil {
			// tx is not in EF - ignore for now (consider as a normal tx)
			return false
		}

		ops, err := in.SourceTxOutput().LockingScript.ParseOps()
		if err != nil {
			// something wrong with parsing the script - ignore
			return false
		}

		spec := new(specifications.Bsv21EnvelopeSpec)
		hasBsvEnvelope, _ := spec.IsSatisfiedBy(ops)

		if hasBsvEnvelope {
			// this is a token tx
			return true
		}
	}

	return false
}
