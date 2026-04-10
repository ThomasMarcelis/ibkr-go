package session

import (
	"testing"
	"time"
)

func TestOrderHandleStateChannelClosesWhenFull(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(7)
	for i := 0; i < 12; i++ {
		handle.emitState(SubscriptionStateEvent{Kind: SubscriptionGap, ConnectionSeq: uint64(i + 1)})
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var seqs []uint64
	timeout := time.After(time.Second)
	for {
		select {
		case evt, ok := <-handle.State():
			if !ok {
				if len(seqs) != 8 {
					t.Fatalf("gap event count = %d, want 8", len(seqs))
				}
				for i, seq := range seqs {
					want := uint64(i + 5)
					if seq != want {
						t.Fatalf("seqs[%d] = %d, want %d (keep latest 8)", i, seq, want)
					}
				}
				return
			}
			if evt.Kind == SubscriptionGap {
				seqs = append(seqs, evt.ConnectionSeq)
			}
		case <-timeout:
			t.Fatal("State() channel did not close")
		}
	}
}
