import { z } from "zod";

import type { AgentCapability } from "./registry.js";

const inputSchema = z.object({ expression: z.string().trim().min(1).max(200) }).strict();
type CalculateInput = z.infer<typeof inputSchema>;

export class CalculateCapability implements AgentCapability<CalculateInput, unknown> {
  readonly descriptor = {
    id: "calculator.evaluate", risk: "read" as const, requiredPermission: "calculator.evaluate",
    inputSchema: { type: "object", properties: { expression: { type: "string", minLength: 1, maxLength: 200 } }, required: ["expression"], additionalProperties: false }
  };
  readonly inputSchema = inputSchema;

  resolveResource() { return { resourceType: "calculator", resourceId: "*", action: "evaluate" }; }

  async execute(input: CalculateInput): Promise<unknown> {
    return { expression: input.expression, value: evaluateExpression(input.expression) };
  }
}

export function evaluateExpression(expression: string): number {
  const parser = new ArithmeticParser(expression);
  const value = parser.expression();
  parser.expectEnd();
  if (!Number.isFinite(value) || Math.abs(value) > 1e15) throw new Error("Calculation result is outside the allowed range");
  return value;
}

class ArithmeticParser {
  #position = 0;
  #tokens = 0;

  constructor(private readonly source: string) {}

  expression(): number {
    let value = this.term();
    for (;;) {
      if (this.consume("+")) value += this.term();
      else if (this.consume("-")) value -= this.term();
      else return value;
      this.ensureFinite(value);
    }
  }

  private term(): number {
    let value = this.factor();
    for (;;) {
      if (this.consume("*")) value *= this.factor();
      else if (this.consume("/")) {
        const divisor = this.factor();
        if (divisor === 0) throw new Error("Division by zero is not allowed");
        value /= divisor;
      } else if (this.consume("%")) {
        const divisor = this.factor();
        if (divisor === 0) throw new Error("Division by zero is not allowed");
        value %= divisor;
      } else return value;
      this.ensureFinite(value);
    }
  }

  private factor(): number {
    if (this.consume("+")) return this.factor();
    if (this.consume("-")) return -this.factor();
    if (this.consume("(")) {
      const value = this.expression();
      if (!this.consume(")")) throw new Error("Unclosed parenthesis");
      return value;
    }
    this.skipWhitespace();
    const match = /^(?:\d+(?:\.\d*)?|\.\d+)/.exec(this.source.slice(this.#position));
    if (match === null) throw new Error("Invalid token in arithmetic expression");
    this.#position += match[0].length;
    this.#tokens++;
    if (this.#tokens > 64) throw new Error("Arithmetic expression has too many tokens");
    const value = Number(match[0]);
    if (!Number.isFinite(value)) throw new Error("Invalid number in arithmetic expression");
    return value;
  }

  expectEnd(): void {
    this.skipWhitespace();
    if (this.#position !== this.source.length) throw new Error("Invalid token in arithmetic expression");
  }

  private consume(token: string): boolean {
    this.skipWhitespace();
    if (!this.source.startsWith(token, this.#position)) return false;
    this.#position += token.length;
    this.#tokens++;
    if (this.#tokens > 64) throw new Error("Arithmetic expression has too many tokens");
    return true;
  }

  private skipWhitespace(): void {
    while (/\s/.test(this.source[this.#position] ?? "")) this.#position++;
  }

  private ensureFinite(value: number): void {
    if (!Number.isFinite(value)) throw new Error("Calculation result is outside the allowed range");
  }
}
