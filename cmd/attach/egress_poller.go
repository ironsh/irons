package attach

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/ironsh/irons/api"
)

const pollInterval = 5 * time.Second

// DomainEntry holds aggregated egress data for a single domain/host.
type DomainEntry struct {
	Host         string
	RequestCount int
	LastSeen     time.Time
	Verdict      string // "allowed", "blocked", "warn"
	Protocol     string
	RuleID       string // non-empty if an allow rule exists for this host
	Events       []api.EgressAuditEvent
}

// EgressStore is a thread-safe store of aggregated egress data.
type EgressStore struct {
	mu      sync.RWMutex
	domains map[string]*DomainEntry
	rules   map[string]*api.EgressRule // keyed by host
}

// NewEgressStore creates an empty store.
func NewEgressStore() *EgressStore {
	return &EgressStore{
		domains: make(map[string]*DomainEntry),
		rules:   make(map[string]*api.EgressRule),
	}
}

// Ingest adds audit events to the store, updating counts and timestamps.
func (s *EgressStore) Ingest(events []api.EgressAuditEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ev := range events {
		d, ok := s.domains[ev.Host]
		if !ok {
			d = &DomainEntry{Host: ev.Host}
			s.domains[ev.Host] = d
		}
		d.RequestCount++
		if ev.Timestamp.After(d.LastSeen) {
			d.LastSeen = ev.Timestamp
			d.Verdict = normalizeVerdict(ev)
			d.Protocol = ev.Protocol
		}
		// Keep last 50 events per domain for detail view.
		d.Events = append(d.Events, ev)
		if len(d.Events) > 50 {
			d.Events = d.Events[len(d.Events)-50:]
		}
	}
}

// UpdateRules replaces the rule index, updating domain entries with rule IDs.
func (s *EgressStore) UpdateRules(rules []api.EgressRule) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rules = make(map[string]*api.EgressRule, len(rules))
	for i := range rules {
		r := &rules[i]
		if r.Host != "" {
			s.rules[r.Host] = r
		}
	}

	// Update rule IDs on domain entries.
	for host, d := range s.domains {
		if r, ok := s.rules[host]; ok {
			d.RuleID = r.ID
		} else {
			d.RuleID = ""
		}
	}
}

// Snapshot returns a sorted copy of all domain entries (most recent first).
func (s *EgressStore) Snapshot() []DomainEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]DomainEntry, 0, len(s.domains))
	for _, d := range s.domains {
		entries = append(entries, *d)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastSeen.After(entries[j].LastSeen)
	})
	return entries
}

// RuleIDForHost returns the egress rule ID for a host, or empty string.
func (s *EgressStore) RuleIDForHost(host string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.rules[host]; ok {
		return r.ID
	}
	return ""
}

func normalizeVerdict(ev api.EgressAuditEvent) string {
	v := ev.Verdict
	if v == "" {
		if ev.Allowed {
			return "allowed"
		}
		return "blocked"
	}
	return v
}

// Poller periodically fetches egress audit data and rules from the API.
// It uses the `since` parameter to limit events to those from the session
// start, and drains all pages before waiting for the next poll interval.
type Poller struct {
	client      *api.Client
	vmID        string
	store       *EgressStore
	notify      chan struct{}
	refreshCh   chan struct{}
	since       string // RFC3339 timestamp of session start
	cursor      string
	lastEventID string // last event ID ingested, for deduping partial pages
}

// NewPoller creates a new egress poller scoped to events from now onward.
func NewPoller(client *api.Client, vmID string, store *EgressStore) *Poller {
	return &Poller{
		client:    client,
		vmID:      vmID,
		store:     store,
		since:     time.Now().UTC().Format(time.RFC3339),
		notify:    make(chan struct{}, 1),
		refreshCh: make(chan struct{}, 1),
	}
}

// Subscribe returns a channel that receives a signal when new data is available.
func (p *Poller) Subscribe() <-chan struct{} {
	return p.notify
}

// Refresh triggers an immediate poll cycle (non-blocking).
func (p *Poller) Refresh() {
	select {
	case p.refreshCh <- struct{}{}:
	default:
	}
}

// Start begins polling in a loop until the context is cancelled.
func (p *Poller) Start(ctx context.Context) {
	// Drain all existing pages immediately.
	p.drainPages()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.drainPages()
		case <-p.refreshCh:
			p.drainPages()
		}
	}
}

// drainPages fetches pages of audit events until has_more is false,
// then fetches current rules and notifies subscribers.
func (p *Poller) drainPages() {
	firstPage := true
	for {
		params := api.AuditEgressParams{
			VMID:   p.vmID,
			Since:  p.since,
			Cursor: p.cursor,
		}
		resp, err := p.client.AuditEgress(params)
		if err != nil {
			break
		}

		events := resp.Data

		// On the first page of a drain cycle, the cursor may point to a
		// page we partially ingested last time. Skip events up to and
		// including the last one we already ingested.
		if firstPage && p.lastEventID != "" && len(events) > 0 {
			events = p.trimSeen(events)
			firstPage = false
		}

		if len(events) > 0 {
			p.store.Ingest(events)
			p.lastEventID = events[len(events)-1].ID
		}

		if resp.Cursor != nil {
			p.cursor = *resp.Cursor
		}

		if !resp.HasMore {
			break
		}
	}

	// Fetch current rules.
	rulesResp, err := p.client.EgressListRules()
	if err == nil {
		p.store.UpdateRules(rulesResp.Data)
	}

	// Notify subscribers (non-blocking).
	select {
	case p.notify <- struct{}{}:
	default:
	}
}

// trimSeen returns events after the last ingested event ID. If the ID is not
// found in the slice, all events are returned (the page has rolled over).
func (p *Poller) trimSeen(events []api.EgressAuditEvent) []api.EgressAuditEvent {
	for i, ev := range events {
		if ev.ID == p.lastEventID {
			return events[i+1:]
		}
	}
	return events
}
