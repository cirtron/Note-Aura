# Note-Aura

**自架 AI 知識收件匣 — 什麼都能丟進來，隨時問得回去。**

貼上連結、轉寄 email、上傳圖片、或直接打筆記。
背景 AI 自動為你命名、摘要、打標籤、建立語義索引。
用自然語言**向筆記提問**（RAG 附引用來源），或以關鍵字搜尋。

單一 Go 執行檔 + SQLite，預設使用本地 [Ollama](https://ollama.com) 模型。
資料不出你的伺服器，也支援任何 OpenAI 相容端點。

[快速開始](#執行) · [安裝指南](INSTALL.md) · [使用手冊](USER_GUIDE.zh-Hant.md) · [English](README.md) · [简中](README.zh-Hans.md) · [日本語](README.ja.md) · [한국어](README.ko.md)

---

## 功能

- **擷取 → 處理 → 取回循環。** 儲存即時完成；背景工作池執行耗時的 AI 任務，完成後將筆記從「處理中」切換為「就緒」。AI 不可用時內容仍會保存（筆記提供**重試**選項）。
- **擷取時自動整理** — AI 自動產生標題、摘要、標籤與**分類**（只填你留空的欄位），摘要語言可自選或自動偵測。
- **網頁連結、YouTube 與 Facebook 擷取** — 貼入任何連結，自動抓取頁面全文或影片逐字稿並摘要。支援 JavaScript 渲染頁面的無頭瀏覽器備援。Facebook 貼文、影片與 Reels 均支援，管理員可提供 `cookies.txt` 擷取登入內容。
- **停止按鈕** — 隨時取消筆記的 AI 處理。筆記進入「已停止」狀態（有別於「失敗」），可於稍後重試。
- **圖片 OCR** — 上傳照片或截圖，使用視覺模型提取文字。
- **檔案上傳** — 依角色設定允許的檔案類型；文字檔直接成為筆記內容，其他格式作為可下載附件儲存。
- **Email → 筆記** — 每位使用者擁有專屬收件位址；寄到該地址的郵件自動成為筆記（純連結郵件會抓取連結頁面）。
- **向筆記提問（RAG）** — 以嵌入向量做語義檢索，餵給對話模型後以附引用來源的方式回答。
- **多使用者** — Email／密碼帳號、email 驗證、內建圖形驗證碼（不依賴外部服務）、邀請制；筆記可逐篇分享（唯讀或可編輯）及建立群組（含共同管理員與讀寫控制）。
- **行事曆** — 筆記可設定活動日期、開始／結束時間與全天旗標；月視圖加每日議程顯示。支援 Email 提醒與各國國定假日。
- **Markdown 編輯器**（EasyMDE）— 即時預覽與行內圖片；內容透過 goldmark 渲染並以 bluemonday 清理。
- **整理與瀏覽** — 階層式**分類**（`父類別/子類別`）、標籤、關鍵字搜尋、多種**排序**方式，以及可選頁面大小的分頁。手機上分類／標籤篩選器收納於選單下方的**篩選**折疊面板。
- **筆記管理** — 多選**批量刪除**，以及將所有筆記匯出／匯入 JSON 檔案（於設定中操作）。
- **角色與配額** — 依角色設定儲存空間、AI 存取與每日上限、群組／邀請上限，以及允許的上傳類型；支援個別使用者覆寫。
- **可插拔 AI 後端** — 預設使用本地 **Ollama**；每位使用者可覆寫為任何 OpenAI 相容端點（OpenAI、Gemini compat、OpenRouter 等），並可自訂提示詞。
- **管理後台** — 每位使用者的使用量、最新筆記、伺服器監控；管理使用者（停用／刪除）、角色、品牌、模型與提示詞、國定假日、註冊設定、Email，以及 **HTTPS 開關**。
- **多語系介面** — English／繁體中文／简体中文／日本語（預設跟隨瀏覽器語言；可透過頁首切換並記憶於帳號）。驗證、密碼重設、邀請等系統郵件以各收件人的語言寄送。

## 技術棧

Go + Fiber、SQLite（`modernc.org/sqlite`，純 Go，無 CGO）、FTS5 全文搜尋、純 Go 餘弦相似度向量搜尋、`html/template` 伺服器端渲染。

## 執行

1. 安裝 [Ollama](https://ollama.com) 並下載預設模型（也可在設定中改用 OpenAI 相容後端）：
   ```
   ollama pull llama3.1
   ollama pull nomic-embed-text
   ollama pull deepseek-ocr
   ```
2. 複製並編輯設定檔：
   ```
   cp config.example.yaml config.yaml
   ```
3. 編譯並執行：
   ```
   go build -o note-aura.exe .
   ./note-aura.exe
   ```
4. 開啟 http://localhost:8090 並註冊帳號（第一個帳號自動成為管理員）。

**維護工具**（在專案資料夾中執行）：

```powershell
.\update.ps1     # Windows：更新至新程式碼，保留所有資料（停止 → 重新編譯 → 重啟）
.\reset.exe      # 將所有資料清除回全新狀態（編譯：go build -o reset.exe ./cmd/reset/）
```

```bash
./update.sh      # Linux/macOS：同上（自動偵測 systemd）
```

`update.ps1` / `update.sh` 不會動到資料庫或上傳檔案；`reset.exe` 會永久刪除（執行前請先停止伺服器）。詳見 [INSTALL.md](INSTALL.md#9-backup-upgrade--reset)。

如需 **HTTPS**，在 `config.yaml` 中指定憑證與金鑰（PEM 格式），並設定 `session.secure: true` 及 `https://` 的 `base_url`：

```yaml
tls:
  cert_file: "certs/note-aura.crt"
  key_file:  "certs/note-aura.key"
```

> **選用整合：** YouTube 擷取需要 [`yt-dlp`](https://github.com/yt-dlp/yt-dlp) 在 `PATH` 中；無頭瀏覽器備援需要 Chrome/Chromium；Email→筆記、提醒、驗證與邀請功能需設定 SMTP/IMAP。詳見 [INSTALL.md](INSTALL.md)。

完整設定請參閱 **[INSTALL.md](INSTALL.md)**，使用說明請參閱 **[USER_GUIDE.zh-Hant.md](USER_GUIDE.zh-Hant.md)**。

## 專案結構

```
main.go                 連線：設定、資料庫、工作池、伺服器、Email 輪詢
internal/
  config/   db/         YAML 設定；SQLite schema + 查詢（FTS 同步）
  auth/                 bcrypt + session token
  ai/                   Provider 介面；Ollama + OpenAI 相容實作
  ingest/               HTML→文字、URL 抓取（JSON-LD + 無頭瀏覽器）、YouTube 逐字稿
  rag/                  分塊、嵌入（反）序列化、餘弦相似度、top-k
  worker/               非同步作業管線（抓取/OCR → 標題/摘要/標籤/嵌入）
  mailer/   reminder/   寄件 SMTP；行事曆提醒排程器
  emailin/              收件 IMAP → 筆記輪詢器
  server/               Fiber 路由 + 處理器
cmd/reset/              獨立「清除所有資料」工具
web/templates/          伺服器端渲染頁面（Markdown 編輯器）
update.ps1              就地更新執行檔，保留所有資料
```
