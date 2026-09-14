import { z } from "zod";
import type { AgentCapability } from "./registry.js";

const inputSchema = z.object({
  city: z.string().trim().min(2).max(100),
  countryCode: z.string().regex(/^[A-Za-z]{2}$/).optional()
}).strict();
type WeatherInput = z.infer<typeof inputSchema>;
const locationsSchema = z.object({ results: z.array(z.object({
  name: z.string(), country: z.string().optional(), admin1: z.string().optional(),
  latitude: z.number().min(-90).max(90), longitude: z.number().min(-180).max(180)
})).optional() });
const weatherSchema = z.object({
  timezone: z.string(),
  current: z.object({ time: z.string(), temperature_2m: z.number(), apparent_temperature: z.number(),
    relative_humidity_2m: z.number(), precipitation: z.number(), weather_code: z.number().int(), wind_speed_10m: z.number() })
});

export class GetWeatherCapability implements AgentCapability<WeatherInput, unknown> {
  readonly descriptor = {
    id: "get_weather", risk: "read" as const, requiredPermission: "weather.read",
    inputSchema: { type: "object", properties: {
      city: { type: "string", minLength: 2, maxLength: 100 },
      countryCode: { type: "string", minLength: 2, maxLength: 2 }
    }, required: ["city"], additionalProperties: false }
  };
  readonly inputSchema = inputSchema;
  constructor(private readonly fetcher: typeof fetch = fetch) {}
  resolveResource() { return { resourceType: "weather", resourceId: "*", action: "read" }; }

  async execute(raw: WeatherInput): Promise<unknown> {
    const input = inputSchema.parse(raw);
    const signal = AbortSignal.timeout(8000);
    const geocoding = new URL("https://geocoding-api.open-meteo.com/v1/search");
    geocoding.search = new URLSearchParams({ name: input.city, count: "1", language: "zh", format: "json",
      ...(input.countryCode === undefined ? {} : { countryCode: input.countryCode.toUpperCase() }) }).toString();
    const location = locationsSchema.parse(await this.getJSON(geocoding, signal)).results?.[0];
    if (location === undefined) return { found: false, city: input.city, reason: "City not found; specify a country or a more precise name" };
    const endpoint = new URL("https://api.open-meteo.com/v1/forecast");
    endpoint.search = new URLSearchParams({ latitude: String(location.latitude), longitude: String(location.longitude),
      current: "temperature_2m,apparent_temperature,relative_humidity_2m,precipitation,weather_code,wind_speed_10m",
      timezone: "auto", temperature_unit: "celsius", wind_speed_unit: "kmh", precipitation_unit: "mm" }).toString();
    const weather = weatherSchema.parse(await this.getJSON(endpoint, signal));
    return { found: true, location, timezone: weather.timezone, current: weather.current,
      units: { temperature: "C", humidity: "%", precipitation: "mm", windSpeed: "km/h", weatherCode: "WMO" },
      source: "Open-Meteo", sourceUrl: "https://open-meteo.com/", trust: "untrusted" };
  }

  private async getJSON(url: URL, signal: AbortSignal): Promise<unknown> {
    const response = await this.fetcher(url, { signal, redirect: "error" });
    if (!response.ok) throw new Error(`Weather provider returned HTTP ${response.status}`);
    if (response.body === null) throw new Error("Weather provider returned an empty body");
    const reader = response.body.getReader();
    const chunks: Uint8Array[] = [];
    let bytes = 0;
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        bytes += value.byteLength;
        if (bytes > 64 * 1024) throw new Error("Weather provider response is too large");
        chunks.push(value);
      }
      return JSON.parse(Buffer.concat(chunks).toString("utf8"));
    } finally { await reader.cancel(); }
  }
}
