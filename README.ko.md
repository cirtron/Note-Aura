# Note-Aura

**셀프호스팅 AI 지식 받은 편지함 — 무엇이든 넣고, 언제든 물어보세요.**

URL을 붙여넣거나, 이메일을 전달하거나, 사진을 업로드하거나, 직접 메모를 작성하세요.
백그라운드 AI가 자동으로 제목·요약·태그·의미 색인을 생성합니다.
자연어로 **노트에 질문**하거나(RAG + 출처 인용), 키워드로 검색하세요.

단일 Go 바이너리 + SQLite로 실행됩니다. 기본적으로 로컬 [Ollama](https://ollama.com) 모델을 사용합니다.
데이터가 서버 밖으로 나가지 않으며, OpenAI 호환 엔드포인트도 지원합니다.

[빠른 시작](#실행) · [설치 가이드](INSTALL.md) · [사용 설명서](USER_GUIDE.ko.md) · [English](README.md) · [繁中](README.zh-Hant.md) · [简中](README.zh-Hans.md) · [日本語](README.ja.md)

---

## 기능

- **수집 → 처리 → 검색 루프.** 저장은 즉시 완료됩니다. 백그라운드 워커 풀이 무거운 AI 작업을 처리하고, 완료되면 노트 상태를 「처리 중」에서 「완료」로 전환합니다. AI를 사용할 수 없어도 내용은 저장됩니다(노트에 **재시도** 버튼 표시).
- **수집 시 자동 정리** — AI가 제목·요약·태그·**카테고리**를 자동 생성(비워 둔 필드에만 적용). 요약 언어는 수동 선택 또는 콘텐츠에 맞게 자동 감지.
- **웹 링크·YouTube·Facebook 수집** — 링크를 붙여넣으면 페이지 전체 텍스트나 동영상 자막을 가져와 요약합니다. JavaScript 렌더링 페이지를 위한 헤드리스 브라우저 폴백도 지원. Facebook 게시물·동영상·릴스를 지원하며, 관리자가 `cookies.txt`를 설정해 로그인이 필요한 콘텐츠에도 대응 가능.
- **중지 버튼** — AI 처리 중인 노트를 언제든 취소할 수 있습니다. 노트는 「중지됨」 상태가 되며(오류인 「실패」와는 구분), 나중에 재시도할 수 있습니다.
- **이미지 OCR** — 사진이나 스크린샷을 업로드하면 비전 모델이 텍스트를 추출합니다.
- **파일 업로드** — 역할별로 허용 파일 형식을 설정합니다. 텍스트 파일은 노트 본문이 되고, 나머지는 다운로드 가능한 첨부 파일로 저장됩니다.
- **이메일 → 노트** — 각 사용자에게 전용 플러스 주소가 부여되며, 해당 주소로 보낸 메일이 자동으로 노트가 됩니다(링크만 있는 이메일은 링크 페이지를 가져옵니다).
- **노트에 질문(RAG)** — 임베딩 벡터로 의미 검색을 수행한 뒤 대화 모델이 출처 노트를 인용하며 답변합니다.
- **멀티 사용자** — 이메일/비밀번호 계정, 이메일 인증, 외부 서비스 없이 동작하는 내장 이미지 CAPTCHA, 초대제. 노트별 공유(읽기 전용 또는 편집 가능)와 그룹(공동 관리자·읽기/쓰기 권한 설정 포함) 지원.
- **캘린더** — 노트에 이벤트 날짜·시작/종료 시간·종일 플래그를 설정할 수 있습니다. 월 보기 + 일별 일정 표시. 이메일 리마인더와 각국 공휴일도 지원.
- **Markdown 편집기**(EasyMDE) — 실시간 미리보기와 인라인 이미지 지원. goldmark로 렌더링하고 bluemonday로 정제.
- **정리 및 탐색** — 계층형 **카테고리**(`상위/하위` 형식), 태그, 키워드 검색, 다양한 **정렬** 옵션, 페이지 크기 조절 가능한 페이지네이션. 모바일에서는 카테고리/태그 필터를 메뉴 아래 접이식 **필터** 패널에 수납.
- **노트 관리** — 다중 선택 **일괄 삭제**, 모든 노트를 JSON 파일로 내보내기/가져오기(설정에서 조작).
- **역할과 할당량** — 역할별 저장 용량·AI 접근·일일 상한·그룹/초대 상한·허용 업로드 형식을 설정. 사용자별 개별 재정의도 가능.
- **교체 가능한 AI 백엔드** — 기본값은 로컬 **Ollama**. 사용자별로 임의의 OpenAI 호환 엔드포인트(OpenAI·Gemini 호환·OpenRouter 등)로 재정의 가능. 프롬프트 커스터마이징도 지원.
- **관리 대시보드** — 사용자별 사용량·최신 노트·서버 모니터. 사용자 관리(정지/삭제), 역할, 브랜딩, 모델과 프롬프트, 공휴일, 가입 설정, 이메일, **HTTPS 온/오프 전환** 관리.
- **다국어 UI** — English/繁體中文/简体中文/日本語(기본값은 브라우저 언어, 헤더에서 전환하면 계정에 기억됨). 인증·비밀번호 재설정·초대 이메일은 각 수신자의 언어로 발송.

## 기술 스택

Go + Fiber, SQLite(`modernc.org/sqlite`, 순수 Go — CGO 없음), FTS5 전문 검색, Go 구현 코사인 유사도 벡터 검색, `html/template` 서버 사이드 렌더링.

## 실행

1. [Ollama](https://ollama.com)를 설치하고 기본 모델을 다운로드합니다(설정에서 나중에 OpenAI 호환 백엔드로 변경 가능):
   ```
   ollama pull llama3.1
   ollama pull nomic-embed-text
   ollama pull deepseek-ocr
   ```
2. 설정 파일을 복사하고 편집합니다:
   ```
   cp config.example.yaml config.yaml
   ```
3. 빌드하고 실행합니다:
   ```
   go build -o note-aura.exe .
   ./note-aura.exe
   ```
4. http://localhost:8090 을 열어 계정을 등록합니다(첫 번째 계정이 자동으로 관리자가 됩니다).

**유지 관리 도구**(프로젝트 폴더에서 실행):

```powershell
.\update.ps1     # Windows: 새 코드로 업데이트, 모든 데이터 유지 (중지 → 재빌드 → 재시작)
.\reset.exe      # 모든 데이터를 초기 상태로 초기화 (빌드: go build -o reset.exe ./cmd/reset/)
```

```bash
./update.sh      # Linux/macOS: 동일 (systemd 자동 감지)
```

`update.ps1` / `update.sh`는 데이터베이스나 업로드 파일에 영향을 주지 않습니다. `reset.exe`는 영구 삭제합니다(실행 전 서버를 중지하세요). 자세한 내용은 [INSTALL.md](INSTALL.md#9-backup-upgrade--reset)를 참조하세요.

**HTTPS**로 제공하려면 `config.yaml`에서 인증서와 키(PEM 형식)를 지정하고, `session.secure: true` 및 `https://` `base_url`을 설정하세요:

```yaml
tls:
  cert_file: "certs/note-aura.crt"
  key_file:  "certs/note-aura.key"
```

> **선택적 연동:** YouTube 수집에는 [`yt-dlp`](https://github.com/yt-dlp/yt-dlp)가 `PATH`에 필요합니다. 헤드리스 웹 링크 폴백에는 Chrome/Chromium이 필요합니다. 이메일→노트·리마인더·인증·초대 기능에는 SMTP/IMAP 설정이 필요합니다. 자세한 내용은 [INSTALL.md](INSTALL.md)를 참조하세요.

전체 설정은 **[INSTALL.md](INSTALL.md)**, 사용 방법은 **[USER_GUIDE.ko.md](USER_GUIDE.ko.md)**를 참조하세요.

## 프로젝트 구조

```
main.go                 연결: 설정·DB·워커 풀·서버·이메일 폴링
internal/
  config/   db/         YAML 설정; SQLite 스키마 + 쿼리 (FTS 동기화)
  auth/                 bcrypt + 세션 토큰
  ai/                   Provider 인터페이스; Ollama + OpenAI 호환 구현
  ingest/               HTML→텍스트, URL 가져오기(JSON-LD + 헤드리스), YouTube 자막
  rag/                  청킹, 임베딩 (역)직렬화, 코사인 유사도, top-k
  worker/               비동기 작업 파이프라인 (가져오기/OCR → 제목/요약/태그/임베딩)
  mailer/   reminder/   발신 SMTP; 캘린더 리마인더 스케줄러
  emailin/              수신 IMAP → 노트 폴링
  server/               Fiber 라우팅 + 핸들러
cmd/reset/              독립형 「전체 데이터 삭제」 도구
web/templates/          서버 사이드 렌더링 페이지 (Markdown 편집기)
update.ps1              모든 데이터를 유지하면서 바이너리를 제자리에서 업데이트
```
