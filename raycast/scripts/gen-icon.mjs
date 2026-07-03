// assets/icon.svg を 512x512 PNG (assets/extension-icon.png) にラスタライズする。
// @resvg/resvg-js を使う（alpha を正しく保持。qlmanage は透明を白で潰すため不可）。
// アイコンは full-bleed（透明なし）で、角丸は Raycast 側のマスクに任せる。
import { Resvg } from "@resvg/resvg-js";
import { readFileSync, writeFileSync } from "node:fs";

const svg = readFileSync(new URL("../assets/icon.svg", import.meta.url));
const resvg = new Resvg(svg, {
  fitTo: { mode: "width", value: 512 },
  background: "rgba(0,0,0,0)", // 透明背景
});
const png = resvg.render().asPng();
writeFileSync(new URL("../assets/extension-icon.png", import.meta.url), png);
console.log("wrote assets/extension-icon.png", png.length, "bytes (full-bleed, via resvg)");
