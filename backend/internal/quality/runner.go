package quality

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type Accounts interface {
	GetByID(context.Context, int64) (*service.Account, error)
}

type Prober interface {
	RunQualityProbe(context.Context, int64, string, service.QualityProbeOptions) (*service.QualityProbeResult, error)
}

type Slots interface {
	AcquireAccountSlot(context.Context, int64, int) (*service.AcquireResult, error)
}

type Runner struct {
	Store    *Store
	Accounts Accounts
	Prober   Prober
	Slots    Slots
	Location *time.Location
	cancel   context.CancelFunc
	done     chan struct{}
	wake     chan struct{}
	start    sync.Once
	stop     sync.Once
}

func NewRunner(db *sql.DB, accounts Accounts, prober Prober, slots Slots, timezone string) *Runner {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return &Runner{Store: &Store{DB: db}, Accounts: accounts, Prober: prober, Slots: slots, Location: loc, done: make(chan struct{}), wake: make(chan struct{}, 1)}
}

func (r *Runner) Start() {
	r.start.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		r.cancel = cancel
		go r.loop(ctx)
	})
}

func (r *Runner) Stop() {
	r.stop.Do(func() {
		if r.cancel != nil {
			r.cancel()
			select {
			case <-r.done:
			case <-time.After(5 * time.Second):
			}
		}
	})
}

func (r *Runner) Wake() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *Runner) loop(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	lastCleanup := time.Time{}
	for {
		if ctx.Err() != nil {
			return
		}
		if err := r.cycle(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("quality monitor cycle failed", "reason", safeReason(err))
		}
		if time.Since(lastCleanup) > time.Hour {
			cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := r.Store.Cleanup(cleanupCtx); err != nil && ctx.Err() == nil {
				slog.Warn("quality monitor cleanup failed", "reason", safeReason(err))
			}
			cancel()
			lastCleanup = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-r.wake:
		}
	}
}

func (r *Runner) cycle(ctx context.Context) error {
	scheduleCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	cfg, err := r.Store.Settings(scheduleCtx)
	if err == nil && cfg.AutoEnabled {
		err = r.schedule(scheduleCtx, cfg)
	}
	if err != nil {
		cancel()
		return err
	}
	job, err := r.Store.Claim(scheduleCtx)
	cancel()
	if err != nil || job == nil {
		return err
	}
	if err := r.execute(ctx, *job, cfg); err != nil {
		return err
	}
	r.Wake() // Drain the durable queue without waiting another 15 seconds.
	return nil
}

func (r *Runner) schedule(ctx context.Context, cfg Settings) error {
	b, err := r.Store.Budget(ctx, cfg, time.Now(), r.Location)
	if err != nil || b.Limited {
		return err
	}
	targets, err := r.Store.Targets(ctx)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if !t.Enabled || !t.AccountEnabled {
			continue
		}
		p, now := t.Effective(cfg), time.Now()
		kind := ""
		switch {
		case p.QualityEnabled && !t.NextQualityAt.After(now):
			kind = "quality"
		case p.AvailabilityEnabled && !t.NextAvailabilityAt.After(now):
			kind = "availability"
		case p.CacheEnabled && !t.NextCacheAt.After(now):
			kind = "cache"
		}
		if kind != "" {
			if _, err := r.Store.Queue(ctx, t.ID, kind, false); err != nil {
				// Concurrent schedulers and an existing manual job are expected.
				if !isConflict(err) {
					return err
				}
			}
		}
	}
	return nil
}

func isConflict(err error) bool {
	return errors.Is(err, conflict(""))
}

func (r *Runner) execute(ctx context.Context, job Run, cfg Settings) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	t, err := r.Store.Target(ctx, job.TargetID)
	if err != nil {
		return err
	}
	p := t.Effective(cfg)
	job.Result = &Result{Items: []Sample{}}
	job.Verdict = "unknown"
	next, retry := time.Now().Add(period(p, job.Kind)), 0
	finish := func() error {
		finishCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return r.Store.Finish(finishCtx, job, next, retry, t.Version, cfg.Version)
	}
	if !job.Manual && (!t.Enabled || !t.AccountEnabled || !cfg.AutoEnabled || !kindEnabled(p, job.Kind)) {
		job.Result.Reason = "disabled"
		return finish()
	}
	a, err := r.Accounts.GetByID(ctx, t.AccountID)
	if err != nil {
		job.Result.Reason = "account_unavailable"
		return finish()
	}
	if !service.QualityProbeSupported(a, t.Model) {
		job.Result.Reason = "unsupported"
		return finish()
	}
	if !job.Manual && !a.IsSchedulable() {
		job.Result.Reason = "account_paused"
		return finish()
	}
	job.ProfileHash = profileHash(a, t, p)
	if err = r.Store.Renew(ctx, job, p, job.ProfileHash); err != nil {
		return err
	}
	if job.Kind == "availability" && !job.Manual {
		m, metricErr := r.Store.Metrics(ctx, t, time.Now().Add(-15*time.Minute))
		if metricErr != nil {
			return metricErr
		}
		if m.Successes > 0 && m.Errors == 0 {
			job.Verdict, job.Result.Reason = "passed", "real_traffic"
			return finish()
		}
	}
	count := 1
	if job.Kind == "quality" {
		count = p.Questions
	} else if job.Kind == "cache" {
		count = 3
	}
	job.Result.Planned = count
	prefix := ""
	if job.Kind == "cache" {
		prefix = cachePrefix(job.ID, p.CachePrefixKB)
		job.Result.CachePrefix = prefix
	}
	for i := 0; i < count; i++ {
		current, readErr := r.Store.Target(ctx, t.ID)
		latestSettings, settingsErr := r.Store.Settings(ctx)
		if readErr != nil || settingsErr != nil {
			return errors.Join(readErr, settingsErr)
		}
		if configurationChanged(current, latestSettings, t, cfg, p, job) || ctx.Err() != nil {
			job.Result.Reason = "configuration_changed"
			break
		}
		currentAccount, accountErr := r.Accounts.GetByID(ctx, t.AccountID)
		if accountErr != nil || !service.QualityProbeSupported(currentAccount, t.Model) || profileHash(currentAccount, t, p) != job.ProfileHash {
			job.Result.Reason = "configuration_changed"
			break
		}
		if !job.Manual && !currentAccount.IsSchedulable() {
			job.Result.Reason = "account_paused"
			break
		}
		a = currentAccount
		if err = r.Store.Renew(ctx, job, p, job.ProfileHash); err != nil {
			return err
		}
		q, questionErr := newQuestion(i, job.Kind != "quality")
		if questionErr != nil {
			job.Result.Reason = "question_unavailable"
			break
		}
		opts := service.QualityProbeOptions{Prompt: q.Prompt, Prefix: prefix, ReasoningEffort: p.ReasoningEffort, MaxOutputTokens: p.MaxOutputTokens}
		if job.Kind == "cache" {
			opts.CacheKey = "quality-" + job.ID
		}
		probe, used, probeErr := r.probe(ctx, a, t.Model, opts, job.Manual)
		if probeErr != nil {
			if probe != nil {
				job.Result.Requests++
				job.Result.BudgetTokens += used
				job.Result.Items = append(job.Result.Items, Sample{QuestionID: q.ID, Family: q.Family, Prompt: q.Prompt, Expected: q.Expected, Explanation: q.Explanation, Probe: probe})
			}
			job.Result.Reason = probeErr.Error()
			if job.Result.Reason == "budget_limited" {
				next = nextDay(time.Now(), r.Location)
			}
			break
		}
		job.Result.Requests++
		job.Result.BudgetTokens += used
		job.Result.DurationMs += probe.DurationMs
		job.Result.FirstTokenMs = probe.FirstTokenMs
		sample := Sample{QuestionID: q.ID, Family: q.Family, Prompt: q.Prompt, Expected: q.Expected, Explanation: q.Explanation, Probe: probe}
		if job.Kind == "cache" && i > 0 && probe.Complete && probe.CacheReported {
			job.Result.CacheReported = true
			job.Result.CacheReadTokens += probe.CacheReadTokens
			job.Result.CacheInputTokens += probe.InputTokens
			if a.Platform == service.PlatformAnthropic {
				job.Result.CacheInputTokens += probe.CacheReadTokens + probe.CacheWriteTokens
			}
			if !probe.UsageReported || job.Result.CacheInputTokens <= 0 || job.Result.CacheReadTokens > job.Result.CacheInputTokens {
				job.Result.CacheReported = false
				job.Result.Reason = "cache_usage_invalid"
			}
		}
		if probe.Complete {
			correct := grade(probe.Text, q)
			sample.Correct = &correct
			job.Result.Graded++
			if correct {
				job.Result.Correct++
			}
		}
		job.Result.Items = append(job.Result.Items, sample)
		if !probe.Complete {
			job.Result.Reason = probe.Reason
			if job.Kind == "availability" {
				job.Verdict = "unavailable"
			}
			break
		}
		if job.Kind == "cache" && i > 0 {
			if job.Result.Reason != "" {
				break
			}
			if !probe.CacheReported {
				job.Result.Reason = "cache_unreported"
				break
			}
			if probe.CacheReadTokens > 0 {
				break
			}
		}
	}
	// An edit during the final upstream request must also invalidate the round,
	// even though there is no next sample on which to observe that edit.
	if job.Result.Reason == "" {
		current, targetErr := r.Store.Target(ctx, t.ID)
		settings, settingsErr := r.Store.Settings(ctx)
		account, accountErr := r.Accounts.GetByID(ctx, t.AccountID)
		if targetErr != nil || settingsErr != nil || accountErr != nil || configurationChanged(current, settings, t, cfg, p, job) || !service.QualityProbeSupported(account, t.Model) || profileHash(account, t, p) != job.ProfileHash {
			job.Result.Reason = "configuration_changed"
		}
	}
	if job.Result.Reason == "" {
		switch job.Kind {
		case "quality":
			if job.Result.Graded == count {
				if job.Result.Correct*100 >= p.PassPercent*count {
					job.Verdict = "passed"
				} else {
					job.Verdict = "suspected"
					history, historyErr := r.Store.History(ctx, t.ID, 30)
					if historyErr != nil {
						return historyErr
					}
					for _, previous := range history {
						if previous.Kind != "quality" || previous.ID == job.ID || previous.Status != "finished" {
							continue
						}
						if previous.ProfileHash == job.ProfileHash && (previous.Verdict == "suspected" || previous.Verdict == "degraded") {
							job.Verdict = "degraded"
						}
						break
					}
					if !job.Manual && t.RetryAttempt < p.RetryCount {
						retry, next = t.RetryAttempt+1, time.Now().Add(time.Duration(p.RetryIntervalMinutes)*time.Minute)
					}
				}
			}
		case "availability":
			if job.Result.Correct == 1 {
				job.Verdict = "passed"
			} else {
				job.Verdict, job.Result.Reason = "unavailable", "unexpected_response"
			}
		case "cache":
			if job.Result.Graded != job.Result.Requests || job.Result.Correct != job.Result.Requests {
				job.Result.Reason = "unexpected_response"
			} else {
				job.Verdict = "not_observed"
				for i, sample := range job.Result.Items {
					if i > 0 && sample.Probe.CacheReported && sample.Probe.CacheReadTokens > 0 {
						job.Verdict = "cache_hit"
					}
				}
			}
		}
	}
	return finish()
}

func kindEnabled(p Profile, kind string) bool {
	switch kind {
	case "availability":
		return p.AvailabilityEnabled
	case "quality":
		return p.QualityEnabled
	case "cache":
		return p.CacheEnabled
	default:
		return false
	}
}

func configurationChanged(current Target, settings Settings, original Target, cfg Settings, p Profile, job Run) bool {
	if job.Manual {
		effective := current.Effective(settings)
		return effective.Questions != p.Questions || effective.PassPercent != p.PassPercent ||
			effective.MaxOutputTokens != p.MaxOutputTokens || effective.ReasoningEffort != p.ReasoningEffort ||
			effective.CachePrefixKB != p.CachePrefixKB
	}
	return !current.Enabled || !current.AccountEnabled || !settings.AutoEnabled || !kindEnabled(current.Effective(settings), job.Kind) || current.Version != original.Version || settings.Version != cfg.Version
}

func nextDay(now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 5, 0, loc)
}

func (r *Runner) probe(ctx context.Context, a *service.Account, model string, opts service.QualityProbeOptions, manual bool) (*service.QualityProbeResult, int64, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if r.Slots == nil {
		return nil, 0, errors.New("capacity_unavailable")
	}
	var slot *service.AcquireResult
	var err error
	for {
		slot, err = r.Slots.AcquireAccountSlot(requestCtx, a.ID, a.Concurrency)
		if err != nil {
			return nil, 0, errors.New("capacity_unavailable")
		}
		if slot != nil && slot.Acquired {
			break
		}
		if !manual {
			return nil, 0, errors.New("capacity_unavailable")
		}
		select {
		case <-requestCtx.Done():
			return nil, 0, errors.New("capacity_unavailable")
		case <-time.After(500 * time.Millisecond):
		}
	}
	if slot.ReleaseFunc != nil {
		defer slot.ReleaseFunc()
	}
	reserved := service.QualityProbeReservation(a.Platform, opts)
	day := time.Now().In(r.Location).Format("2006-01-02")
	var ok bool
	if manual {
		ok, err = r.Store.ReserveManual(requestCtx, day, reserved)
	} else {
		ok, err = r.Store.Reserve(requestCtx, day, reserved)
	}
	if err != nil {
		return nil, 0, errors.New("budget_unavailable")
	}
	if !ok {
		return nil, 0, errors.New("budget_limited")
	}
	result, probeErr := r.Prober.RunQualityProbe(requestCtx, a.ID, model, opts)
	actual, confirmed := reserved, false
	if result != nil && result.UsageReported {
		actual = result.InputTokens + result.OutputTokens
		if a.Platform == service.PlatformAnthropic {
			actual += result.CacheReadTokens + result.CacheWriteTokens
		}
		confirmed = result.Complete
	}
	if !confirmed {
		actual = max(actual, reserved)
	}
	settleCtx, settleCancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = r.Store.settle(settleCtx, day, reserved, actual, confirmed, manual)
	settleCancel()
	if err != nil {
		return result, reserved, errors.New("budget_unavailable")
	}
	if probeErr != nil || result == nil {
		result = &service.QualityProbeResult{Reason: "transport_error"}
	}
	return result, actual, nil
}
