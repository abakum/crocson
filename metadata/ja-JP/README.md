# crocson

[howeyc/crocgui](https://github.com/howeyc/crocgui/releases/tag/v1.11.5) のフォーク — [croc](https://github.com/schollz/croc) をベースに、Android、Windows、Linux、macOS向けに設計。

<p align="center">
  <img src="images/phoneScreenshots/1.png?raw=true" width="200">
  <img src="images/phoneScreenshots/2.png?raw=true" width="200">
  <img src="images/phoneScreenshots/4.png?raw=true" width="200">
  <br>
</p>

<p align="center">
  <img src="../../Icon.png?raw=true" width="100"><br>
  <i>croc</i> の作者 <a href="https://github.com/schollz">Zack Scholl</a> は、<i>croc</i> という名前を
  ワニのタクシー運転手が動物たちを安全に運ぶという比喩として選びました。<BR>
  <i>crocson</i> はそのワニのタクシー運転手の後継者であり、安全なオンライン通信を可能にします。
</p>

## ファイル転送
`F-Droid: File Transfer` `FreeDesktop: Network;FileTransfer` `MS Store: Productivity` `Google Play: Communication;Tools`

- リレー経由で2台のコンピュータ間でファイルやフォルダを転送
- 複数のファイルやフォルダを順次転送
- クロスプラットフォーム: Windows、Linux、macOS、Android
- ローカルサーバーやポート転送不要
- IPv6優先、自動IPv4フォールバック
- セルフホストリレー（`crocson relay`）
- カスタムリレーポート
- ファイル・フォルダ選択ダイアログ
- ドラッグ＆ドロップ（Windows、Linux、macOS）
- コマンドライン: 引数およびstdinパイプ対応（`cat file | crocson`）
- Android: ファイルマネージャーの「共有」メニューから単一または複数ファイルを送信
- Android: 「開く」対応
- クリップボード経由でテキスト/URLを送信
- Android: ネストされたフォルダなしで送信、ネストされたフォルダを受信して.zipとして保存
- 全体およびファイルごとの進捗バー
- 転送キャンセルボタン
- 送信 ↔ 受信 切り替え

## WebDAV
`F-Droid: Connectivity;Cloud Storage & File Sync` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Productivity`

- 内蔵WebDAVサーバー（HTTP/HTTPS）
- 自己署名TLS証明書（ローカルIPに基づく決定論的生成）
- Webブラウザでファイルの閲覧、アップロード（ダイアログまたはドラッグ＆ドロップ）、削除（ディレクトリ一覧）
- GUI内のWebDAVファイルツリー
- ホスト/ポートの選択
- WebDAV経由の音声/動画ストリーミング再生
- 暗号化トンネル経由でWebDAVを転送（パブリックIPなしでリモートアクセス可能）:
  - リレー [croc](https://github.com/schollz/croc/pull/1113) [fork](https://github.com/abakCroc/croc/v10)
  - リレー [magic-wormhole](https://github.com/magic-wormhole/magic-wormhole) — スキーム `ws:` [fork](https://github.com/abakum/wormhole-william)
  - リレー [webwormhole](https://github.com/saljam/webwormhole) — スキーム `https:` [fork](https://github.com/abakum/webwormhole)


## ビデオ通話
`F-Droid: Voice & Video Chat` `FreeDesktop: Network;VideoConference` `MS Store: Social` `Google Play: Communication`

- P2Pビデオ通話（内蔵HTMLページ）
- 画面共有: ブラウザタブ、アプリケーションウィンドウ、またはデスクトップ全体
- ビデオ通話ルーム（作成/参加/待機/終了）
- WebSocketによるリアルタイム動画+音声配信
- 2人目の参加者が接続する前のミラープレビュー
- サーバー側でのビデオ通話録画（WebM/MP4）
- コーデックと解像度の選択

## チャット
`F-Droid: Messaging` `FreeDesktop: Network;Chat` `MS Store: Social` `Google Play: Communication`

- 内蔵テキストチャット
- メッセージ履歴
- メッセージ受信時にチャットを自動で開く
- チャットにスクリーンショットと動画録画を送信

## 動画/音声録画
`F-Droid: Multimedia` `FreeDesktop: AudioVideo;Recorder` `MS Store: Photo + video` `Google Play: Video Players & Editors`

- ブラウザ経由でウェブカメラ/マイクで動画+音声を録画
- タイムスタンプ付き録画（YYYYMMDD_HHMMSS_mmm）
- 録画をチャットに公開

## ウェブカメラ
`F-Droid: Multimedia` `FreeDesktop: AudioVideo` `MS Store: Photo + video` `Google Play: Photography`

- ブラウザ経由のウェブカメラキャプチャ
- ビデオ通話からのスクリーンショット

## セキュリティ
`F-Droid: Security` `FreeDesktop: Security` `MS Store: Security` `Google Play: Tools`

- PAKEベースのエンドツーエンド暗号化
- ワンタイムパスワード（TOTP）とタイマー
- 入力フィールドまたはCROC_SECRET環境変数からシークレットを設定
- 暗号化用の楕円曲線選択
- ハッシュ: imohash、md5、xxhash、highway
- リレーパスワード
- リレー経由の暗号化トンネル

## QRコード & ディープリンク
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- コードフレーズ付きQRコード生成
- カメラ経由のQRコードスキャン
- 複数のAndroidスキャナーアプリ対応（Xiaomi、Samsung、OPlus、BinaryEye、Lens、ZXing、Chrome、Via、Samsung Browser、Opera mini、Microsoft、Firefox）
- ディープリンク: `https://abakum.github.io/croc#...`（Base64エンコードされた設定）
- ディープリンク: `davX:` / `webdavX:` でWebDAVを開く

## リレープロファイル
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- リレープロファイルの一覧（保存/読み込み）
- リレー間の切り替え
- GUI管理付きローカルリレー
- カスタムアドレス、IPv6、ポート、パスワード
- プロキシ対応: SOCKS5（Tor含む）、HTTP

## 転送設定
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- ローカル転送の無効化（リレーのみ）
- ローカル送信者のみに接続
- .gitignore対応
- ファイルの上書き
- 圧縮のオン/オフ
- 多重化のオン/オフ
- 送信時にフォルダをZIP化
- アップロード速度制限

## インターフェース
`F-Droid: Personalization` `FreeDesktop: Settings` `MS Store: Personalization` `Google Play: Personalization`

- テーマ: システム、ライト、グレー、ダーク、ブラック
- フォント選択（埋め込み + システムフォント）
- 多言語対応（en-US、tr-TR、ja-JP、zh-CN、ru-RU）
- ロゴ非表示
- カラー付き転送ログ
- Android: `adb logcat -s croc` でログ出力

## CLIモード

ファイル以外のパラメータが1つ以上渡された場合、crocsonはcrocのCLIとして動作します:
- 中断された転送の再開（レジューム）
- パイプ（stdin/stdout）: `cat file | crocson send`
- テキスト送信: `crocson send --text "hello"`
- モバイルデバイス用QRコード
- スクリプト用のクワイエットモード
- コードをクリップボードにコピー
- フォルダの除外（`--exclude`）
- プロセスセキュリティのためのCROC_SECRET環境変数

---

## アプリストア別カテゴリ一覧

| Feature | F-Droid | FreeDesktop | MS Store | Google Play |
|---|---|---|---|---|
| File Transfer | **File Transfer** | Network;FileTransfer | Productivity | Communication |
| WebDAV | **Connectivity**; Cloud Storage & File Sync | Network | Productivity | Productivity |
| Video Calls | **Voice & Video Chat** | Network;VideoConference | Social | Communication |
| Chat | **Messaging** | Network;Chat | Social | Communication |
| Video/Audio Recording | **Multimedia** | AudioVideo;Recorder | Photo + video | Video Players & Editors |
| Webcam | **Multimedia** | AudioVideo | Photo + video | Photography |
| Security | **Security** | Security | Security | Tools |
| QR Code / Deep Links | **Connectivity** | Network | Productivity | Tools |
| Relay Profiles | **Connectivity** | Network | Productivity | Tools |
| Transfer Settings | **Connectivity** | Network | Productivity | Tools |
| Interface | **Personalization** | Settings | Personalization | Personalization |
