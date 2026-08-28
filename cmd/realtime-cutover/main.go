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
) (cutoverRuntime, error)

type cutoverRuntime struct {
	executor  realtimeDelivery.CutoverAttemptActionExecutor
	ownership realtimeDelivery.CutoverControllerOwnership
	cleanup   func()
}

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
	operation := flags.String("operation", "status", "create, status, advance, renew, rollback, or run")
	directory := flags.String("attempt-dir", "", "durable cutover attempt workspace directory")
	inputsPath := flags.String("inputs", "", "strict attempt inputs JSON for create")
	attemptID := flags.String("attempt-id", "", "bounded attempt identity for create")
	source := flags.String("source", "", "source authority for create")
	target := flags.String("target", "", "target authority for create")
	maxInterruption := flags.Duration("max-interruption", time.Minute, "maximum no-authority window")
	operator := flags.String("operator", "", "audited operator label for advance/rollback")
	leaseDuration := flags.Duration("lease-duration", 10*time.Minute, "authority lease duration for transitions")
	controllerID := flags.String("controller-id", "", "stable controller process identity for run")
	controlLease := flags.Duration("control-lease", 2*time.Minute, "exclusive controller ownership lease for run")
	actionTimeout := flags.Duration("action-timeout", 30*time.Second, "single external action timeout for run")
	renewBefore := flags.Duration("renew-before", time.Minute, "authority lease renewal margin after a blocked action")
	retryInterval := flags.Duration("retry-interval", time.Second, "blocked action retry interval for run")
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
	if *operation != "advance" && *operation != "renew" && *operation != "rollback" && *operation != "run" {
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
	runtime, err := factory(workspace, *operator, *leaseDuration)
	if err != nil {
		return err
	}
	if runtime.executor == nil || runtime.cleanup == nil {
		return fmt.Errorf("cutover executor runtime is invalid")
	}
	defer runtime.cleanup()
	orchestrator, err := realtimeDelivery.NewCutoverAttemptOrchestrator(workspace.Journal, runtime.executor, now)
	if err != nil {
		return err
	}
	if *operation == "run" {
		if strings.TrimSpace(*controllerID) == "" {
			return fmt.Errorf("-controller-id is required for run")
		}
		if runtime.ownership == nil {
			return fmt.Errorf("cutover controller ownership is unavailable")
		}
		authorityLease, err := realtimeDelivery.NewCutoverWorkspaceAuthorityLease(workspace)
		if err != nil {
			return err
		}
		controller, err := realtimeDelivery.NewCutoverAttemptController(realtimeDelivery.CutoverAttemptControllerConfig{
			Orchestrator: orchestrator, Ownership: runtime.ownership, AuthorityLease: authorityLease,
			OwnerID: *controllerID, OwnershipTTL: *controlLease, ActionTimeout: *actionTimeout,
			RenewBefore: *renewBefore, RetryInterval: *retryInterval, Now: now,
		})
		if err != nil {
			return err
		}
		result, err := controller.Run(ctx)
		if err != nil {
			return err
		}
		return encodeOutput(output, result)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result realtimeDelivery.CutoverAttemptAdvance
	if *operation == "advance" {
		result, err = orchestrator.Advance(ctx)
	} else if *operation == "renew" {
		result, err = orchestrator.RenewLease(ctx)
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
) (cutoverRuntime, error) {
	if err := config.Load(); err != nil {
		return cutoverRuntime{}, fmt.Errorf("load config: %w", err)
	}
	if err := store.InitRedis(); err != nil {
		return cutoverRuntime{}, fmt.Errorf("initialize Redis: %w", err)
	}
	cleanup := func() { _ = store.RDB.Close() }
	realtimeConfig := config.RealtimeConfig()
	writer, err := realtimeDelivery.NewRedisAuthorityFenceWriter(
		store.RDB, realtimeConfig.FencingKey, realtimeConfig.FencingKey+":receipt:", 7*24*time.Hour, time.Now,
	)
	if err != nil {
		cleanup()
		return cutoverRuntime{}, err
	}
	aggregator, err := realtimeDelivery.NewRedisFenceObservationAggregator(
		store.RDB, realtimeConfig.FencingKey+":observation:", time.Now,
	)
	if err != nil {
		cleanup()
		return cutoverRuntime{}, err
	}
	kafkaConfig := config.KafkaConfig()
	if !kafkaConfig.Enabled {
		cleanup()
		return cutoverRuntime{}, fmt.Errorf("Kafka must be enabled for cutover execution")
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
		return cutoverRuntime{}, err
	}
	collector, err := realtimeDelivery.NewDualGroupCheckpointCollector(source, time.Now)
	if err != nil {
		cleanup()
		return cutoverRuntime{}, err
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
		return cutoverRuntime{}, err
	}
	ownership, err := realtimeDelivery.NewRedisCutoverControllerOwnership(
		store.RDB, realtimeConfig.FencingKey+":controller:"+workspace.Journal.Manifest.AttemptID,
	)
	if err != nil {
		cleanup()
		return cutoverRuntime{}, err
	}
	return cutoverRuntime{executor: executor, ownership: ownership, cleanup: cleanup}, nil
}

func encodeOutput(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
