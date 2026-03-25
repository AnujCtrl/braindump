import { defineConfig } from "tsup";

export default defineConfig({
  entry: {
    "cli/index": "src/cli/index.ts",
    "server/index": "src/server/index.ts",
  },
  format: ["esm"],
  target: "node20",
  dts: false,
  clean: true,
});
