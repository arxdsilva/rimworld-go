package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// skip is a sound the run leaves alone, and why.
type skip struct{ id, why string }

// decide splits the selection into what to generate and what to skip. The
// money rules live here: an approved or still-unheard sound is never
// regenerated, and a rejected one gets exactly one retry (-retry).
func decide(sel []Sound, l *Ledger, retry bool) (todo []Sound, skips []skip) {
	for _, s := range sel {
		rejected := 0
		why := ""
		for _, i := range l.forID(s.ID) {
			switch l.Takes[i].state() {
			case stateApproved:
				why = "already approved"
			case statePending:
				if why == "" {
					why = "take waiting for review"
				}
			case stateRejected:
				rejected++
			}
		}
		switch {
		case why != "":
		case rejected >= 2:
			why = "rejected twice: use the group sound"
		case rejected == 1 && !retry:
			why = "rejected once: -retry makes the one allowed retry"
		case rejected == 0 && retry:
			why = "-retry only repeats rejected sounds"
		}
		if why != "" {
			skips = append(skips, skip{s.ID, why})
		} else {
			todo = append(todo, s)
		}
	}
	return todo, skips
}

// remaining is the credits a run may still spend before the reserve: the
// budget less the ledger, and when the API counter is read, the smaller of
// that and the API's own headroom.
func (a *app) remaining(l *Ledger, sub *subscription) int {
	r := a.cfg.budget - l.spent() - a.cfg.reserve
	if sub != nil {
		r = min(r, sub.Limit-sub.Used-a.cfg.reserve)
	}
	return r
}

func (a *app) generate() error {
	p, err := loadPrompts(a.cfg.prompts)
	if err != nil {
		return err
	}
	sel, err := selectSounds(p.Sounds, a.cfg.only)
	if err != nil {
		return err
	}
	if a.cfg.seconds > 0 {
		for i := range sel {
			sel[i].Seconds = a.cfg.seconds
		}
	}
	l, err := loadLedger(a.cfg.ledger)
	if err != nil {
		return err
	}
	todo, skips := decide(sel, l, a.cfg.retry)
	ctx := context.Background()
	var sub subscription
	var subp *subscription
	startUsed := 0
	if a.cfg.budgetFromAPI {
		if a.key == "" {
			return errors.New(keyEnv + " is not set (needed for -budget-from-api)")
		}
		if sub, err = a.subscription(ctx); err != nil {
			return err
		}
		subp = &sub
		startUsed = sub.Used
	}
	if a.cfg.dryRun {
		a.dryRun(todo, skips, l, a.remaining(l, subp))
		return nil
	}
	if a.key == "" {
		return errors.New(keyEnv + " is not set")
	}
	for _, k := range skips {
		fmt.Fprintf(a.stdout, "skip %s: %s\n", k.id, k.why)
	}
	spent, made := 0, 0
	for _, s := range todo {
		est := estimate(s.Seconds, a.cfg.creditRate)
		remaining := a.remaining(l, subp)
		if a.cfg.maxCredits > 0 && spent+est > a.cfg.maxCredits {
			fmt.Fprintf(a.stdout, "stop: %s would take this run to %d credits, over -max-credits %d\n", s.ID, spent+est, a.cfg.maxCredits)
			break
		}
		if est > remaining {
			fmt.Fprintf(a.stdout, "stop: %s needs about %d credits but only %d remain before the %d reserve\n", s.ID, est, remaining, a.cfg.reserve)
			break
		}
		audio, err := a.synthesize(ctx, p.text(s), s.Seconds, p.Defaults.PromptInfluence)
		if err != nil {
			return fmt.Errorf("%s: %w", s.ID, err)
		}
		credits := est
		if a.cfg.budgetFromAPI {
			after, err := a.subscription(ctx)
			if err != nil {
				return fmt.Errorf("%s: %w", s.ID, err)
			}
			// The counter lags and then jumps by several takes at once, some
			// already logged at their estimate: charge only what the counter
			// shows beyond this run's ledger, never less than the estimate.
			if after.Used == sub.Used {
				fmt.Fprintf(a.stdout, "note: the API counter did not move for %s (it lags); logged the estimate %d\n", s.ID, est)
			}
			credits = max(est, after.Used-startUsed-spent)
			sub = after
		}
		n := l.nextTake(s.ID)
		raw := filepath.Join(a.cfg.out, fmt.Sprintf("%s_t%d.mp3", s.ID, n))
		if err := os.MkdirAll(a.cfg.out, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(raw, audio, 0o644); err != nil {
			return err
		}
		l.Takes = append(l.Takes, Take{ID: s.ID, Take: n, File: a.ledgerPath(raw), Tool: "text_to_sound_effects",
			Seconds: s.Seconds, Credits: credits, Date: a.now().Format("2006-01-02"), Prompt: p.text(s)})
		if err := l.save(a.cfg.ledger); err != nil {
			return err
		}
		spent += credits
		made++
		fmt.Fprintf(a.stdout, "%s take %d: %.1f s, %d credits -> %s\n", s.ID, n, s.Seconds, credits, a.ledgerPath(raw))
	}
	fmt.Fprintf(a.stdout, "done: %d takes, %d credits this run, ledger total %d credits\n", made, spent, l.spent())
	return nil
}

// ledgerPath is raw as the ledger records it: relative to the ledger's
// directory when it sits below it.
func (a *app) ledgerPath(raw string) string {
	if rel, err := filepath.Rel(filepath.Dir(a.cfg.ledger), raw); err == nil && filepath.IsLocal(rel) {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(raw)
}

func (a *app) dryRun(todo []Sound, skips []skip, l *Ledger, remaining int) {
	total, secs := 0, 0.0
	for _, s := range todo {
		est := estimate(s.Seconds, a.cfg.creditRate)
		fmt.Fprintf(a.stdout, "%-40s %4.1f s %4d credits\n", s.ID, s.Seconds, est)
		total += est
		secs += s.Seconds
	}
	for _, k := range skips {
		fmt.Fprintf(a.stdout, "skip %s: %s\n", k.id, k.why)
	}
	fmt.Fprintf(a.stdout, "total: %d sounds, %.1f s, %d credits (at %g credits a second)\n", len(todo), secs, total, a.cfg.creditRate)
	fmt.Fprintf(a.stdout, "budget: %d - %d already spent in the ledger - %d reserve = %d usable; this plan leaves %d\n",
		a.cfg.budget, l.spent(), a.cfg.reserve, remaining, remaining-total)
	if a.cfg.maxCredits > 0 {
		fmt.Fprintf(a.stdout, "-max-credits %d would stop the run early if the plan costs more\n", a.cfg.maxCredits)
	}
}
