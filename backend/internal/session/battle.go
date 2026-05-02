package session

import "sync"

type Instance struct {
	userID int64
	mu     sync.RWMutex
	active bool
}

func NewBattleInstance(userID int64) *Instance {
	return &Instance{userID: userID}
}

func (b *Instance) Active() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.active
}

func (b *Instance) SetActive(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.active = v
}
