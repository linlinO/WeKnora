package evolution

import "testing"

func TestQuestionHashNormalizesPunctuation(t *testing.T) {
	if QuestionHash("服务器负载高怎么定位？") != QuestionHash(" 服务器负载高怎么定位 ") {
		t.Fatal("equivalent questions should have the same hash")
	}
}

func TestTokenCoverage(t *testing.T) {
	if got := TokenCoverage("先检查 CPU 和 IO", "先检查 CPU 和 IO，再看内存"); got < 0.9 {
		t.Fatalf("coverage = %v, want high coverage", got)
	}
	if got := TokenCoverage("必须重启服务", "查看日志即可"); got >= 0.5 {
		t.Fatalf("coverage = %v, want low coverage", got)
	}
}
