import { useEffect, useState } from "react";
import { ActionPanel, Action, List, Icon, Color, showToast, Toast } from "@raycast/api";
import { homedir } from "node:os";
import { listServices, stopService, groupByRoot, Env, Service } from "./dx";

const stateColor: Record<Env["state"], Color> = {
  running: Color.Green,
  partial: Color.Yellow,
  stopped: Color.SecondaryText,
};

function tilde(p: string): string {
  const home = homedir();
  return p.startsWith(home) ? "~" + p.slice(home.length) : p;
}

// One dot per service: green when running, label = dx.toml key (fallback: name).
function dots(env: Env): List.Item.Accessory[] {
  return env.services.map((svc) => ({
    icon: {
      source: svc.state === "running" ? Icon.CircleFilled : Icon.Circle,
      tintColor: svc.state === "running" ? Color.Green : Color.SecondaryText,
    },
    text: svc.key || svc.name,
    tooltip: `${svc.name}: ${svc.state} (pid ${svc.pid})`,
  }));
}

export default function Command() {
  const [envs, setEnvs] = useState<Env[]>([]);
  const [loading, setLoading] = useState(true);

  async function refresh() {
    setLoading(true);
    try {
      setEnvs(groupByRoot(await listServices()));
    } catch (e) {
      await showToast({ style: Toast.Style.Failure, title: "dx status failed", message: String(e) });
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function stopMany(services: Service[], label: string) {
    await showToast({ style: Toast.Style.Animated, title: `Stopping ${label}…` });
    try {
      for (const svc of services) {
        if (svc.state === "running") await stopService(svc.name);
      }
      await showToast({ style: Toast.Style.Success, title: `Stopped ${label}` });
      await refresh();
    } catch (e) {
      await showToast({ style: Toast.Style.Failure, title: "Stop failed", message: String(e) });
    }
  }

  return (
    <List isLoading={loading}>
      <List.EmptyView title="No dx services" description="起動中の dx サービスはありません" />
      {envs.map((env) => (
        <List.Item
          key={env.root}
          icon={{ source: Icon.CircleFilled, tintColor: stateColor[env.state] }}
          title={env.openSvc.name}
          subtitle={tilde(env.root)}
          accessories={dots(env)}
          actions={
            <ActionPanel>
              {env.openSvc.url ? <Action.OpenInBrowser url={env.openSvc.url} /> : null}
              {env.openSvc.url ? <Action.CopyToClipboard title="Copy URL" content={env.openSvc.url} /> : null}
              <Action
                title="Stop All in Env"
                icon={Icon.Stop}
                style={Action.Style.Destructive}
                shortcut={{ modifiers: ["cmd"], key: "x" }}
                onAction={() => stopMany(env.services, env.openSvc.name)}
              />
              <ActionPanel.Submenu title="Stop Service…" icon={Icon.StopFilled}>
                {env.services.map((svc) => (
                  <Action
                    key={svc.name}
                    title={`Stop ${svc.key || svc.name}`}
                    style={Action.Style.Destructive}
                    onAction={() => stopMany([svc], svc.name)}
                  />
                ))}
              </ActionPanel.Submenu>
              <Action
                title="Refresh"
                icon={Icon.ArrowClockwise}
                shortcut={{ modifiers: ["cmd"], key: "r" }}
                onAction={refresh}
              />
            </ActionPanel>
          }
        />
      ))}
    </List>
  );
}
