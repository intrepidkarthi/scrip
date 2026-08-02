package scrip

import (
	"context"
	"sync"
	"time"
)

// memStore is an in-memory [Store]: the reference against which a real implementation
// can be compared, and what the tests run on.
//
// It is not a toy. It enforces every rule the interface documents, including the
// atomicity of drawdown-and-mint — under one mutex here, under one transaction in a
// database. An implementation that passes the same tests as this one has implemented
// the protocol; one that does not, has not.
//
// It is deliberately not exported. A protocol that ships a convenient in-memory store
// gets used with it in production by somebody, and losing the register on restart is
// not a failure mode worth making easy.
type memStore struct {
	mu          sync.Mutex
	instruments map[string]Instrument
	auths       map[string]*Authorization
	mints       map[string]Mint
	settlements map[string]*Settlement
	// issued tracks outstanding units per instrument — mints less burns.
	issued map[string]uint64
	// anchors and salts are kept apart because they have different exposure: the anchor
	// is public by construction, the salt is disclosed only to a party entitled to
	// verify. A real implementation should hold the salt wherever it holds secrets.
	anchors map[string]Anchor
	salts   map[string][SaltLen]byte
}

func newMemStore() *memStore {
	return &memStore{
		instruments: map[string]Instrument{},
		auths:       map[string]*Authorization{},
		mints:       map[string]Mint{},
		settlements: map[string]*Settlement{},
		issued:      map[string]uint64{},
		anchors:     map[string]Anchor{},
		salts:       map[string][SaltLen]byte{},
	}
}

func (m *memStore) Instrument(_ context.Context, id string) (Instrument, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.instruments[id]
	if !ok {
		return Instrument{}, ErrNotFound
	}
	return i, nil
}

func (m *memStore) SaveAuthorization(_ context.Context, a *Authorization) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auths[a.ID] = a
	return nil
}

func (m *memStore) Authorization(_ context.Context, id string) (*Authorization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.auths[id]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

func (m *memStore) CounterSign(_ context.Context, id string, by Party, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.auths[id]
	if !ok {
		return ErrNotFound
	}
	// Under the lock, so two concurrent counter-signatures cannot both succeed.
	return a.CounterSign(by, at)
}

// DrawAndMint is the method that matters. The drawdown and the mint record happen under
// one lock and either both apply or neither does — the in-memory equivalent of one
// transaction. Splitting them is the race the protocol exists to prevent.
func (m *memStore) DrawAndMint(_ context.Context, mint Mint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.auths[mint.AuthorizationID]
	if !ok {
		return ErrNotFound
	}

	inst, ok := m.instruments[a.InstrumentID]
	if !ok {
		return ErrNotFound
	}

	// Re-check the reserve ceiling here, not only in Issue. A Store must enforce the
	// invariants itself: Issue's check gives the caller a good error, this one gives
	// the system its guarantee, and only this one holds under concurrency.
	if ceiling, bounded := inst.Profile.reserveCeiling(a.Attestations); bounded {
		if m.issued[a.InstrumentID]+mint.Quantity > ceiling {
			return ErrReserveShortfall
		}
	}

	if err := a.Draw(mint.Quantity); err != nil {
		return err
	}

	m.mints[mint.ID] = mint
	m.issued[a.InstrumentID] += mint.Quantity
	return nil
}

func (m *memStore) TotalIssued(_ context.Context, instrumentID string) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.issued[instrumentID], nil
}

func (m *memStore) SaveSettlement(_ context.Context, s *Settlement) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Exactly-once on terminal transitions. A settlement that reached a terminal state
	// cannot be overwritten with a different one — the in-memory equivalent of the
	// partial unique index a database would use.
	if prev, ok := m.settlements[s.ID]; ok && prev.IsTerminal() && prev.State != s.State {
		switch prev.State {
		case Settled:
			return ErrAlreadySettled
		case Unwound:
			return ErrAlreadyUnwound
		case Indeterminate:
			// Resolution is the one legitimate way out, and it is how a settlement
			// leaves Indeterminate for Settled or Unwound.
			if s.State != Settled && s.State != Unwound {
				return ErrIndeterminate
			}
		}
	}

	m.settlements[s.ID] = s
	return nil
}

func (m *memStore) Settlement(_ context.Context, id string) (*Settlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.settlements[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (m *memStore) SaveAnchor(_ context.Context, a Anchor, salt [SaltLen]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// One commitment per authorisation. A second would leave a verifier no way to know
	// which digest is authoritative.
	if _, exists := m.anchors[a.AuthorizationID]; exists {
		return ErrAlreadyAnchored
	}
	m.anchors[a.AuthorizationID] = a
	m.salts[a.AuthorizationID] = salt
	return nil
}

func (m *memStore) Anchor(_ context.Context, authorizationID string) (Anchor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.anchors[authorizationID]
	if !ok {
		return Anchor{}, ErrNotAnchored
	}
	return a, nil
}

// salt returns the retained salt, for tests and for disclosure to a verifier.
func (m *memStore) salt(authorizationID string) ([SaltLen]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.salts[authorizationID]
	return s, ok
}
