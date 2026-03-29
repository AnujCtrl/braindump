// src/printer/config.ts
//
// Printer configuration loader.
// Not tested -- minimal stub ported from Go internal/printer/config.go.

import { readFileSync } from "node:fs";

const RECEIPT_WIDTH = 32;

export interface PrinterConfig {
  enabled: boolean;
  devicePath: string;
  mode: "text" | "escpos";
  encoderScript: string;
  width: number;
}

export function defaultConfig(): PrinterConfig {
  return {
    enabled: true,
    devicePath: "/dev/usb/lp0",
    mode: "text",
    encoderScript: "scripts/receipt-encoder/encode.js",
    width: RECEIPT_WIDTH,
  };
}

export function loadConfig(path: string): PrinterConfig {
  const def = defaultConfig();
  try {
    const raw = readFileSync(path, "utf-8");
    if (!raw.trim()) return def;
    const parsed = JSON.parse(raw) as Partial<PrinterConfig>;
    return {
      enabled: parsed.enabled ?? def.enabled,
      devicePath: parsed.devicePath ?? def.devicePath,
      mode: parsed.mode ?? def.mode,
      encoderScript: parsed.encoderScript ?? def.encoderScript,
      width: parsed.width ?? def.width,
    };
  } catch {
    return def;
  }
}
