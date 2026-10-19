package evolution

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

var nonWordEvolution = regexp.MustCompile(`[\p{P}\p{S}\p{Z}\s]+`)

// NormalizeQuestion provides a stable key for exact feedback grouping. It is
// intentionally conservative: semantic similarity is handled by the review
// page, while this key only prevents duplicate proposals for the same wording.
func NormalizeQuestion(question string) string {
	q := strings.ToLower(strings.TrimSpace(question))
	q = nonWordEvolution.ReplaceAllString(q, " ")
	return strings.Join(strings.Fields(q), " ")
}

func QuestionHash(question string) string {
	sum := sha256.Sum256([]byte(NormalizeQuestion(question)))
	return hex.EncodeToString(sum[:])
}

func TokenSet(text string) map[rune]struct{} {
	set := make(map[rune]struct{})
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r) {
			set[r] = struct{}{}
		}
	}
	return set
}

// TokenCoverage is a deterministic, provider-independent regression metric.
// It is deliberately used as a first gate; administrators can still inspect
// the answer and override a failed normal-severity case with an audit note.
func TokenCoverage(expected, actual string) float64 {
	want := TokenSet(expected)
	if len(want) == 0 {
		return 1
	}
	got := TokenSet(actual)
	hit := 0
	for token := range want {
		if _, ok := got[token]; ok {
			hit++
		}
	}
	return float64(hit) / float64(len(want))
}
