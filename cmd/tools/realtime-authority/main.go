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
	action := flag.String("action", "", "transition action: bootstrap, freeze, activate, or renew")
	transitionID := flag.String("transition-id", "", "unique idempotency identifier")
	operatorID := flag.String("operator", "", "audited OS/operator identity label")
	reason := flag.String("reason", "", "bounded operational reason")
	expectedSHA := flag.String("expected-sha256", "", "exact current raw lease SHA-256")
	target := flag.String("target", "", "target authority for bootstrap/activate: go, shadow, or cpp")
	leaseUntilUnixMS := flag.Int64("lease-until-unix-ms", 0, "fixed absolute lease deadline, 5 seconds to 1 hour from first apply")
	confirm := flag.Bool("confirm", false, "confirm the authority transition")
	flag.Parse()

	if !*confirm {
		fatal(fmt.Errorf("-confirm is required"))
	}
	if *leaseUntilUnixMS <= 0 {
		fatal(fmt.Errorf("-lease-until-unix-ms is required"))
	}
	if err := config.Load(); err != nil {
		fatal(fmt.Errorf("load config: %w", err))
	}
	if err := cache.InitRedis(); err != nil {
		fatal(fmt.Errorf("initialize Redis: %w", err))
	}
	defer func() { _ = cache.RDB.Close() }()

	realtimeCfg := config.RealtimeConfig()
	writer, err := realtimeDelivery.NewRedisAuthorityFenceWriter(
		cache.RDB,
		realtimeCfg.FencingKey,
		realtimeCfg.FencingKey+":receipt:",
		7*24*time.Hour,
		time.Now,
	)
	if err != nil {
		fatal(err)
	}
	var targetAuthority realtimeDelivery.Authority
	if strings.TrimSpace(*target) != "" {
		targetAuthority, err = realtimeDelivery.ParseAuthority(*target)
		if err != nil {
			fatal(err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	receipt, err := writer.Apply(ctx, realtimeDelivery.FenceTransitionRequest{
		TransitionID:    *transitionID,
		Action:          realtimeDelivery.FenceTransitionAction(strings.ToLower(strings.TrimSpace(*action))),
		OperatorID:      *operatorID,
		Reason:          *reason,
		ExpectedSHA256:  *expectedSHA,
		TargetAuthority: targetAuthority,
		LeaseUntil:      time.UnixMilli(*leaseUntilUnixMS).UTC(),
	})
	if err != nil {
		fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(receipt); err != nil {
		fatal(fmt.Errorf("encode delivery authority transition receipt: %w", err))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
