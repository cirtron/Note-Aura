// Package worker runs the asynchronous AI pipeline: it consumes queued jobs and,
// per note source type, fetches/OCRs the content, then generates title, summary,
// tags, and embeddings before marking the note ready.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"note-aura/internal/ai"
	"note-aura/internal/db"
	"note-aura/internal/i18n"
	"note-aura/internal/ingest"
	"note-aura/internal/rag"
	"note-aura/internal/syslog"
)

const (
	maxAttempts = 3
	chunkRunes  = 1200
	chunkOver   = 150
)

// Worker processes jobs from the database queue.
type Worker struct {
	db       *db.DB
	fallback ai.GlobalConfig // config.yaml defaults; admin app_settings overlay it
	timeout  time.Duration
	notify   chan struct{}

	mu      sync.Mutex
	cancels map[int64]context.CancelFunc // note id -> cancel for the in-flight run
}

// New builds a Worker. fallback holds the config.yaml AI defaults; the live admin
// app_settings and per-user cloud overrides are resolved per job.
func New(database *db.DB, fallback ai.GlobalConfig, timeout time.Duration) *Worker {
	return &Worker{db: database, fallback: fallback, timeout: timeout,
		notify: make(chan struct{}, 1), cancels: map[int64]context.CancelFunc{}}
}

// Kick signals the pool that a new job is available (non-blocking).
func (w *Worker) Kick() {
	select {
	case w.notify <- struct{}{}:
	default:
	}
}

// Start launches n worker goroutines. Stuck jobs from a previous run are
// requeued first.
func (w *Worker) Start(n int) {
	if err := w.db.RequeueStuckJobs(); err != nil {
		log.Printf("worker: requeue stuck jobs: %v", err)
	}
	for i := 0; i < n; i++ {
		go w.loop()
	}
}

func (w *Worker) loop() {
	for {
		job, err := w.db.ClaimJob()
		if err == db.ErrNotFound {
			select {
			case <-w.notify:
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if err != nil {
			log.Printf("worker: claim job: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		w.process(job)
	}
}

func (w *Worker) process(job *db.Job) {
	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()
	w.register(job.NoteID, cancel)
	defer w.deregister(job.NoteID)

	if err := w.run(ctx, job); err != nil {
		// A user Stop cancels ctx and has already set the note's final state and
		// deleted its jobs — don't fight it by re-marking failed/requeuing.
		if errors.Is(err, context.Canceled) {
			return
		}
		syslog.Errorf("worker", "note %d job %d failed (attempt %d): %v", job.NoteID, job.ID, job.Attempts, err)
		_ = w.db.FailJob(job.ID, job.Attempts, maxAttempts, err.Error())
		if job.Attempts >= maxAttempts {
			_ = w.db.SetNoteStatus(job.NoteID, "failed", err.Error())
		}
		return
	}
	_ = w.db.CompleteJob(job.ID)
	syslog.Infof("worker", "completed note %d job %d", job.NoteID, job.ID)
}

func (w *Worker) register(noteID int64, cancel context.CancelFunc) {
	w.mu.Lock()
	w.cancels[noteID] = cancel
	w.mu.Unlock()
}

func (w *Worker) deregister(noteID int64) {
	w.mu.Lock()
	delete(w.cancels, noteID)
	w.mu.Unlock()
}

// Cancel aborts the in-flight AI run for a note, if one is currently running.
func (w *Worker) Cancel(noteID int64) {
	w.mu.Lock()
	cancel := w.cancels[noteID]
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// run performs the full pipeline for one note.
func (w *Worker) run(ctx context.Context, job *db.Job) error {
	note, err := w.db.GetNote(job.NoteID)
	if err == db.ErrNotFound {
		return nil // note deleted before processing; treat as success
	}
	if err != nil {
		return err
	}
	start := time.Now() // measured for the note's AI processing time
	syslog.Infof("worker", "processing note %d job %d (source=%s)", note.ID, job.ID, note.SourceType)

	// Build the provider from the live admin config overlaid on config.yaml
	// defaults, plus this user's optional cloud override.
	app, err := w.db.GetAppSettings()
	if err != nil {
		return err
	}
	global := ai.LoadGlobal(app, w.fallback)
	global = ai.ApplySourcePrompts(global, app, note.SourceType) // web/youtube prompt overrides
	settings, err := w.db.GetUserSettings(note.OwnerID)
	if err != nil {
		return err
	}
	// External-server users' own prompts apply (cloud branch only; no-op for Ollama).
	provider := ai.BuildProvider(global, settings, true)
	canAI, usingOllama := w.db.UserAICapability(note.OwnerID)

	// Decide whether AI actually runs: allowed AND (if on Ollama) within the
	// owner's daily Ollama-use limit.
	doAI := canAI
	if doAI && usingOllama {
		if limit := w.db.OllamaDailyLimit(note.OwnerID); limit > 0 && w.db.OllamaUsedToday(note.OwnerID) >= limit {
			doAI = false
		}
	}
	// Record how long the AI run took (shown on the note), even if a later step
	// fails or times out. Only when AI actually runs.
	if doAI {
		defer func() { _ = w.db.SetNoteAITime(note.ID, time.Since(start).Milliseconds()) }()
	}

	lang := i18n.EnglishName(note.SummaryLang)

	// 1. Resolve the source into plain text (for AI/search) + Markdown body.
	text, bodyMd, fetchedTitle, err := w.materialize(ctx, provider, note, doAI, lang)
	if err != nil {
		return err
	}
	text = strings.TrimSpace(text)

	// When AI is unavailable (role disallows Ollama and no cloud key, or daily
	// Ollama limit reached), save the content as-is and mark the note ready.
	if !doAI {
		title := note.Title
		if title == "" {
			title = fetchedTitle
		}
		if title == "" {
			title = "(untitled)"
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return w.db.ApplyAIResult(note.ID, title, note.Summary, bodyMd, text)
	}

	if text == "" {
		return fmt.Errorf("no text content to process")
	}

	// 2. Regenerate only the AI fields requested in the job (title/summary/tags);
	// the rest keep their current values.
	want := parseParts(job.Params)

	title := note.Title
	if want["title"] {
		if t, terr := provider.Title(ctx, text, lang); terr == nil && t != "" {
			title = t
		} else if fetchedTitle != "" {
			title = fetchedTitle
		}
		if title == "" {
			title = "(untitled)"
		}
	} else if title == "" && fetchedTitle != "" {
		title = fetchedTitle // never leave a note untitled
	}

	// Persist the fetched/materialized content up front so it's captured even if a
	// later AI step (summary/tags/embed) fails — e.g. when Ollama is unreachable.
	// The note then shows its full body plus a Retry button rather than being empty.
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.db.ApplyAIResult(note.ID, title, note.Summary, bodyMd, text); err != nil {
		return err
	}

	summary := note.Summary
	if want["summary"] {
		s, serr := provider.Summarize(ctx, text, lang)
		if serr != nil {
			return fmt.Errorf("summarize: %w", serr)
		}
		summary = s
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.db.ApplyAIResult(note.ID, title, summary, bodyMd, text); err != nil {
		return err
	}
	if want["tags"] {
		tags, terr := provider.Tags(ctx, text)
		if terr != nil {
			return fmt.Errorf("tags: %w", terr)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := w.db.SetNoteTags(note.OwnerID, note.ID, "ai", tags); err != nil {
			return err
		}
	}

	// AI category: prefer an existing category, else create one. Best-effort —
	// a failure here doesn't fail the note (content + summary are already saved).
	if want["category"] {
		var names []string
		if cats, cerr := w.db.CategoriesWithCounts(note.OwnerID); cerr == nil {
			for _, ct := range cats {
				names = append(names, ct.Name)
			}
		}
		if cat, cerr := provider.Category(ctx, text, names, lang); cerr == nil {
			if cat = strings.TrimSpace(cat); cat != "" {
				if cid, e := w.db.UpsertCategory(note.OwnerID, cat); e == nil && cid > 0 {
					if err := ctx.Err(); err != nil {
						return err
					}
					_ = w.db.SetNoteCategory(note.ID, &cid)
				}
			}
		}
	}

	// 3. Chunk + embed for RAG.
	chunks := rag.Chunk(text, chunkRunes, chunkOver)
	if len(chunks) > 0 {
		embs, err := provider.Embed(ctx, chunks)
		if err != nil {
			return fmt.Errorf("embed: %w", err)
		}
		rows := make([]db.Chunk, 0, len(chunks))
		for i := range chunks {
			if i >= len(embs) || len(embs[i]) == 0 {
				continue
			}
			rows = append(rows, db.Chunk{
				NoteID:    note.ID,
				Index:     i,
				Text:      chunks[i],
				Embedding: rag.EncodeEmbedding(embs[i]),
			})
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := w.db.ReplaceChunks(note.ID, rows); err != nil {
			return err
		}
	}
	// Count one successful Ollama use toward the owner's daily quota.
	if usingOllama {
		_ = w.db.IncrementOllamaUsage(note.OwnerID)
	}
	return nil
}

// parseParts returns the set of AI fields a job should (re)generate. An empty
// params string means all of them (used by first-time processing).
func parseParts(s string) map[string]bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return map[string]bool{"title": true, "summary": true, "tags": true, "category": true}
	}
	m := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			m[t] = true
		}
	}
	return m
}

// materialize returns (plainText, markdownBody, fetchedTitle). Extracted text is
// already plain text, which is valid Markdown, so the two are the same for
// non-manual sources.
func (w *Worker) materialize(ctx context.Context, provider ai.Provider, note *db.Note, canAI bool, lang string) (string, string, string, error) {
	// Reprocessing path: once a note already has body content — either captured on
	// first run or hand-edited by the user — an AI rerun/retry uses that current
	// body instead of re-fetching the source. This makes the AI act on the user's
	// edits and never overwrites them. Only the initial capture (empty body) fetches.
	if strings.TrimSpace(note.BodyText) != "" {
		return note.BodyText, note.BodyMd, "", nil
	}

	switch note.SourceType {
	case "manual":
		return note.BodyText, note.BodyMd, "", nil

	case "url":
		if ingest.IsFacebook(note.SourceRef) {
			cookies := ""
			if userSettings, err := w.db.GetUserSettings(note.OwnerID); err == nil {
				cookies = userSettings["facebook.cookies"]
			}
			if cookies == "" {
				if app, err := w.db.GetAppSettings(); err == nil {
					cookies = app["facebook.cookies"]
				}
			}
			f, err := ingest.FetchFacebook(ctx, note.SourceRef, cookies, ingest.EnableHeadless)
			if err != nil {
				return "", "", "", err
			}
			return f.Text, f.Text, f.Title, nil
		}
		f, err := ingest.FetchURL(ctx, note.SourceRef)
		if err != nil {
			return "", "", "", err
		}
		return f.Text, f.Text, f.Title, nil

	case "youtube":
		text, title, err := ingest.FetchYouTubeTranscript(ctx, note.SourceRef)
		if err != nil {
			return "", "", "", err
		}
		return text, text, title, nil

	case "image":
		atts, err := w.db.AttachmentsForNote(note.ID)
		if err != nil {
			return "", "", "", err
		}
		if len(atts) == 0 {
			return "", "", "", fmt.Errorf("image note has no attachment")
		}
		if !canAI {
			return "", "", "", nil // OCR is an AI feature; store the image, no text
		}
		var parts []string
		for _, a := range atts {
			data, err := os.ReadFile(a.Path)
			if err != nil {
				return "", "", "", fmt.Errorf("read attachment: %w", err)
			}
			out, err := provider.OCR(ctx, data, a.Mime, lang)
			if err != nil {
				return "", "", "", fmt.Errorf("ocr: %w", err)
			}
			_ = w.db.SetAttachmentOCR(a.ID, out)
			// Image analysis adds visual context only when OCR found no substantial
			// text; for text-heavy documents the Describe model would just repeat
			// the OCR output, causing duplication.
			block := out
			if len([]rune(strings.TrimSpace(out))) < 50 {
				if desc, derr := provider.Describe(ctx, data, a.Mime, lang); derr != nil {
					syslog.Errorf("worker", "note %d image analysis failed: %v", note.ID, derr)
				} else if desc != "" {
					block = strings.TrimSpace(out + "\n\n[Image] " + desc)
				}
			}
			parts = append(parts, block)
		}
		text := strings.TrimSpace(strings.Join(parts, "\n\n"))
		return text, text, "", nil

	default:
		return note.BodyText, note.BodyMd, "", nil
	}
}
