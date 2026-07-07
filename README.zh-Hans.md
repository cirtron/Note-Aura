# Note-Aura

**自部署 AI 知识收件箱 — 什么都能放进来，随时问得回去。**

粘贴链接、转发邮件、上传图片、或直接写笔记。
后台 AI 自动命名、摘要、打标签、建立语义索引。
用自然语言**向笔记提问**（RAG 附引用来源），或以关键词搜索。

单一 Go 可执行文件 + SQLite，默认使用本地 [Ollama](https://ollama.com) 模型。
数据不离开你的服务器，也支持任何 OpenAI 兼容端点。

[快速开始](#运行) · [安装指南](INSTALL.md) · [使用手册](USER_GUIDE.zh-Hans.md) · [English](README.md) · [繁中](README.zh-Hant.md) · [日本語](README.ja.md) · [한국어](README.ko.md)

---

## 功能

- **采集 → 处理 → 取回循环。** 保存即时完成；后台工作池执行耗时的 AI 任务，完成后将笔记从「处理中」切换为「就绪」。AI 不可用时内容仍会保存（笔记提供**重试**选项）。
- **采集时自动整理** — AI 自动生成标题、摘要、标签与**分类**（只填你留空的字段），摘要语言可自选或自动检测。
- **网页链接、YouTube 与 Facebook 采集** — 粘贴任何链接，自动抓取页面全文或视频字幕并摘要。支持 JavaScript 渲染页面的无头浏览器备选。Facebook 帖子、视频与 Reels 均支持，管理员可提供 `cookies.txt` 采集需登录的内容。
- **停止按钮** — 随时取消笔记的 AI 处理。笔记进入「已停止」状态（有别于「失败」），可于稍后重试。
- **图片 OCR** — 上传照片或截图，使用视觉模型提取文字。
- **文件上传** — 依角色设置允许的文件类型；文本文件直接成为笔记内容，其他格式作为可下载附件储存。
- **Email → 笔记** — 每位用户拥有专属收件地址；发送到该地址的邮件自动成为笔记（纯链接邮件会抓取链接页面）。
- **向笔记提问（RAG）** — 以嵌入向量做语义检索，喂给对话模型后以附引用来源的方式回答。
- **多用户** — Email／密码账号、Email 验证、内置图形验证码（不依赖外部服务）、邀请制；笔记可逐篇分享（只读或可编辑）及建立群组（含共同管理员与读写控制）。
- **日历** — 笔记可设置活动日期、开始／结束时间与全天标志；月视图加每日议程显示。支持 Email 提醒与各国法定节假日。
- **Markdown 编辑器**（EasyMDE）— 实时预览与行内图片；内容通过 goldmark 渲染并以 bluemonday 清理。
- **整理与浏览** — 层级式**分类**（`父类别/子类别`）、标签、关键词搜索、多种**排序**方式，以及可选页面大小的分页。手机上分类／标签筛选器收纳于菜单下方的**筛选**折叠面板。
- **笔记管理** — 多选**批量删除**，以及将所有笔记导出／导入 JSON 文件（在设置中操作）。
- **角色与配额** — 依角色设置存储空间、AI 访问与每日上限、群组／邀请上限，以及允许的上传类型；支持个别用户覆盖。
- **可插拔 AI 后端** — 默认使用本地 **Ollama**；每位用户可覆盖为任何 OpenAI 兼容端点（OpenAI、Gemini compat、OpenRouter 等），并可自定义提示词。
- **管理后台** — 每位用户的使用量、最新笔记、服务器监控；管理用户（停用／删除）、角色、品牌、模型与提示词、法定节假日、注册设置、Email，以及 **HTTPS 开关**。
- **多语言界面** — English／繁體中文／简体中文／日本語（默认跟随浏览器语言；可通过页首切换并记忆于账号）。验证、密码重置、邀请等系统邮件以各收件人的语言发送。

## 技术栈

Go + Fiber、SQLite（`modernc.org/sqlite`，纯 Go，无 CGO）、FTS5 全文搜索、纯 Go 余弦相似度向量搜索、`html/template` 服务端渲染。

## 运行

1. 安装 [Ollama](https://ollama.com) 并拉取默认模型（也可在设置中改用 OpenAI 兼容后端）：
   ```
   ollama pull llama3.1
   ollama pull nomic-embed-text
   ollama pull deepseek-ocr
   ```
2. 复制并编辑配置文件：
   ```
   cp config.example.yaml config.yaml
   ```
3. 编译并运行：
   ```
   go build -o note-aura.exe .
   ./note-aura.exe
   ```
4. 打开 http://localhost:8090 并注册账号（第一个账号自动成为管理员）。

**维护工具**（在项目文件夹中运行）：

```powershell
.\update.ps1     # Windows：更新至新代码，保留所有数据（停止 → 重新编译 → 重启）
.\reset.exe      # 将所有数据清除回全新状态（编译：go build -o reset.exe ./cmd/reset/）
```

```bash
./update.sh      # Linux/macOS：同上（自动检测 systemd）
```

`update.ps1` / `update.sh` 不会动到数据库或上传文件；`reset.exe` 会永久删除（运行前请先停止服务器）。详见 [INSTALL.md](INSTALL.md#9-backup-upgrade--reset)。

如需 **HTTPS**，在 `config.yaml` 中指定证书与密钥（PEM 格式），并设置 `session.secure: true` 及 `https://` 的 `base_url`：

```yaml
tls:
  cert_file: "certs/note-aura.crt"
  key_file:  "certs/note-aura.key"
```

> **可选集成：** YouTube 采集需要 [`yt-dlp`](https://github.com/yt-dlp/yt-dlp) 在 `PATH` 中；无头浏览器备选需要 Chrome/Chromium；Email→笔记、提醒、验证与邀请功能需配置 SMTP/IMAP。详见 [INSTALL.md](INSTALL.md)。

完整配置请参阅 **[INSTALL.md](INSTALL.md)**，使用说明请参阅 **[USER_GUIDE.zh-Hans.md](USER_GUIDE.zh-Hans.md)**。

## 项目结构

```
main.go                 连线：配置、数据库、工作池、服务器、Email 轮询
internal/
  config/   db/         YAML 配置；SQLite schema + 查询（FTS 同步）
  auth/                 bcrypt + session token
  ai/                   Provider 接口；Ollama + OpenAI 兼容实现
  ingest/               HTML→文本、URL 抓取（JSON-LD + 无头浏览器）、YouTube 字幕
  rag/                  分块、嵌入（反）序列化、余弦相似度、top-k
  worker/               异步任务管道（抓取/OCR → 标题/摘要/标签/嵌入）
  mailer/   reminder/   发件 SMTP；日历提醒调度器
  emailin/              收件 IMAP → 笔记轮询器
  server/               Fiber 路由 + 处理器
cmd/reset/              独立「清除所有数据」工具
web/templates/          服务端渲染页面（Markdown 编辑器）
update.ps1              就地更新可执行文件，保留所有数据
```
