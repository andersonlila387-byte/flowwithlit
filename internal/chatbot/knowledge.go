package chatbot

import (
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed knowledge/flowwithlit_kb.md
var kbRaw string

// kbChunk is one retrievable unit of the knowledge base — everything from a
// "## " or "### " heading up to (but not including) the next heading of
// either level, tagged with the line number the heading starts on so answers
// can cite exactly where the information came from.
type kbChunk struct {
	heading   string
	startLine int
	body      string // heading + content, lowercased text used for matching lives in bodyLower
	bodyLower string
}

var kbChunks = parseKB(kbRaw)

var headingRe = regexp.MustCompile(`^#{2,3}\s+(.+)$`)
var wordRe = regexp.MustCompile(`[a-z0-9]+`)

// stopWords are too common to be useful signals for matching a question to a chunk.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true, "was": true, "were": true,
	"do": true, "does": true, "did": true, "how": true, "what": true, "why": true, "when": true,
	"where": true, "who": true, "can": true, "could": true, "should": true, "would": true,
	"i": true, "my": true, "me": true, "you": true, "your": true, "it": true, "its": true,
	"to": true, "for": true, "of": true, "on": true, "in": true, "and": true, "or": true,
	"with": true, "this": true, "that": true, "have": true, "has": true, "had": true,
	"about": true, "please": true, "hi": true, "hello": true, "hey": true, "im": true,
}

func parseKB(raw string) []kbChunk {
	lines := strings.Split(raw, "\n")
	var chunks []kbChunk

	var cur *kbChunk
	for i, line := range lines {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			if cur != nil {
				cur.bodyLower = strings.ToLower(cur.body)
				chunks = append(chunks, *cur)
			}
			cur = &kbChunk{heading: m[1], startLine: i + 1, body: line + "\n"}
			continue
		}
		if cur != nil {
			cur.body += line + "\n"
		}
	}
	if cur != nil {
		cur.bodyLower = strings.ToLower(cur.body)
		chunks = append(chunks, *cur)
	}
	return chunks
}

func queryTokens(q string) []string {
	words := wordRe.FindAllString(strings.ToLower(q), -1)
	tokens := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) >= 3 && !stopWords[w] {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

// retrieveKBContext finds the knowledge-base chunk(s) whose text best matches the
// user's message and returns a formatted block citing the exact source line, ready
// to inject into the system prompt. Returns "" if nothing scores above zero.
func retrieveKBContext(userMessage string) string {
	tokens := queryTokens(userMessage)
	if len(tokens) == 0 {
		return ""
	}

	type scored struct {
		chunk kbChunk
		score int
	}
	var results []scored
	for _, c := range kbChunks {
		if c.heading == "Table of Contents" {
			continue // index only, not real content
		}
		// Skip near-empty parent headings whose real content lives in their "### "
		// subsections instead — otherwise a strong heading-text match on a mostly
		// empty "## " wrapper can crowd out a more informative result.
		contentAfterHeading := strings.TrimSpace(strings.SplitN(c.body, "\n", 2)[1])
		if len(wordRe.FindAllString(contentAfterHeading, -1)) < 10 {
			continue
		}

		headingLower := strings.ToLower(c.heading)
		score := 0
		for _, t := range tokens {
			if strings.Contains(headingLower, t) {
				score += 5 // a heading match is a much stronger relevance signal than body text
			}
			score += strings.Count(c.bodyLower, t)
		}
		if score > 0 {
			results = append(results, scored{c, score})
		}
	}
	if len(results) == 0 {
		return ""
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > 2 {
		results = results[:2] // top 2 chunks keeps the injected context small and precise
	}

	var b strings.Builder
	b.WriteString("RELEVANT DOCUMENTATION (grounded facts — prefer these over general knowledge; you may mention these come from Flowwithlit's documentation, but do not read out raw line numbers to the user):\n")
	for _, r := range results {
		fmt.Fprintf(&b, "\n[Source: flowwithlit_kb.md, line %d — \"%s\"]\n%s\n", r.chunk.startLine, r.chunk.heading, strings.TrimSpace(r.chunk.body))
	}
	return b.String()
}
