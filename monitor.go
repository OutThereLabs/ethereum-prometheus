package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	peerCountGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_net_peerCount",
		Help: "The number of connected peers",
	})

	syncingRemainingBlocksGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_eth_syncing_remaining_blocks",
		Help: "Blocks remaining to sync",
	})
)

func init() {
	prometheus.MustRegister(peerCountGauge)
	prometheus.MustRegister(syncingRemainingBlocksGauge)
}

func main() {
	fmt.Println("Starting")

	var providerURL = os.Getenv("WEB3_PROVIDER_URL")
	if providerURL == "" {
		providerURL = "http://127.0.0.1:8545"
	}
	flag.StringVar(&providerURL, "providerURL", providerURL, "Web3 Provider URL")
	client, clientErr := rpc.Dial(providerURL)

	if clientErr != nil {
		fmt.Println("Error connecting: ", clientErr)
		return
	}

	ticker := time.NewTicker(time.Second * 5)
	go func() {
		for t := range ticker.C {
			updateStats(t, client)
		}
	}()

	var port = os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9990"
	}
	flag.StringVar(&port, "port", port, "Port number")

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(":"+port, nil)

	client.Close()
}

func updateStats(t time.Time, client *rpc.Client) {
	updateSyncing(client)
	updatePeers(client)
}

func updateSyncing(client *rpc.Client) {
	ec := ethclient.NewClient(client)
	syncing, err := ec.SyncProgress(context.Background())
	if err != nil {
		syncingRemainingBlocksGauge.Set(-1)
		return
	}

	if syncing == nil {
		syncingRemainingBlocksGauge.Set(0)
		return
	}

	remainingBlocks := syncing.HighestBlock - syncing.CurrentBlock

	syncingRemainingBlocksGauge.Set(float64(remainingBlocks))
}

func updatePeers(client *rpc.Client) {
	ctx := context.Background()
	var raw json.RawMessage
	if err := client.CallContext(ctx, &raw, "net_peerCount"); err != nil {
		peerCountGauge.Set(-1)
		return
	}

	var peerCount hexutil.Uint64
	if err := json.Unmarshal(raw, &peerCount); err != nil {
		peerCountGauge.Set(-1)
		return
	}

	peerCountGauge.Set(float64(peerCount))
}
