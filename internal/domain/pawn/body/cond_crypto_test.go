package body

import "testing"

// Cryptosleep sickness lowers consciousness, moving and manipulation by
// the wiki's 20/10/10 %, vomits every 3 hours on average and wears off
// after 10000 ticks.
func TestCryptosleepSickness(t *testing.T) {
	b := &Body{}
	CryptoSicken(b, 100)
	if !CryptoSick(b) || VomitEvery(b) != CryptoVomitEvery {
		t.Fatal("no sickness after waking")
	}
	if StepCryptoSickness(b, 100+CryptoSickTicks/2); !CryptoSick(b) {
		t.Fatal("gone half way")
	}
	if c, _ := b.Cond(CondCryptosleepSickness); c.Sev != 500 {
		t.Fatalf("severity %d half way, want 500", c.Sev)
	}
	if !StepCryptoSickness(b, 100+CryptoSickTicks) || CryptoSick(b) || VomitEvery(b) != 0 {
		t.Fatal("still sick after 10000 ticks")
	}
}
