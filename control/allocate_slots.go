package control

import (
	"errors"
	"fmt"
	"sync"
)

var ErrNoAvailableSlots = errors.New("no available sandbox IP slots (range 1-250 exhausted)")

type SlotAllocator struct {
	mu       sync.Mutex
	maxSlots int
	slotToID map[int]string
	idToSlot map[string]int
}

func NewSlotAllocator(maxSlots int) *SlotAllocator {
	if maxSlots <= 0 || maxSlots > 250 {
		maxSlots = 250
	}
	return &SlotAllocator{
		maxSlots: maxSlots,
		slotToID: make(map[int]string, maxSlots),
		idToSlot: make(map[string]int, maxSlots),
	}
}

func (a *SlotAllocator) Allocate(sandboxID string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if slot, exists := a.idToSlot[sandboxID]; exists {
		return slot, nil
	}

	for i := 1; i <= a.maxSlots; i++ {
		if _, occupied := a.slotToID[i]; !occupied {
			a.slotToID[i] = sandboxID
			a.idToSlot[sandboxID] = i
			return i, nil
		}
	}

	return 0, fmt.Errorf("%w: %d slots in use", ErrNoAvailableSlots, len(a.slotToID))
}

func (a *SlotAllocator) Release(sandboxID string) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	slot, exists := a.idToSlot[sandboxID]
	if !exists {
		return 0
	}

	delete(a.idToSlot, sandboxID)
	delete(a.slotToID, slot)
	return slot
}

func (a *SlotAllocator) GetSlot(sandboxID string) (int, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	slot, exists := a.idToSlot[sandboxID]
	return slot, exists
}

// Occupy explicitly marks a specific slot as occupied by sandboxID (used for state reconciliation).
func (a *SlotAllocator) Occupy(sandboxID string, slot int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if slot < 1 || slot > a.maxSlots {
		return fmt.Errorf("slot %d out of bounds (1-%d)", slot, a.maxSlots)
	}

	if curSlot, exists := a.idToSlot[sandboxID]; exists {
		if curSlot == slot {
			return nil
		}
		return fmt.Errorf("sandbox %s already occupies slot %d", sandboxID, curSlot)
	}

	if occupyingID, exists := a.slotToID[slot]; exists {
		return fmt.Errorf("slot %d already occupied by %s", slot, occupyingID)
	}

	a.slotToID[slot] = sandboxID
	a.idToSlot[sandboxID] = slot
	return nil
}
