import { Kafka, type Consumer } from "kafkajs";

export interface KafkaConsumerPort {
  connect(): Promise<void>;
  subscribe(config: { topic: string; fromBeginning: boolean }): Promise<void>;
  run(config: { eachMessage(payload: { message: { value: Buffer | null } }): Promise<void> }): Promise<void>;
  disconnect(): Promise<void>;
}

export interface KafkaConsumerFactoryPort {
  create(groupId: string): KafkaConsumerPort;
}

export interface KafkaShadowConsumerConfig {
  readonly groupId: string;
  readonly topic: string;
  readonly startupAttempts?: number;
  readonly startupRetryDelayMs?: number;
}

export class KafkaJSConsumerFactory implements KafkaConsumerFactoryPort {
  readonly #kafka: Kafka;

  constructor(clientId: string, brokers: readonly string[]) {
    if (!clientId.trim() || brokers.length === 0) {
      throw new Error("Kafka client ID and brokers are required");
    }
    this.#kafka = new Kafka({ clientId: clientId.trim(), brokers: [...brokers] });
  }

  create(groupId: string): Consumer {
    return this.#kafka.consumer({ groupId });
  }
}

export class KafkaShadowConsumer {
  #consumer: KafkaConsumerPort | undefined;
  readonly #groupId: string;

  constructor(private readonly factory: KafkaConsumerFactoryPort, private readonly config: KafkaShadowConsumerConfig, private readonly process: (raw: string) => Promise<void>) {
    const groupId = config.groupId.trim();
    if (!groupId.startsWith("dipole-agent-shadow-")) {
      throw new Error("Kafka shadow consumer requires an isolated dipole-agent-shadow-* group");
    }
    if (!config.topic.trim()) {
      throw new Error("Kafka shadow consumer topic is required");
    }
    if ((config.startupAttempts ?? 5) < 1 || (config.startupRetryDelayMs ?? 1000) < 0) {
      throw new Error("Kafka shadow consumer retry configuration is invalid");
    }
    this.#groupId = groupId;
  }

  async start(): Promise<void> {
    const attempts = this.config.startupAttempts ?? 5;
    for (let attempt = 1; attempt <= attempts; attempt += 1) {
      const consumer = this.factory.create(this.#groupId);
      try {
        await consumer.connect();
        await consumer.subscribe({ topic: this.config.topic.trim(), fromBeginning: false });
        await consumer.run({
          eachMessage: async ({ message }) => {
            if (message.value === null) {
              throw new Error("empty Kafka message is not a valid Agent event");
            }
            await this.process(message.value.toString("utf8"));
          }
        });
        this.#consumer = consumer;
        return;
      } catch (error) {
        await consumer.disconnect().catch(() => undefined);
        if (attempt === attempts) {
          throw error;
        }
        await new Promise((resolve) => setTimeout(resolve, this.config.startupRetryDelayMs ?? 1000));
      }
    }
  }

  async stop(): Promise<void> {
    const consumer = this.#consumer;
    this.#consumer = undefined;
    if (consumer !== undefined) {
      await consumer.disconnect();
    }
  }
}
