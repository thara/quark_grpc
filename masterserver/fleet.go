package masterserver

import (
	"errors"
	"sort"
	"sync"

	"github.com/google/uuid"

	"quark"
)

var (
	ErrNotEnoughGameServers = errors.New("not enough game servers")
	ErrRoomAlreadyAllocated = errors.New("room already allocated")
	ErrRoomStatusNotFound   = errors.New("room status not found")
)

type Fleet struct {
	rs map[quark.RoomID]*RoomStatus
	rg map[quark.RoomID]*GameServer
	g  []*GameServer

	allocListeners map[chan<- RoomAllocatedEvent]bool

	mux sync.RWMutex
}

func NewFleet() *Fleet {
	return &Fleet{
		rs:             make(map[quark.RoomID]*RoomStatus),
		rg:             make(map[quark.RoomID]*GameServer),
		g:              make([]*GameServer, 0),
		allocListeners: make(map[chan<- RoomAllocatedEvent]bool),
	}
}

func (f *Fleet) AddRoomAllocationListener(c chan<- RoomAllocatedEvent) {
	f.mux.Lock()
	defer f.mux.Unlock()

	f.allocListeners[c] = true
}

func (f *Fleet) RemoveRoomAllocationListener(c chan<- RoomAllocatedEvent) {
	f.mux.Lock()
	defer f.mux.Unlock()

	delete(f.allocListeners, c)
}

func (f *Fleet) RegisterGameServer(addr GameServerAddr, cap uint) GameServerID {
	f.mux.Lock()
	defer f.mux.Unlock()

	id := GameServerID(uuid.Must(uuid.NewRandom()).String())
	gs := newGameServer(id, addr, cap)
	f.g = append(f.g, gs)
	return id
}

func (f *Fleet) IsRegisteredGameServer(id GameServerID) bool {
	f.mux.RLock()
	defer f.mux.RUnlock()

	for _, g := range f.g {
		if g.id == id {
			return true
		}
	}
	return false
}

func (f *Fleet) AllocateRoom(roomID quark.RoomID, roomName string) (GameServerAddr, error) {
	err := func() error {
		f.mux.RLock()
		defer f.mux.RUnlock()

		if len(f.g) == 0 {
			return ErrNotEnoughGameServers
		}
		_, ok := f.rg[roomID]
		if ok {
			return ErrRoomAlreadyAllocated
		}
		return nil
	}()
	if err != nil {
		return GameServerAddr{}, err
	}
	f.mux.Lock()
	defer f.mux.Unlock()

	var lookup func(gs []*GameServer) *GameServer
	lookup = func(gs []*GameServer) *GameServer {
		if len(gs) == 0 {
			return nil
		}
		g := gs[0]
		if g.HasCapacity() {
			return g
		}
		return lookup(gs[1:])
	}

	g := lookup(f.g)
	if g == nil {
		return GameServerAddr{}, ErrNotEnoughGameServers
	}

	err = g.AddRoom(roomID)
	if err != nil {
		return GameServerAddr{}, err
	}

	room := RoomStatus{RoomID: roomID, RoomName: roomName, ActorCount: 0}
	f.rg[roomID] = g
	f.rs[roomID] = &room

	ev := RoomAllocatedEvent{
		GameServer: g.addr,
		Room:       room,
	}
	for c := range f.allocListeners {
		c <- ev
	}

	return g.addr, nil
}

func (f *Fleet) LookupGameServerAddr(roomID quark.RoomID) (GameServerAddr, bool) {
	f.mux.RLock()
	defer f.mux.RUnlock()
	g, ok := f.rg[roomID]
	return g.addr, ok
}

func (f *Fleet) UpdateRoomStatus(status RoomStatus) error {
	f.mux.Lock()
	defer f.mux.Unlock()
	roomID := status.RoomID

	_, ok := f.rs[roomID]
	if !ok {
		return ErrRoomStatusNotFound
	}
	f.rs[roomID] = &status

	gs, ok := f.rg[roomID]
	if !ok {
		return errors.New("game server not found")
	}
	gs.UpdateRoomStatus(status)

	sort.SliceStable(f.g, func(i, j int) bool {
		return f.g[i].Cap() > f.g[j].Cap()
	})
	return nil
}

func (f *Fleet) RoomList() []RoomStatus {
	f.mux.RLock()
	defer f.mux.RUnlock()

	rs := make([]RoomStatus, len(f.rs))
	for i, r := range f.rs {
		rs[i] = *r
	}
	return rs
}
