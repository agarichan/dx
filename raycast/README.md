# dx dev services (Raycast)

`dx status --all --json` を叩いて起動中サービスを一覧し、公開 URL をブラウザで開く / `dx stop <name>` で停止する Raycast extension。

## インストール

**配布ユーザー（リポジトリ不要）**:
```
dx raycast install      # バイナリ同梱ソースを展開して Raycast に取り込み
```

**このリポジトリで拡張を開発する場合**（Raycast がこの checkout を直接指す）:
```
mise run raycast:install    # 依存→ビルド→自動終了（Ctrl+C 不要）で Raycast に取込み
mise run raycast:dev        # コード編集のライブ更新（ray develop 常駐・Ctrl+C で停止）
```
注意: 両方を実行すると Raycast に同名拡張が2エントリ並ぶ。開発者は mise 側だけを使う。

一度取り込めば Raycast に残り、コマンドを開くたび `dx status --all --json` をライブ実行する。

`dx` のパスは Raycast の extension preferences（既定 `~/.local/bin/dx`）で変更可。

## アイコン
`assets/icon.svg` がソース。編集したら再生成:
```
mise run raycast:icon       # qlmanage で assets/extension-icon.png を出力
```
