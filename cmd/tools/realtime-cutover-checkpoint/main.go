package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	realtimeDelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
)

func main() {
	transitionReceiptPath := flag.String("transition-receipt", "", "path to the authority transition receipt JSON")
	expectedNodesPath := flag.String("expected-nodes", "", "path to the expected-node manifest JSON")
	checkpointManifestPath := flag.String("checkpoint-manifest", "", "path to the dual-group checkpoint manifest JSON")
	outputPath := flag.String("output", "", "new immutable checkpoint bundle path")
	confirm := flag.Bool("confirm", false, "confirm checkpoint capture and immutable publication")
	flag.Parse()

	if !*confirm {
		fatal(fmt.Errorf("-confirm is required"))
	}
	if strings.TrimSpace(*transitionReceiptPath) == "" || strings.TrimSpace(*expectedNodesPath) == "" ||
		strings.TrimSpace(*checkpointManifestPath) == "" || strings.TrimSpace(*outputPath) == "" {
		fatal(fmt.Errorf("transition receipt, expected nodes, checkpoint manifest, and output paths are required"))
	}
	transition, err := readStrictJSON[realtimeDelivery.FenceTransitionReceipt](*transitionReceiptPath)
	if err != nil {
		fatal(fmt.Errorf("read authority transition receipt: %w", err))
	}
	expectedNodes, err := readStrictJSON[realtimeDelivery.FenceExpectedNodeManifest](*expectedNodesPath)
	if err != nil {
		fatal(fmt.Errorf("read expected-node manifest: %w", err))
	}
	checkpointManifest, err := readStrictJSON[realtimeDelivery.DualGroupCheckpointManifest](*checkpointManifestPath)
	if err != nil {
		fatal(fmt.Errorf("read dual-group checkpoint manifest: %w", err))
	}
	if err := config.Load(); err != nil {
		fatal(fmt.Errorf("load config: %w", err))
	}
	kafkaConfig := config.KafkaConfig()
	if !kafkaConfig.Enabled {
		fatal(fmt.Errorf("Kafka must be enabled for delivery checkpoint capture"))
	}
	if err := cache.InitRedis(); err != nil {
		fatal(fmt.Errorf("initialize Redis: %w", err))
	}
	defer func() { _ = cache.RDB.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	realtimeConfig := config.RealtimeConfig()
	aggregator, err := realtimeDelivery.NewRedisFenceObservationAggregator(
		cache.RDB, realtimeConfig.FencingKey+":observation:", time.Now,
	)
	if err != nil {
		fatal(err)
	}
	proof, err := aggregator.Aggregate(ctx, expectedNodes, transition)
	if err != nil {
		fatal(err)
	}
	dialTimeout := time.Duration(kafkaConfig.DialTimeoutSeconds) * time.Second
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	clientID := strings.TrimSpace(kafkaConfig.ClientID)
	if clientID == "" {
		clientID = "dipole"
	}
	source, err := realtimeDelivery.NewKafkaGoCheckpointSource(kafkaConfig.Brokers, clientID+"-cutover-checkpoint", dialTimeout)
	if err != nil {
		fatal(err)
	}
	collector, err := realtimeDelivery.NewDualGroupCheckpointCollector(source, time.Now)
	if err != nil {
		fatal(err)
	}
	checkpoint, err := collector.Capture(ctx, checkpointManifest, proof)
	if err != nil {
		fatal(err)
	}
	bundle, err := realtimeDelivery.NewCheckpointBundle(proof, checkpoint)
	if err != nil {
		fatal(err)
	}
	if err := realtimeDelivery.WriteImmutableCheckpointBundle(*outputPath, bundle); err != nil {
		fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(bundle); err != nil {
		fatal(fmt.Errorf("encode delivery checkpoint bundle: %w", err))
	}
}

func readStrictJSON[T any](path string) (T, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		var value T
		return value, err
	}
	return realtimeDelivery.DecodeStrictJSON[T](payload)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
