export interface KafkaFailureMessage {
  readonly topic: string;
  readonly key: Buffer | null;
  readonly value: Buffer | null;
  readonly headers: Readonly<Record<string, string>>;
}

export interface KafkaFailurePublisher {
  publish(topic: string, message: Omit<KafkaFailureMessage, "topic">): Promise<void>;
}

export interface ManagedKafkaFailurePublisher extends KafkaFailurePublisher {
  connect(): Promise<void>;
  disconnect(): Promise<void>;
}

export class PermanentKafkaEventError extends Error {
  constructor(readonly cause: unknown) {
    super(cause instanceof Error ? cause.message : String(cause));
    this.name = "PermanentKafkaEventError";
  }
}

export class KafkaFailureRouter {
  constructor(
    private readonly publisher: KafkaFailurePublisher,
    private readonly maxAttempts: number,
    private readonly now: () => Date = () => new Date()
  ) {
    if (!Number.isSafeInteger(maxAttempts) || maxAttempts < 1) {
      throw new Error("Kafka failure routing attempts must be a positive integer");
    }
  }

  async route(message: KafkaFailureMessage, error: unknown, permanent: boolean): Promise<void> {
    const baseTopic = message.topic.endsWith(".retry") ? message.topic.slice(0, -".retry".length) : message.topic;
    const attempt = retryAttempt(message.headers.retry_attempt);
    const headers: Record<string, string> = {
      ...message.headers,
      original_topic: baseTopic,
      last_error: errorText(error)
    };

    if (!permanent && attempt + 1 < this.maxAttempts) {
      headers.retry_attempt = String(attempt + 1);
      await this.publisher.publish(`${baseTopic}.retry`, { key: message.key, value: message.value, headers });
      return;
    }

    headers.retry_attempt = String(attempt);
    headers.dead_reason = permanent ? "invalid_envelope" : "handler_failed";
    headers.failed_at = this.now().toISOString();
    await this.publisher.publish(`${baseTopic}.dead`, { key: message.key, value: message.value, headers });
  }
}

function retryAttempt(value: string | undefined): number {
  if (value === undefined || !/^\d+$/.test(value)) {
    return 0;
  }
  const attempt = Number.parseInt(value, 10);
  return Number.isSafeInteger(attempt) ? attempt : 0;
}

function errorText(error: unknown): string {
  const value = error instanceof Error ? error.message : String(error);
  return value.slice(0, 65_535);
}
