package engine

import (
	"context"
	"errors"
	"time"

	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/spv-wallet/conv"
	chainerrors "github.com/bitcoin-sv/spv-wallet/engine/chain/errors"
	"github.com/bitcoin-sv/spv-wallet/engine/spverrors"
	"github.com/rs/zerolog"
)

// timeForReceivingCallback indicates the time after which a broadcasted transaction should be checked (with Callback enabled)
// If ARC callback hasn't been received in this time, we need to check the transaction status "manually"
func timeForReceivingCallback() time.Time {
	return time.Now().Add(-time.Hour)
}

// timeForMineTransaction indicates the time after which transaction could be mined.
// It is used when tx is not broadcasted or ARC callback is off.
// We're checking the transaction status after potentially enough time has passed for it to be mined.
func timeForMineTransaction() time.Time {
	return time.Now().Add(-10 * time.Minute)
}

// problematicTxDelay indicates the time after which a transaction with an unknown status will be marked as problematic
// This is to prevent the system from trying to check old transactions that are not likely to be valid anymore
// NOTE: The SYNC task will check such "old" transactions at least once before marking them as problematic
func problematicTxDelay() time.Time {
	return time.Now().Add(-24 * time.Hour)
}

// processSyncTransactions is a crucial periodic task which try to query transactions which cannot be considered as finalized
// 1. It gets transaction IDs to sync
// 2. For every transaction check the status using ARC QueryTransaction API
// 3. If found - change the status
// 4. On error - try to rebroadcast (if needed) or
func processSyncTransactions(ctx context.Context, client *Client) {
	logger := client.Logger()
	db := client.Datastore().DB()

	recoverAndLog(logger)

	var txIDsToSync []struct {
		ID string
	}
	queryIDsCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var delayForBroadcastedTx time.Time
	if client.options.arcConfig.Callback != nil {
		delayForBroadcastedTx = timeForReceivingCallback()
	} else {
		delayForBroadcastedTx = timeForMineTransaction()
	}

	err := db.
		WithContext(queryIDsCtx).
		Model(&Transaction{}).
		Where("tx_status = ? AND created_at < ?", TxStatusBroadcasted, delayForBroadcastedTx).
		Or("tx_status = ? AND created_at < ?", TxStatusCreated, timeForMineTransaction()).
		Or("tx_status IS NULL"). // backward compatibility
		Find(&txIDsToSync).Error
	if err != nil {
		logger.Error().Err(err).Msg("Cannot fetch transactions to sync")
		return
	}

	logger.Info().Msgf("Transactions to SYNC: %d", len(txIDsToSync))

	for _, record := range txIDsToSync {
		txID := record.ID
		tx, err := getTransactionByID(ctx, "", txID, WithClient(client))
		if tx == nil || err != nil {
			// this should "never" happen, because we've just fetched the transaction IDs from the database
			logger.Error().Err(err).Str("txID", txID).Msg("Cannot get transaction by ID")
			continue
		}
		saveTx := func() {
			if err := tx.Save(ctx); err != nil {
				logger.Error().Err(err).Str("txID", txID).Msg("Cannot update transaction")
			}
		}
		updateStatus := func(newStatus TxStatus) {
			if newStatus == "" || tx.TxStatus == newStatus {
				return
			}
			tx.TxStatus = newStatus
			saveTx()
		}

		txInfo, err := client.Chain().QueryTransaction(ctx, txID)

		if err != nil {
			if errors.Is(err, chainerrors.ErrARCUnreachable) {
				// checking subsequent transactions is pointless if the broadcast server (ARC) is unreachable, will try again in the next cycle
				logger.Warn().Msgf("%s", err.Error())
				return
			}
			logger.Error().Err(err).Str("txID", txID).Msg("Cannot query transaction")
			if tx.UpdatedAt.Before(problematicTxDelay()) {
				updateStatus(TxStatusProblematic)
			}
			continue
		}

		if !txInfo.Found() {
			updateStatus(_handleUnknownTX(ctx, tx, logger))
			continue
		}

		if changedTxStatus := tx.UpdateFromBroadcastStatus(txInfo.TXStatus); !changedTxStatus {
			logger.Warn().Str("txID", txID).Str("ARCTxStatus", string(txInfo.TXStatus)).
				Msg("Queried transaction has not changed its status yet")

			// skip updating tx if the status hasn't changed
			continue
		}

		if tx.TxStatus == TxStatusMined {
			bump, err := transaction.NewMerklePathFromHex(txInfo.MerklePath)
			if err != nil {
				logger.Warn().
					Err(err).Str("txID", txID).Str("ARCMerklePath", txInfo.MerklePath).
					Msg("Queried transaction doesn't contain BUMP (MerklePath) even though it's mined")

				// skip updating tx on error when the status is Mined but the MerklePath is empty
				continue
			}

			tx.SetBUMP(bump)
			tx.BlockHash = txInfo.BlockHash
			tx.BlockHeight = _parseBlockHeight(txInfo.BlockHeight)

			logger.Info().Str("txID", txID).Str("blockHash", tx.BlockHash).Uint64("blockHeight", tx.BlockHeight).
				Msg("Queried transaction has been mined - received BUMP")
		}

		saveTx()
	}
}

func _handleUnknownTX(ctx context.Context, tx *Transaction, logger *zerolog.Logger) (newStatus TxStatus) {
	if tx.UpdatedAt.Before(problematicTxDelay()) {
		return TxStatusProblematic
	}

	shouldBroadcast := tx.TxStatus == TxStatusCreated
	if !shouldBroadcast {
		// do nothing - tx will be queried next time (until become "old" and marked problematic)
		return ""
	}

	err := broadcastTransaction(ctx, tx)
	if err == nil {
		return TxStatusBroadcasted
	}

	if errors.Is(err, chainerrors.ErrARCProblematicStatus) {
		return TxStatusProblematic
	}

	// tx will be broadcasted next time (until become "old" and marked problematic)
	logger.Warn().Str("txID", tx.ID).Msg("Broadcast attempt has failed in SYNC task")
	return ""
}

func _parseBlockHeight(h int64) uint64 {
	blockHeight, err := conv.Int64ToUint64(h)
	if err != nil {
		panic(spverrors.Wrapf(err, "cannot convert block height"))
	}
	return blockHeight
}
