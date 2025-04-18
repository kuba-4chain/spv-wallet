package arc

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	sdk "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/spv-wallet/engine/chain/errors"
	"github.com/bitcoin-sv/spv-wallet/engine/chain/models"
	"github.com/bitcoin-sv/spv-wallet/engine/spverrors"
	"github.com/go-resty/resty/v2"
)

// Custom ARC defined http status codes
const (
	StatusNotExtendedFormat             = 460
	StatusFeeTooLow                     = 465
	StatusCumulativeFeeValidationFailed = 473
)

// Broadcast submits a transaction to the ARC server and returns the transaction info.
func (s *Service) Broadcast(ctx context.Context, tx *sdk.Transaction) (*chainmodels.TXInfo, error) {
	result := &chainmodels.TXInfo{}
	arcErr := &chainmodels.ArcError{}
	req := s.prepareARCRequest(ctx).
		SetResult(result).
		SetError(arcErr)

	s.setCallbackHeaders(req)
	s.setWaitForHeader(req)

	txHex, err := s.prepareTxHex(ctx, tx)
	if err != nil {
		return nil, err
	}

	req.SetBody(requestBody{
		RawTx: txHex,
	})

	response, err := req.Post(fmt.Sprintf("%s/v1/tx", s.arcCfg.URL))

	if err != nil {
		return nil, s.wrapRequestError(err)
	}

	switch response.StatusCode() {
	case http.StatusOK:
		if result.TXStatus.IsProblematic() {
			return nil, chainerrors.ErrARCProblematicStatus.Wrap(spverrors.Newf("ARC Problematic tx status: %s", result.TXStatus))
		}
		return result, nil
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return nil, s.wrapARCError(chainerrors.ErrARCUnauthorized, arcErr)
	case StatusNotExtendedFormat:
		return nil, s.wrapARCError(chainerrors.ErrARCNotExtendedFormat, arcErr)
	case StatusFeeTooLow, StatusCumulativeFeeValidationFailed:
		return nil, s.wrapARCError(chainerrors.ErrARCWrongFee, arcErr)
	default:
		return nil, s.wrapARCError(chainerrors.ErrARCUnprocessable, arcErr)
	}
}

type requestBody struct {
	// Even though the name suggests that it is a raw transaction,
	// it is actually a hex encoded transaction
	// and can be in Raw, Extended Format or BEEF format.
	RawTx string `json:"rawTx"`
}

func (s *Service) prepareTxHex(ctx context.Context, tx *sdk.Transaction) (string, error) {
	efHex, err := s.efConverter.Convert(ctx, tx)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "", spverrors.ErrCtxInterrupted.Wrap(err)
	}
	if err != nil {
		// Log level is set to Info because it can happen in standard flow when source transaction is not from our wallet (and Junglebus is disabled)
		s.logger.Info().Err(err).Msg("Could not convert transaction to EFHex. Using raw transaction hex as a fallback.")
		return tx.Hex(), nil
	}
	return efHex, nil
}

func (s *Service) setCallbackHeaders(req *resty.Request) {
	cb := s.arcCfg.Callback
	if cb != nil && cb.URL != "" {
		req.SetHeader("X-CallbackUrl", cb.URL)

		if cb.Token != "" {
			req.SetHeader("X-CallbackToken", cb.Token)
		}
	}
}

func (s *Service) setWaitForHeader(req *resty.Request) {
	if s.arcCfg.WaitFor != "" {
		req.SetHeader("X-WaitFor", s.arcCfg.WaitFor)
	}
}
