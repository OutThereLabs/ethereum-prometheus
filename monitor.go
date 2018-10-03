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

	maxRemainingBlocks uint64 = 10
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

	flag.Uint64Var(&maxRemainingBlocks, "max_remaining_blocks", 10, "Maximum remaining blocks to allow for readiness check")

	http.Handle("/metrics", prometheus.Handler())

	http.HandleFunc("/health/alive", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		remainingBlocks := remainingBlocks(client)

		if remainingBlocks > maxRemainingBlocks {
			w.WriteHeader(500)
			w.Write([]byte("error: syncing"))
		} else {
			w.WriteHeader(200)
			w.Write([]byte("OK"))
		}
	})

	http.ListenAndServe(":"+port, nil)

	client.Close()
}

func updateStats(t time.Time, client *rpc.Client) {
	updateSyncing(client)
	updatePeers(client)
}

func remainingBlocks(client *rpc.Client) uint64 {
	ec := ethclient.NewClient(client)
	syncing, err := ec.SyncProgress(context.Background())
	if err != nil {
		syncingRemainingBlocksGauge.Set(-1)
		return 0
	}

	if syncing == nil {
		syncingRemainingBlocksGauge.Set(0)
		return 0
	}

	remainingBlocks := syncing.HighestBlock - syncing.CurrentBlock

	return remainingBlocks
}

func updateSyncing(client *rpc.Client) {
	remainingBlocks := remainingBlocks(client)

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
