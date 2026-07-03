import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { homedir } from "node:os";
import { getPreferenceValues } from "@raycast/api";
import { Service } from "./group";

export { groupByRoot } from "./group";
export type { Service, Env, EnvState } from "./group";

const pExecFile = promisify(execFile);

function dxBin(): string {
  const { dxPath } = getPreferenceValues<{ dxPath?: string }>();
  const raw = (dxPath && dxPath.trim()) || "~/.local/bin/dx";
  return raw.startsWith("~") ? raw.replace(/^~/, homedir()) : raw;
}

export async function listServices(): Promise<Service[]> {
  const { stdout } = await pExecFile(dxBin(), ["status", "--all", "--json"]);
  const parsed = JSON.parse(stdout || "[]");
  return Array.isArray(parsed) ? (parsed as Service[]) : [];
}

export async function stopService(name: string): Promise<void> {
  await pExecFile(dxBin(), ["stop", name]);
}
