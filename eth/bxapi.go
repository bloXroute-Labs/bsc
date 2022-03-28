package eth

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"net/http"
	"net/url"
)

type BxAPIClients []BxAPIClient

type BxAPIClient struct {
	*rpc.Client
	endpoint *url.URL
}

func NewBxAPIClients(config *ethconfig.Config) BxAPIClients {
	clients := make(BxAPIClients, 0)

	if config.BxAPIAuthHeader == "" {
		return clients
	}

	contentType := "application/json"
	httpHeaders := make(http.Header, 3)
	httpHeaders.Set("accept", contentType)
	httpHeaders.Set("content-type", contentType)
	httpHeaders.Set("authorization", config.BxAPIAuthHeader)

	wsHeaders := make(http.Header, 1)
	wsHeaders.Set("authorization", config.BxAPIAuthHeader)

	for _, endpoint := range config.BxAPIEndpoints {
		var (
			client *rpc.Client
			err    error
		)
		switch endpoint.Scheme {
		case "http", "https":
			httpClient := new(http.Client)
			if config.BxAPIAllowInsecure {
				httpClient.Transport = &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}
			}
			client, err = rpc.DialHTTPWithClientHeaders(endpoint.String(), httpClient, httpHeaders)
		case "ws", "wss":
			client, err = rpc.DialWebsocketWithDialerHeaders(context.Background(), endpoint.String(), "", rpc.WebsocketDialer(), wsHeaders)
		default:
			err = errors.New("unknown URL scheme")
		}
		if err == nil {
			clients = append(clients, BxAPIClient{client, endpoint})
			log.Info("Registered bloXroute API client", "endpoint", endpoint)
		} else {
			log.Error("Failed registering bloXroute API client", "endpoint", endpoint, "err", err)
		}
	}

	return clients
}

func (b BxAPIClients) SubmitTx(ctx context.Context, tx *types.Transaction) {
	go func() {
		txHash := tx.Hash().String()
		txBytes, err := tx.MarshalBinary()
		if err != nil {
			log.Error("Failed serializing transaction for sending to bloXroute API", "err", err, "hash", txHash)
			return
		}

		request := blxrTxRequest{
			Transaction: hex.EncodeToString(txBytes),
		}

		for _, client := range b {
			var result blxrTxResponse
			err = client.CallContext(context.Background(), &result, "blxr_tx", request)
			if err != nil {
				if serr, ok := err.(rpc.DataError); ok {
					log.Error("Sending transaction to bloXroute API failed", "err", serr.Error(), "details", serr.ErrorData(), "hash", txHash, "url", client.endpoint)
				} else {
					log.Error("Sending transaction to bloXroute API failed", "err", err, "hash", txHash, "url", client.endpoint)
				}
			} else {
				log.Debug("Submitted transaction to bloXroute API", "hash", txHash, "url", client.endpoint)
			}
		}
	}()
}

type blxrTxRequest struct {
	Transaction    string `json:"transaction"`
	ValidatorsOnly bool   `json:"validators_only"`
}

type blxrTxResponse struct {
	Hash string `json:"txHash"`
}
