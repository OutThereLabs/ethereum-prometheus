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
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	blockGapGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "web3_block_gap",
		Help: "Block gap, the remaining warp sync blocks",
	})
)

func init() {
	prometheus.MustRegister(peerCountGauge)
	prometheus.MustRegister(syncingRemainingBlocksGauge)
	prometheus.MustRegister(blockGapGauge)
}

func shouldEnableParity() bool {
	var enableParityString = os.Getenv("ENABLE_PARITY")
	var enableParity = false
	if enableParityString == "true" {
		enableParity = true
	}
	flag.BoolVar(&enableParity, "enableParity", enableParity, "Enable parity")
	return enableParity
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

	enableParity := shouldEnableParity()

	ticker := time.NewTicker(time.Second * 5)
	go func() {
		for t := range ticker.C {
			updateStats(t, client, enableParity)
		}
	}()

	var port = os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9990"
	}
	flag.StringVar(&port, "port", port, "Port number")

	flag.Uint64Var(&maxRemainingBlocks, "max_remaining_blocks", 10, "Maximum remaining blocks to allow for readiness check")

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/health/alive", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		remainingBlocks, err := remainingBlocks(client)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("error: syncing"))
			return
		}

		var blockGap uint64

		if enableParity {
			chainStatus, err := getChainStatus(client)
			if err == nil {
				if len(chainStatus.BlockGap) == 2 {
					blockGap = uint64(chainStatus.BlockGap[1] - chainStatus.BlockGap[0])
				} else {
					blockGap = 0
				}
			} else {
				fmt.Println("Error getting chain status: ", err)
			}
		}

		if remainingBlocks > maxRemainingBlocks || blockGap > 0 {
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

func updateStats(t time.Time, client *rpc.Client, enableParity bool) {
	if enableParity {
		updateChainStatus(client)
	}

	updateSyncing(client)
	updatePeers(client)
}

type chainStatus struct {
	BlockGap []hexutil.Uint64 `json:"blockGap,omitempty"`
}

func getChainStatus(client *rpc.Client) (*chainStatus, error) {
	ctx := context.Background()

	var raw json.RawMessage
	if err := client.CallContext(ctx, &raw, "parity_chainStatus"); err != nil {
		return nil, err
	}

	var result *chainStatus
	if err := json.Unmarshal(raw, &result); err != nil {
		if err != nil {
			fmt.Println("Could not decode ")
		}
		return nil, err
	}

	return result, nil
}

func updateChainStatus(client *rpc.Client) {
	chainStatus, err := getChainStatus(client)

	if err == nil {
		if len(chainStatus.BlockGap) == 2 {
			blockGapGauge.Set(float64(chainStatus.BlockGap[1] - chainStatus.BlockGap[0]))
		} else {
			blockGapGauge.Set(0)
		}
	}
}

func remainingBlocks(client *rpc.Client) (uint64, error) {
	ec := ethclient.NewClient(client)
	syncing, err := ec.SyncProgress(context.Background())
	if err != nil {
		syncingRemainingBlocksGauge.Set(-1)
		return 0, err
	}

	if syncing == nil {
		syncingRemainingBlocksGauge.Set(0)
		return 0, nil
	}

	remainingBlocks := syncing.HighestBlock - syncing.CurrentBlock

	return remainingBlocks, nil
}

func updateSyncing(client *rpc.Client) {
	remainingBlocks, _ := remainingBlocks(client)

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
