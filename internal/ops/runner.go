// Package ops manages single-flight lifecycle operations, ring-buffered progress, and SSE fanout.
package ops

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/env"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

const (
	maxRingLines   = 200
	maxJobHistory  = 20
	subBufferDepth = 64
)

// Sentinel errors returned by Runner validation and locking.
var (
	ErrBusy           = errors.New("an operation is already in progress")
	ErrUnknownService = errors.New("unknown service")
	ErrInvalidAction  = errors.New("invalid lifecycle action")
	ErrServiceLocked  = errors.New("service is locked or unconfigured")
	ErrPrereqFailed   = errors.New("prerequisite checks failed")
)

// Job status values.
const (
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// Job describes a single or batch lifecycle operation.
type Job struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Service    string     `json:"service,omitempty"`
	Action     string     `json:"action"`
	Up         []string   `json:"up,omitempty"`
	Down       []string   `json:"down,omitempty"`
	Status     string     `json:"status"`
	Message    string     `json:"message,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

// Event is pushed to SSE subscribers for progress lines and job completion.
type Event struct {
	Type  string `json:"type"`
	JobID string `json:"jobId"`
	Line  string `json:"line,omitempty"`
	Job   *Job   `json:"job,omitempty"`
}

// StatusSnapshot is a point-in-time view of runner state, buffered lines, and recent jobs.
type StatusSnapshot struct {
	Busy    bool     `json:"busy"`
	Current *Job     `json:"current,omitempty"`
	Lines   []string `json:"lines"`
	History []Job    `json:"history"`
}

// DockerClient combines the Docker SDK interfaces needed by the ops runner.
type DockerClient interface {
	docker.ContainerInspector
	docker.NetworkManager
}

// Config configures a Runner instance.
type Config struct {
	RepoPath                   string
	Identity                   stack.Identity
	ActiveServices             []string
	Docker                     DockerClient
	Compose                    docker.ComposeRunner
	ReadyTimeout, PollInterval time.Duration
}

// Runner serializes mutating lifecycle operations and fans out masked progress to SSE clients.
type Runner struct {
	cfg          Config
	mu           sync.Mutex
	current      *Job
	lines        []string
	history      []Job
	subs         map[uint64]chan Event
	nextSub, seq uint64
}

// NewRunner creates a Runner with default readiness polling intervals when unset.
func NewRunner(cfg Config) *Runner {
	cfg.ReadyTimeout = cmp.Or(cfg.ReadyTimeout, docker.DefaultReadyTimeout)
	cfg.PollInterval = cmp.Or(cfg.PollInterval, docker.DefaultPollInterval)
	if cfg.Compose == nil {
		cfg.Compose = docker.ExecComposeRunner{}
	}
	return &Runner{cfg: cfg, subs: make(map[uint64]chan Event)}
}

// Status returns a snapshot of the runner state, ring buffer lines, and job history.
func (r *Runner) Status() StatusSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

// Subscribe registers an SSE listener and returns its ID, event channel, and initial snapshot.
func (r *Runner) Subscribe() (uint64, <-chan Event, StatusSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextSub++
	ch := make(chan Event, subBufferDepth)
	r.subs[r.nextSub] = ch
	return r.nextSub, ch, r.snapshotLocked()
}

// Unsubscribe removes an SSE listener and closes its channel.
func (r *Runner) Unsubscribe(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch, ok := r.subs[id]; ok {
		delete(r.subs, id)
		close(ch)
	}
}

func (r *Runner) snapshotLocked() StatusSnapshot {
	var cur *Job
	if r.current != nil {
		cp := *r.current
		cur = &cp
	}
	return StatusSnapshot{Busy: r.current != nil, Current: cur, Lines: slices.Clone(r.lines), History: slices.Clone(r.history)}
}

func (r *Runner) validateServices(services []string, checkLock, checkPrereq bool) error {
	for _, s := range services {
		if !slices.Contains(r.cfg.ActiveServices, s) {
			return fmt.Errorf("%w: %s", ErrUnknownService, s)
		}
		if checkLock {
			if code, reason := env.LabelOnlyReason(r.cfg.RepoPath, s); code != "" {
				return fmt.Errorf("%w: %s", ErrServiceLocked, reason)
			}
		}
	}
	if checkPrereq && len(services) > 0 {
		if probs := env.PrerequisiteProblems(r.cfg.RepoPath, services); len(probs) > 0 {
			return fmt.Errorf("%w: %s", ErrPrereqFailed, probs[0].Message)
		}
	}
	return nil
}

// StartSingle validates and starts a single-service lifecycle operation asynchronously.
func (r *Runner) StartSingle(ctx context.Context, service, action string) (Job, error) {
	if !slices.Contains([]string{"up", "down", "stop", "restart"}, action) {
		return Job{}, fmt.Errorf("%w: %s", ErrInvalidAction, action)
	}
	if err := r.validateServices([]string{service}, action == "up" || action == "restart", action == "up"); err != nil {
		return Job{}, err
	}
	return r.launchJob(Job{Kind: "single", Service: service, Action: action}, []string{service}, fmt.Sprintf("%s %s completed", service, action), func(say func(string)) error {
		if action == "up" {
			return r.runBatch(ctx, []string{service}, nil, say)
		}
		return r.cfg.Compose.RunCompose(ctx, r.cfg.RepoPath, r.cfg.Identity.ProjectName(service), service, []string{action}, say)
	})
}

// StartBatch validates and starts a batch apply operation (down then up) asynchronously.
func (r *Runner) StartBatch(ctx context.Context, up, down []string) (Job, error) {
	if err := errors.Join(r.validateServices(up, true, true), r.validateServices(down, false, false)); err != nil {
		return Job{}, err
	}
	orderedUp, orderedDown := docker.StartupQueue(up), docker.ShutdownQueue(down)
	targets := append(slices.Clone(orderedUp), orderedDown...)
	return r.launchJob(Job{Kind: "apply", Action: "apply", Up: orderedUp, Down: orderedDown}, targets, "All changes applied successfully", func(say func(string)) error {
		return r.runBatch(ctx, orderedUp, orderedDown, say)
	})
}

func (r *Runner) launchJob(template Job, services []string, okMsg string, fn func(say func(string)) error) (Job, error) {
	r.mu.Lock()
	if r.current != nil {
		r.mu.Unlock()
		return Job{}, ErrBusy
	}
	r.seq++
	template.ID, template.Status, template.StartedAt = fmt.Sprintf("op-%d", r.seq), StatusRunning, time.Now().UTC()
	cp := template
	r.current = &cp
	r.mu.Unlock()

	go func() {
		maskTargets := slices.Clone(services)
		for _, s := range services {
			maskTargets = append(maskTargets, docker.DependencyQueue(s, r.cfg.ActiveServices)...)
		}
		values := env.ValuesToMask(r.cfg.RepoPath, maskTargets)
		say := func(line string) { r.appendLine(template.ID, env.MaskKnownValues(line, values)) }
		if err := fn(say); err != nil {
			r.finishJob(template.ID, false, env.MaskKnownValues(err.Error(), values))
			return
		}
		r.finishJob(template.ID, true, okMsg)
	}()
	return template, nil
}

func (r *Runner) appendLine(jobID, line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, line)
	if len(r.lines) > maxRingLines {
		r.lines = slices.Clone(r.lines[len(r.lines)-maxRingLines:])
	}
	r.broadcastLocked(Event{Type: "line", JobID: jobID, Line: line})
}

func (r *Runner) finishJob(jobID string, ok bool, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	finished := *r.current
	finished.FinishedAt, finished.Message, finished.Status = &now, message, StatusFailed
	if ok {
		finished.Status = StatusSucceeded
	}
	r.current = nil
	r.history = append([]Job{finished}, r.history...)
	if len(r.history) > maxJobHistory {
		r.history = r.history[:maxJobHistory]
	}
	r.broadcastLocked(Event{Type: "done", JobID: jobID, Job: &finished})
}

func (r *Runner) broadcastLocked(ev Event) {
	for _, ch := range r.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (r *Runner) runBatch(ctx context.Context, up, down []string, say func(string)) error {
	var failed []string
	for _, svc := range down {
		say(fmt.Sprintf("Stopping and removing %s...", svc))
		if err := r.cfg.Compose.RunCompose(ctx, r.cfg.RepoPath, r.cfg.Identity.ProjectName(svc), svc, []string{"down"}, say); err != nil {
			failed = append(failed, svc)
		}
	}
	if len(up) > 0 {
		if err := docker.EnsureNetworks(ctx, r.cfg.Docker, r.cfg.Identity.Networks, say); err != nil {
			return err
		}
		for _, svc := range up {
			for _, dep := range docker.DependencyQueue(svc, r.cfg.ActiveServices) {
				if _, ready, err := docker.ServiceContainersReady(ctx, r.cfg.Docker, r.cfg.Identity.Prefix, dep); err != nil {
					failed = append(failed, svc)
					return fmt.Errorf("completed with errors: %s: %w", strings.Join(failed, ", "), err)
				} else if ready {
					continue
				}
				say(fmt.Sprintf("Queuing %s before %s...", dep, svc))
				if probs := env.PrerequisiteProblems(r.cfg.RepoPath, []string{dep}); len(probs) > 0 {
					return fmt.Errorf("%w: %s", ErrPrereqFailed, probs[0].Message)
				}
				if err := r.cfg.Compose.RunCompose(ctx, r.cfg.RepoPath, r.cfg.Identity.ProjectName(dep), dep, []string{"up", "-d"}, say); err != nil {
					return err
				}
				if err := docker.WaitUntilReady(ctx, r.cfg.Docker, r.cfg.Identity.Prefix, dep, r.cfg.ReadyTimeout, r.cfg.PollInterval, say); err != nil {
					return err
				}
			}
			say(fmt.Sprintf("Starting %s...", svc))
			if err := r.cfg.Compose.RunCompose(ctx, r.cfg.RepoPath, r.cfg.Identity.ProjectName(svc), svc, []string{"up", "-d"}, say); err != nil || docker.WaitUntilOneshotsDone(ctx, r.cfg.Docker, r.cfg.Identity.Prefix, svc, r.cfg.ReadyTimeout, r.cfg.PollInterval, say) != nil {
				failed = append(failed, svc)
				break
			}
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("completed with errors: %s", strings.Join(failed, ", "))
	}
	return nil
}
