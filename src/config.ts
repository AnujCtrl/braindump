export interface Config {
  linearApiKey: string | undefined;
  linearTeamId: string | undefined;
  braindumpDb: string;
  printerDevice: string | undefined;
  port: number;
}

export function loadConfig(): Config {
  return {
    linearApiKey: process.env.LINEAR_API_KEY,
    linearTeamId: process.env.LINEAR_TEAM_ID,
    braindumpDb: process.env.BRAINDUMP_DB ?? "./data/braindump.db",
    printerDevice: process.env.PRINTER_DEVICE,
    port: process.env.PORT !== undefined ? parseInt(process.env.PORT, 10) : 8080,
  };
}
