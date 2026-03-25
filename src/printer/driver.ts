// src/printer/driver.ts
//
// Printer interface and implementations for thermal receipt printing.
// Not tested -- minimal stubs ported from Go internal/printer/escpos.go.

import { writeFile } from "node:fs/promises";
import { accessSync, constants } from "node:fs";

/** Interface for printing raw bytes to a receipt printer. */
export interface Printer {
  print(data: Buffer): Promise<void>;
  available(): boolean;
}

/** No-op printer used when no physical printer is available. */
export class NullPrinter implements Printer {
  async print(_data: Buffer): Promise<void> {
    // no-op
  }
  available(): boolean {
    return false;
  }
}

/** Captures printed bytes in memory (for testing). */
export class BufferPrinter implements Printer {
  data: Buffer = Buffer.alloc(0);

  async print(data: Buffer): Promise<void> {
    this.data = Buffer.concat([this.data, data]);
  }
  available(): boolean {
    return true;
  }
}

/** Sends raw bytes to a thermal receipt printer via a device file. */
export class ESCPOSPrinter implements Printer {
  constructor(private devicePath: string) {}

  available(): boolean {
    try {
      accessSync(this.devicePath, constants.W_OK);
      return true;
    } catch {
      return false;
    }
  }

  async print(data: Buffer): Promise<void> {
    await writeFile(this.devicePath, data);
  }
}
