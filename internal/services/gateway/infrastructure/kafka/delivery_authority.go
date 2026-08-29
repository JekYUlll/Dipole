package kafka

import (
	"context"
	"fmt"
	"time"

	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	realtimeDelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
)

// FenceMessageDeliveryHandler waits for the Gateway delivery authority before
// allowing a Kafka record to produce a client-side effect.
func FenceMessageDeliveryHandler(
	authority realtimeDelivery.Authority,
	fence realtimeDelivery.AuthorityFence,
	next platformKafka.Handler,
) platformKafka.Handler {
	if fence == nil {
		return next
	}
	return func(ctx context.Context, event platformKafka.Event) error {
		for {
			if err := fence.Assert(ctx, authority); err == nil {
				return next(ctx, event)
			}
			timer := time.NewTimer(250 * time.Millisecond)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return fmt.Errorf("wait for realtime delivery authority fence: %w", ctx.Err())
			case <-timer.C:
			}
		}
	}
}
