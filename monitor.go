package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	web3 "github.com/regcostajr/go-web3"
	"github.com/regcostajr/go-web3/providers"
)

var (
	blockNumberGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_eth_block_number",
		Help: "The number of the most recent block",
	})

	miningGuage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_eth_mining",
		Help: "Whether the node is mining or not",
	})

	peerCountGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_net_peerCount",
		Help: "The number of connected peers",
	})

	syncingStartingBlockGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_eth_syncing_starting_block",
		Help: "The starting block",
	})

	syncingCurrentBlockGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_eth_syncing_current_block",
		Help: "The current synced block",
	})

	syncingHighestBlockGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_eth_syncing_highest_block",
		Help: "The highest known block",
	})

	hashRateGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_eth_hash_rate",
		Help: "The current hash rate",
	})

	gasPriceGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_eth_gas_price",
		Help: "The current gas price",
	})

	// pendingTransactionsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	//     Name: "web3_eth_pending_transactions",
	//     Help: "The number of pending transactions"
	// })

	connection = web3.NewWeb3(providers.NewHTTPProvider("127.0.0.1:8454", 10, false))
)

func init() {
	prometheus.MustRegister(blockNumberGauge)
	prometheus.MustRegister(miningGuage)
	prometheus.MustRegister(peerCountGauge)
	prometheus.MustRegister(hashRateGauge)
	prometheus.MustRegister(syncingStartingBlockGauge)
	prometheus.MustRegister(syncingCurrentBlockGauge)
	prometheus.MustRegister(syncingHighestBlockGauge)

	gasPriceGauge.Set(-1)
	prometheus.MustRegister(gasPriceGauge)
}

func main() {
	fmt.Println("Starting")

	ticker := time.NewTicker(time.Second * 5)
	go func() {
		for t := range ticker.C {
			updateStats(t)
		}
	}()

	var providerURL = os.Getenv("WEB3_PROVIDER_URL")
	if providerURL == "" {
		providerURL = "127.0.0.1:8454"
	}
	flag.StringVar(&providerURL, "providerURL", providerURL, "Web3 Provider URL")
	connection = web3.NewWeb3(providers.NewHTTPProvider(providerURL, 10, false))

	var port = os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9990"
	}
	flag.StringVar(&port, "port", port, "Port number")

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(":"+port, nil)
}

func updateStats(t time.Time) {
	updateBlockNumber()
	updatePeerCount()
	updateMining()
	updateSyncing()
	updateHashrate()
	updateGasPrice()
}

func updateBlockNumber() {
	blockNumber, err := connection.Eth.GetBlockNumber()
	if err == nil {
		blockNumberGauge.Set(float64(blockNumber.ToUInt64()))
	}
}

func updateMining() {
	mining, err := connection.Eth.IsMining()
	if err == nil {
		if mining {
			miningGuage.Set(1)
		} else {
			miningGuage.Set(0)
		}
	}
}

func updatePeerCount() {
	peerCount, err := connection.Net.GetPeerCount()
	if err == nil {
		peerCountGauge.Set(float64(peerCount.ToUInt64()))
	}
}

func updateSyncing() {
	syncing, err := connection.Eth.IsSyncing()
	if err == nil {
		syncingStartingBlockGauge.Set(float64(syncing.StartingBlock.ToUInt64()))
		syncingCurrentBlockGauge.Set(float64(syncing.CurrentBlock.ToUInt64()))
		syncingHighestBlockGauge.Set(float64(syncing.HighestBlock.ToUInt64()))
	}
}

func updateHashrate() {
	hashRate, err := connection.Eth.GetHashRate()
	if err == nil {
		hashRateGauge.Set(float64(hashRate.ToUInt64()))
	}
}

func updateGasPrice() {
	gasPrice, err := connection.Eth.GetGasPrice()
	if err == nil {
		gasPriceGauge.Set(float64(gasPrice.ToUInt64()))
	}
}
