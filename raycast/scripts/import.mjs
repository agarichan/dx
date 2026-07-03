// `ray develop` を起動し、初回ビルド（=Raycast への取り込み）が終わったら自動で停止する。
// ray develop は本来ホットリロード用に常駐するが、取り込みだけしたい場合は Ctrl+C 不要にしたい。
// 取り込んだ拡張は ray 停止後も Raycast に残る。コード編集時のライブ更新は `raycast:watch`。
import { spawn } from "node:child_process";

const ray = new URL("../node_modules/.bin/ray", import.meta.url).pathname;
const child = spawn(ray, ["develop", "-I"], { stdio: ["ignore", "pipe", "pipe"] });

let done = false;
function finish(code) {
  if (done) return;
  done = true;
  child.kill("SIGINT");
  setTimeout(() => process.exit(code), 400);
}

function onData(buf) {
  const s = buf.toString();
  process.stdout.write(s);
  if (/built extension successfully/i.test(s)) {
    console.log("\n✓ Raycast に取り込みました。「List dx Services」 が使えます（watcher は停止済み）。");
    finish(0);
  }
}
child.stdout.on("data", onData);
child.stderr.on("data", onData);
child.on("exit", (c) => finish(c ?? 0));

// 安全装置: マーカーが出ないまま 120s 経ったら諦めて停止
setTimeout(() => {
  console.error("timeout: ビルド完了マーカーを検知できませんでした。停止します。");
  finish(1);
}, 120000);
