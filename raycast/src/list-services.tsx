import { useEffect, useState } from "react";
import { ActionPanel, Action, List, Icon, Color, showToast, Toast } from "@raycast/api";
import { listServices, stopService, Service } from "./dx";

export default function Command() {
  const [items, setItems] = useState<Service[]>([]);
  const [loading, setLoading] = useState(true);

  async function refresh() {
    setLoading(true);
    try {
      setItems(await listServices());
    } catch (e) {
      await showToast({ style: Toast.Style.Failure, title: "dx status failed", message: String(e) });
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function onStop(svc: Service) {
    await showToast({ style: Toast.Style.Animated, title: `Stopping ${svc.name}…` });
    try {
      await stopService(svc.name);
      await showToast({ style: Toast.Style.Success, title: `Stopped ${svc.name}` });
      await refresh();
    } catch (e) {
      await showToast({ style: Toast.Style.Failure, title: "Stop failed", message: String(e) });
    }
  }

  return (
    <List isLoading={loading}>
      <List.EmptyView title="No dx services" description="起動中の dx サービスはありません" />
      {items.map((svc) => (
        <List.Item
          key={svc.name}
          icon={{
            source: svc.state === "running" ? Icon.CircleFilled : Icon.Circle,
            tintColor: svc.state === "running" ? Color.Green : Color.SecondaryText,
          }}
          title={svc.name}
          subtitle={svc.root}
          accessories={[{ tag: svc.state }, { text: `pid ${svc.pid}` }]}
          actions={
            <ActionPanel>
              {svc.url ? <Action.OpenInBrowser url={svc.url} /> : null}
              <Action
                title="Stop Service"
                icon={Icon.Stop}
                style={Action.Style.Destructive}
                shortcut={{ modifiers: ["cmd"], key: "x" }}
                onAction={() => onStop(svc)}
              />
              {svc.url ? <Action.CopyToClipboard title="Copy URL" content={svc.url} /> : null}
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
