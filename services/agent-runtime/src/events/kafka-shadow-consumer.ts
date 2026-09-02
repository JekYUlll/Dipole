import { Kafka, Partitioners, type IHeaders, type Producer } from "kafkajs";

import { KafkaFailureRouter, PermanentKafkaEventError, type ManagedKafkaFailurePublisher } from "./kafka-failure-router.js";

export interface KafkaInboundPayload {
  readonly topic: string;
  readonly message: { readonly key: Buffer | null; readonly value: Buffer | null; readonly headers?: IHeaders };
}

export interface KafkaConsumerPort {
  connect(): Promise<void>;
  subscribe(config: { topic: string; fromBeginning: boolean }): Promise<void>;
  run(config: { eachMessage(payload: KafkaInboundPayload): Promise<void> }): Promise<void>;
  disconnect(): Promise<void>;
}

export interface KafkaConsumerFactoryPort {
  create(groupId: string): KafkaConsumerPort;
}

export interface KafkaShadowConsumerConfig {
  readonly groupId: string;
  readonly topic: string;
  readonly runtimeMode?: "shadow" | "active";
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

  create(groupId: string): KafkaConsumerPort {
    const consumer = this.#kafka.consumer({ groupId });
    return {
      connect: () => consumer.connect(),
      subscribe: (config) => consumer.subscribe(config),
      run: (config) => consumer.run({ eachMessage: async ({ topic, message }) => config.eachMessage({ topic, message }) }),
      disconnect: () => consumer.disconnect()
    };
  }

  createFailurePublisher(): ManagedKafkaFailurePublisher {
    return new KafkaJSFailurePublisher(this.#kafka.producer({ createPartitioner: Partitioners.DefaultPartitioner }));
  }

  async ensureTopics(topics: readonly string[], partitions: number, replicationFactor: number): Promise<void> {
    const admin = this.#kafka.admin();
    await admin.connect();
    try {
      const existing = new Set(await admin.listTopics());
      const missing = topics.filter((topic) => !existing.has(topic));
      if (missing.length > 0) {
        await admin.createTopics({
          waitForLeaders: true,
          topics: missing.map((topic) => ({ topic, numPartitions: partitions, replicationFactor }))
        });
      }

      const metadata = await admin.fetchTopicMetadata({ topics: [...topics] });
      for (const topic of metadata.topics) {
        if (topic.partitions.length !== partitions) {
          throw new Error(`Kafka topic ${topic.name} has ${topic.partitions.length} partitions; expected ${partitions}`);
        }
        if (topic.partitions.some((partition) => partition.replicas.length !== replicationFactor)) {
          throw new Error(`Kafka topic ${topic.name} does not use replication factor ${replicationFactor}`);
        }
      }
    } finally {
      await admin.disconnect();
    }
  }
}

class KafkaJSFailurePublisher implements ManagedKafkaFailurePublisher {
  constructor(private readonly producer: Producer) {}

  connect(): Promise<void> {
    return this.producer.connect();
  }

  disconnect(): Promise<void> {
    return this.producer.disconnect();
  }

  async publish(topic: string, message: { key: Buffer | null; value: Buffer | null; headers: Readonly<Record<string, string>> }): Promise<void> {
    await this.producer.send({ topic, messages: [{ key: message.key, value: message.value, headers: { ...message.headers } }] });
  }
}

export class KafkaShadowConsumer {
  #consumer: KafkaConsumerPort | undefined;
  readonly #groupId: string;

  constructor(
    private readonly factory: KafkaConsumerFactoryPort,
    private readonly config: KafkaShadowConsumerConfig,
    private readonly process: (raw: string) => Promise<void>,
    private readonly failureRouter?: KafkaFailureRouter
  ) {
    const groupId = config.groupId.trim();
    const runtimeMode = config.runtimeMode ?? "shadow";
    const requiredPrefix = `dipole-agent-${runtimeMode}-`;
    if (!groupId.startsWith(requiredPrefix)) {
      throw new Error(`Kafka ${runtimeMode} consumer requires an isolated ${requiredPrefix}* group`);
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
        if (this.failureRouter !== undefined) {
          await consumer.subscribe({ topic: `${this.config.topic.trim()}.retry`, fromBeginning: false });
        }
        await consumer.run({
          eachMessage: async ({ topic, message }) => {
            if (message.value === null) {
              const error = new PermanentKafkaEventError("empty Kafka message is not a valid Agent event");
              if (this.failureRouter === undefined) {
                throw error;
              }
              await this.failureRouter.route({
                topic, key: message.key, value: null, headers: decodeHeaders(message.headers)
              }, error, true);
              return;
            }
            try {
              await this.process(message.value.toString("utf8"));
            } catch (error) {
              if (this.failureRouter === undefined) {
                throw error;
              }
              await this.failureRouter.route({
                topic, key: message.key, value: message.value, headers: decodeHeaders(message.headers)
              }, error, error instanceof PermanentKafkaEventError);
            }
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

function decodeHeaders(headers: IHeaders | undefined): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [key, raw] of Object.entries(headers ?? {})) {
    const value = Array.isArray(raw) ? raw.at(-1) : raw;
    if (value !== undefined) {
      result[key] = Buffer.isBuffer(value) ? value.toString("utf8") : String(value);
    }
  }
  return result;
}
