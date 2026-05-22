# crocson

基于 [howeyc/crocgui v1.11.5](https://github.com/howeyc/crocgui/releases/tag/v1.11.5) 的分支 — [croc](https://github.com/schollz/croc) 的图形界面，支持 Windows、Linux、macOS 和 Android。

<p align="center">
  <img src="images/phoneScreenshots/1.png?raw=true" width="200">
  <img src="images/phoneScreenshots/2.png?raw=true" width="200">
  <img src="images/phoneScreenshots/4.png?raw=true" width="200">
  <br>
</p>

<p align="center">
  <img src="../../Icon.png?raw=true" width="100"><br>
  <i>croc</i> 的作者 <a href="https://github.com/schollz">Zack Scholl</a> 选择了 <i>croc</i> 这个名字，
  比喻鳄鱼出租车司机安全地运送动物。<BR>
  <i>crocson</i> 是鳄鱼出租车司机的继承者，实现安全的在线通信。
</p>

## 文件传输
`F-Droid: File Transfer` `FreeDesktop: Network;FileTransfer` `MS Store: Productivity` `Google Play: Communication;Tools`

- 通过中继在任意两台电脑之间传输文件和文件夹
- 传输多个文件和文件夹（顺序传输）
- 跨平台：Windows、Linux、macOS、Android
- 无需本地服务器或端口转发
- IPv6 优先，自动回退 IPv4
- 自建中继（`crocson relay`）
- 自定义中继端口
- 文件和文件夹选择对话框
- 拖放操作（Windows、Linux、macOS）
- 命令行：参数和标准输入管道（`cat file | crocson`）
- Android：从文件管理器的"分享"菜单发送一个或多个文件
- Android："打开方式"
- 通过剪贴板发送文本/URL
- Android：发送时不含嵌套文件夹，接收嵌套文件夹 — 保存为 .zip
- 总体和单文件进度条
- 取消传输按钮
- 发送 ↔ 接收切换

## WebDAV
`F-Droid: Connectivity;Cloud Storage & File Sync` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Productivity`

- 内置 WebDAV 服务器（HTTP/HTTPS）
- 自签名 TLS 证书（确定性，基于本地 IP）
- 通过浏览器浏览文件（目录列表）
- GUI 中的 WebDAV 文件树
- 主机/端口选择
- 通过 WebDAV 流式播放音视频
- 通过加密隧道转发 WebDAV（无需公网 IP 即可远程访问）

## 视频通话
`F-Droid: Voice & Video Chat` `FreeDesktop: Network;VideoConference` `MS Store: Social` `Google Play: Communication`

- 通过 WebRTC 进行 P2P 视频通话（内置 HTML 页面）
- 屏幕共享：浏览器标签页、应用窗口或整个桌面
- 视频通话房间（创建/加入/等待/结束）
- 通过 WebSocket 实时传输视频和音频
- 第二位参与者连接前的镜像预览
- 服务端视频通话录制（WebM/MP4）
- 编解码器和分辨率选择

## 聊天
`F-Droid: Messaging` `FreeDesktop: Network;Chat` `MS Store: Social` `Google Play: Communication`

- 内置文字聊天
- 消息历史记录
- 收到消息时自动打开聊天
- 发送截图和视频录像到聊天

## 视频/音频录制
`F-Droid: Multimedia` `FreeDesktop: AudioVideo;Recorder` `MS Store: Photo + video` `Google Play: Video Players & Editors`

- 通过浏览器使用摄像头/麦克风录制视频和音频
- 带时间戳的录制（YYYYMMDD_HHMMSS_mmm）
- 将录制内容发布到聊天

## 摄像头
`F-Droid: Multimedia` `FreeDesktop: AudioVideo` `MS Store: Photo + video` `Google Play: Photography`

- 通过浏览器捕获摄像头画面
- 视频通话截图

## 安全
`F-Droid: Security` `FreeDesktop: Security` `MS Store: Security` `Google Play: Tools`

- 基于 PAKE 的端到端加密
- 带计时器的一次性密码（TOTP）
- 通过输入框或 CROC_SECRET 环境变量设置密钥
- 加密椭圆曲线选择
- 哈希算法：imohash、md5、xxhash、highway
- 中继密码
- 通过中继的加密隧道

## 二维码与深度链接
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- 根据代码短语生成二维码
- 通过摄像头扫描二维码
- 支持多种 Android 扫描器（小米、三星、OPlus、BinaryEye、Lens、ZXing、Chrome、Via、三星浏览器、Opera mini、Microsoft、Firefox）
- 深度链接：`https://abakum.github.io/croc#...`（Base64 编码的设置）
- 深度链接：`davX:` / `webdavX:` 打开 WebDAV

## 中继配置
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- 中继配置列表（保存/加载）
- 在多个中继之间切换
- 带 GUI 管理的本地中继
- 自定义地址、IPv6、端口、密码
- 代理支持：SOCKS5（包括 Tor）、HTTP

## 传输设置
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- 禁用本地传输（仅中继）
- 仅连接本地发送者
- .gitignore 支持
- 覆盖文件
- 压缩开/关
- 多路复用开/关
- 发送时打包为 ZIP
- 上传限速

## 界面
`F-Droid: Personalization` `FreeDesktop: Settings` `MS Store: Personalization` `Google Play: Personalization`

- 主题：跟随系统、浅色、灰色、深色、黑色
- 字体选择（嵌入字体 + 系统字体）
- 多语言（en-US、tr-TR、ja-JP、zh-CN、ru-RU）
- 隐藏 Logo
- 彩色传输日志
- Android：通过 `adb logcat -s croc` 查看日志

## 命令行模式

如果传入了至少一个非文件参数，crocson 将作为 croc 命令行工具运行：
- 恢复中断的传输（断点续传）
- 管道（标准输入/输出）：`cat file | crocson send`
- 发送文本：`crocson send --text "hello"`
- 为移动设备生成二维码
- 脚本静默模式
- 复制代码到剪贴板
- 排除文件夹（`--exclude`）
- 使用 CROC_SECRET 环境变量保障进程安全

---

## 各应用商店类别排名

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
