package tokencount

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

type failingOnceLoader struct{ calls atomic.Int32 }

func (l *failingOnceLoader) LoadTiktokenBpe(string) (map[string]int, error) {
	if l.calls.Add(1) == 1 {
		return nil, fmt.Errorf("injected initial failure")
	}
	ranks := make(map[string]int, 256)
	for i := range 256 {
		ranks[string([]byte{byte(i)})] = i
	}
	return ranks, nil
}

func TestCountBytes_RecoveryRace(t *testing.T) {
	// Run in a fresh process so the dependency's global encoding cache is empty.
	if os.Getenv("CLAUDE_KIRO_TOKENIZER_RECOVERY_TEST") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestCountBytes_RecoveryRace$")
		cmd.Env = append(os.Environ(), "CLAUDE_KIRO_TOKENIZER_RECOVERY_TEST=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("recovery process: %v\n%s", err, out)
		}
		return
	}
	loader := &failingOnceLoader{}
	tiktoken.SetBpeLoader(loader)
	if _, err := CountBytes([]byte("hello")); err == nil {
		t.Fatal("initial load must fail")
	}
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			for range 100 {
				if n, err := CountBytes([]byte("hello")); err != nil || n <= 0 {
					t.Errorf("recovery count=%d err=%v", n, err)
					return
				}
			}
		})
	}
	wg.Wait()
	if loader.calls.Load() != 2 {
		t.Fatalf("loader calls=%d, want 2", loader.calls.Load())
	}
}
