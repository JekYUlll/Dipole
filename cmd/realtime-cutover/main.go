package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	realtimeDelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	"github.com/JekYUlll/Dipole/internal/store"
)

type executorFactory func(
	workspace *realtimeDelivery.CutoverAttemptWorkspace,
	operator string,
	leaseDuration time.Duration,
) (realtimeDelivery.CutoverAttemptActionExecutor, func(), error)

type statusOutput struct {
	AttemptID    string                               `json:"attempt_id"`
	ManifestSHA  string                               `json:"manifest_sha256"`
	State        realtimeDelivery.CutoverAttemptState `json:"state"`
	LastSequence uint64                               `json:"last_sequence"`
	HeadSHA      string                               `json:"head_sha256"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, time.Now, realExecutorFactory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer, now func() time.Time, factory executorFactory) error {
	flags := flag.NewFlagSet("dipole-realtime-cutover", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	operation := flags.String("operation", "status", "create, status, advance, or rollback")
	directory := flags.String("attempt-dir", "", "durable cutover attempt workspace directory")
	inputsPath := flags.String("inputs", "", "strict attempt inputs JSON for create")
	attemptID := flags.String("attempt-id", "", "bounded attempt identity for create")
	source := flags.String("source", "", "source authority for create")
	target := flags.String("target", "", "target authority for create")
	maxInterruption := flags.Duration("max-interruption", time.Minute, "maximum no-authority window")
	operator := flags.String("operator", "", "audited operator label for advance/rollback")
	leaseDuration := flags.Duration("lease-duration", 10*time.Minute, "authority lease duration for transitions")
	confirm := flags.Bool("confirm", false, "confirm create/advance/rollback mutation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	*operation = strings.ToLower(strings.TrimSpace(*operation))
	if strings.TrimSpace(*directory) == "" {
		return fmt.Errorf("-attempt-dir is required")
	}
	if *operation == "create" {
		if !*confirm {
			return fmt.Errorf("-confirm is required for create")
		}
		return runCreate(output, now, *directory, *inputsPath, *attemptID, *source, *target, *maxInterruption)
	}
	workspace, err := realtimeDelivery.LoadCutoverAttemptWorkspace(*directory)
	if err != nil {
		return err
	}
	if *operation == "status" {
		return encodeOutput(output, statusOutput{
			AttemptID: workspace.Journal.Manifest.AttemptID, ManifestSHA: workspace.Journal.ManifestSHA256,
			State: workspace.Journal.Projection.State, LastSequence: workspace.Journal.Projection.LastSequence,
			HeadSHA: workspace.Journal.HeadSHA256,
		})
	}
	if *operation != "advance" && *operation != "rollback" {
		return fmt.Errorf("unsupported cutover operation %q", *operation)
	}
	if !*confirm {
		return fmt.Errorf("-confirm is required for %s", *operation)
	}
	if strings.TrimSpace(*operator) == "" {
		return fmt.Errorf("-operator is required for %s", *operation)
	}
	if factory == nil {
		return fmt.Errorf("cutover executor factory is unavailable")
	}
	executor, cleanup, err := factory(workspace, *operator, *leaseDuration)
	if err != nil {
		return err
	}
	defer cleanup()
	orchestrator, err := realtimeDelivery.NewCutoverAttemptOrchestrator(workspace.Journal, executor, now)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result realtimeDelivery.CutoverAttemptAdvance
	if *operation == "advance" {
		result, err = orchestrator.Advance(ctx)
	} else {
		result, err = orchestrator.RequestRollback(ctx)
	}
	if err != nil {
		return err
	}
	return encodeOutput(output, result)
}

func runCreate(
	output io.Writer,
	now func() time.Time,
	directory, inputsPath, attemptID, sourceValue, targetValue string,
	maxInterruption time.Duration,
) error {
	if strings.TrimSpace(inputsPath) == "" || now == nil {
		return fmt.Errorf("-inputs is required for create")
	}
	payload, err := os.ReadFile(inputsPath)
	if err != nil {
		return fmt.Errorf("read cutover attempt inputs: %w", err)
	}
	inputs, err := realtimeDelivery.DecodeStrictJSON[realtimeDelivery.CutoverAttemptInputs](payload)
	if err != nil {
		return fmt.Errorf("decode cutover attempt inputs: %w", err)
	}
	source, err := realtimeDelivery.ParseAuthority(sourceValue)
	if err != nil {
		return err
	}
	target, err := realtimeDelivery.ParseAuthority(targetValue)
	if err != nil {
		return err
	}
	workspace, err := realtimeDelivery.CreateCutoverAttemptWorkspace(
		directory, attemptID, source, target, maxInterruption, inputs, now().UTC(),
	)
	if err != nil {
		return err
	}
	return encodeOutput(output, statusOutput{
		AttemptID: workspace.Journal.Manifest.AttemptID, ManifestSHA: workspace.Journal.ManifestSHA256,
		State: workspace.Journal.Projection.State, HeadSHA: workspace.Journal.HeadSHA256,
	})
}

func realExecutorFactory(
	workspace *realtimeDelivery.CutoverAttemptWorkspace,
	operator string,
	leaseDuration time.Duration,
) (realtimeDelivery.CutoverAttemptActionExecutor, func(), error) {
	if err := config.Load(); err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	if err := store.InitRedis(); err != nil {
		return nil, nil, fmt.Errorf("initialize Redis: %w", err)
	}
	cleanup := func() { _ = store.RDB.Close() }
	realtimeConfig := config.RealtimeConfig()
	writer, err := realtimeDelivery.NewRedisAuthorityFenceWriter(
		store.RDB, realtimeConfig.FencingKey, realtimeConfig.FencingKey+":receipt:", 7*24*time.Hour, time.Now,
	)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	aggregator, err := realtimeDelivery.NewRedisFenceObservationAggregator(
		store.RDB, realtimeConfig.FencingKey+":observation:", time.Now,
	)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	kafkaConfig := config.KafkaConfig()
	if !kafkaConfig.Enabled {
		cleanup()
		return nil, nil, fmt.Errorf("Kafka must be enabled for cutover execution")
	}
	dialTimeout := time.Duration(kafkaConfig.DialTimeoutSeconds) * time.Second
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	clientID := strings.TrimSpace(kafkaConfig.ClientID)
	if clientID == "" {
		clientID = "dipole"
	}
	source, err := realtimeDelivery.NewKafkaGoCheckpointSource(kafkaConfig.Brokers, clientID+"-cutover", dialTimeout)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	collector, err := realtimeDelivery.NewDualGroupCheckpointCollector(source, time.Now)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	inputs := workspace.Inputs
	executor, err := realtimeDelivery.NewProductionCutoverAttemptExecutor(
		realtimeDelivery.ProductionCutoverExecutorConfig{
			Manifest: workspace.Journal.Manifest, InitialTransition: inputs.InitialTransition,
			SourceNodes: inputs.SourceNodes, FrozenNodes: inputs.FrozenNodes, TargetNodes: inputs.TargetNodes,
			CheckpointManifest: inputs.CheckpointManifest, OperatorID: operator,
			LeaseDuration: leaseDuration, Now: time.Now,
		},
		writer, aggregator, collector, workspace.Artifacts,
	)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return executor, cleanup, nil
}

func encodeOutput(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
