export interface Config {
  linearApiKey: string | undefined;
  linearTeamId: string | undefined;
  braindumpDb: string;
  printerDevice: string | undefined;
  port: number;
}

function parsePort(raw: string | undefined): number {
  if (raw === undefined) return 8080;
  const port = parseInt(raw, 10);
  if (Number.isNaN(port)) throw new Error(`Invalid PORT: "${raw}"`);
  return port;
}

export function loadConfig(): Config {
  return {
    linearApiKey: process.env.LINEAR_API_KEY,
    linearTeamId: process.env.LINEAR_TEAM_ID,
    braindumpDb: process.env.BRAINDUMP_DB ?? "./data/braindump.db",
    printerDevice: process.env.PRINTER_DEVICE,
    port: parsePort(process.env.PORT),
  };
}
