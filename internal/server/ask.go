package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/ai"
	"note-aura/internal/rag"
)

const (
	askTopK      = 6
	askMinScore  = 0.15
	askSystemMsg = "You are Note-Aura's assistant. Answer the user's question using ONLY the provided " +
		"context from their notes. Cite the note numbers you used like [1], [2]. If the context does not " +
		"contain the answer, say you couldn't find it in their notes."
)

func (s *Server) askForm(c *fiber.Ctx) error {
	if !canUseAI(c) {
		return c.Redirect("/notes", fiber.StatusFound)
	}
	m := baseMap(c, "Ask your notes")
	m["Nav"] = "ask"
	return c.Render("ask", m, "layout")
}

// citation is a note surfaced as a source for the answer.
type citation struct {
	Num    int
	NoteID int64
	Title  string
	Score  float32
}

func (s *Server) ask(c *fiber.Ctx) error {
	if !canUseAI(c) {
		return c.Redirect("/notes", fiber.StatusFound)
	}
	u := currentUser(c)
	question := strings.TrimSpace(c.FormValue("q"))
	m := baseMap(c, "Ask your notes")
	m["Nav"] = "ask"
	m["Question"] = question
	if question == "" {
		return c.Render("ask", m, "layout")
	}
	if s.overOllamaDaily(u) {
		m["Error"] = "You've reached your daily AI limit. Try again tomorrow, or set your own API key in Settings."
		return c.Render("ask", m, "layout")
	}

	ctx, cancel := context.WithTimeout(c.Context(), time.Duration(s.cfg.AI.TimeoutSeconds)*time.Second)
	defer cancel()

	provider := s.providerFor(u.ID)

	// Embed the question.
	qEmb, err := provider.Embed(ctx, []string{question})
	if err != nil || len(qEmb) == 0 {
		m["Error"] = "Embedding failed (is the AI backend running?): " + errText(err)
		return c.Render("ask", m, "layout")
	}

	// Score every accessible chunk.
	chunks, err := s.db.ChunksAccessibleBy(u.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if len(chunks) == 0 {
		m["Answer"] = "You don't have any processed notes yet. Add a few and try again."
		return c.Render("ask", m, "layout")
	}
	cands := make([]rag.Candidate, 0, len(chunks))
	for _, ch := range chunks {
		score := rag.Cosine(qEmb[0], rag.DecodeEmbedding(ch.Embedding))
		cands = append(cands, rag.Candidate{NoteID: ch.NoteID, Text: ch.Text, Score: score})
	}
	top := rag.TopK(cands, askTopK)

	// Build context, one citation per distinct note (best chunk wins).
	var contextBlocks []string
	var citations []citation
	noteToNum := map[int64]int{}
	for _, ct := range top {
		if ct.Score < askMinScore {
			continue
		}
		num, ok := noteToNum[ct.NoteID]
		if !ok {
			num = len(citations) + 1
			noteToNum[ct.NoteID] = num
			title := s.noteTitle(ct.NoteID)
			citations = append(citations, citation{Num: num, NoteID: ct.NoteID, Title: title, Score: ct.Score})
		}
		contextBlocks = append(contextBlocks, fmt.Sprintf("[%d] %s", num, ct.Text))
	}
	if len(citations) == 0 {
		m["Answer"] = "I couldn't find anything relevant in your notes for that question."
		return c.Render("ask", m, "layout")
	}

	prompt := "Context from the user's notes:\n\n" + strings.Join(contextBlocks, "\n\n") +
		"\n\nQuestion: " + question
	answer, err := provider.Chat(ctx, askSystemMsg, []ai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		m["Error"] = "Answering failed: " + errText(err)
		return c.Render("ask", m, "layout")
	}

	// Count this Ollama-backed answer toward the daily quota.
	if _, usingOllama := s.db.UserAICapability(u.ID); usingOllama {
		_ = s.db.IncrementOllamaUsage(u.ID)
	}

	m["Answer"] = answer
	m["Citations"] = citations
	return c.Render("ask", m, "layout")
}

func (s *Server) noteTitle(id int64) string {
	if n, err := s.db.GetNote(id); err == nil {
		if n.Title != "" {
			return n.Title
		}
		return "(untitled)"
	}
	return "(unknown)"
}

func errText(err error) string {
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}
