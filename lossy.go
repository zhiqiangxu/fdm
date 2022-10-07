package fdm

import (
	zlog "github.com/rs/zerolog/log"
)

type LossyForkDetectorManager[S any, H comparable] struct {
	offset     int
	snapshots  []*Snapshot[S, H]
	reorgFn    ReorgFunc[S, H]
	snapFn     SnapFunc[S, H]
	preStateFn PreStateFunc[S]
	stash      []*Snapshot[S, H]
}

type Header[H any] interface {
	Height() uint64
	ParentHash() H
	Hash() H
}

type Snapshot[S any, H comparable] struct {
	header     Header[H]
	hash       H
	preState   S
	afterState S
}

type ReorgFunc[S any, H comparable] func(old, new []*Snapshot[S, H])

// returned Snapshot should contain empty preState
type SnapFunc[S any, H comparable] func(hash H) *Snapshot[S, H]

type PreStateFunc[S any] func(S) S

func New[S any, H comparable](maxFork int, reorgFn ReorgFunc[S, H], snapFn SnapFunc[S, H], preStateFn PreStateFunc[S]) *LossyForkDetectorManager[S, H] {
	return &LossyForkDetectorManager[S, H]{
		snapshots:  make([]*Snapshot[S, H], maxFork+1),
		reorgFn:    reorgFn,
		snapFn:     snapFn,
		preStateFn: preStateFn,
		stash:      make([]*Snapshot[S, H], 1),
	}
}

func (fdm *LossyForkDetectorManager[S, H]) Submit(newSnapshot *Snapshot[S, H]) {

	oldSnapshot := fdm.snapshots[fdm.offset]
	if oldSnapshot == nil || oldSnapshot.header.Height()+1 != newSnapshot.header.Height() {
		// treat as fastforward
		if oldSnapshot == nil {
			zlog.Info().Msg("treat as fastforward since prev is nil")
		} else {
			zlog.Info().Uint64("old", oldSnapshot.header.Height()).Uint64("new", newSnapshot.header.Height()).Msg("treat as fastforward since header is not continuous")
		}

		fdm.offset = (fdm.offset + 1) % len(fdm.snapshots)
		fdm.snapshots[fdm.offset] = newSnapshot
		fdm.stash[0] = newSnapshot
		fdm.reorgFn(nil, fdm.stash)
		return
	}

	// here oldSnapshot and newSnapshot have a gap of 1
	if newSnapshot.header.ParentHash() == oldSnapshot.hash {
		zlog.Info().Msg("fastforward")
		// handle normal case
		fdm.offset = (fdm.offset + 1) % len(fdm.snapshots)
		fdm.snapshots[fdm.offset] = newSnapshot
		fdm.stash[0] = newSnapshot
		fdm.reorgFn(nil, fdm.stash)
		return
	}

	newParentSnapshot := fdm.snapFn(newSnapshot.header.ParentHash())
	if newParentSnapshot == nil {
		zlog.Info().Msg("treat as fastforward since snapFn returns nil")
		// treat as a new start
		fdm.snapshots[fdm.offset] = nil
		fdm.offset = (fdm.offset + 1) % len(fdm.snapshots)
		fdm.snapshots[fdm.offset] = newSnapshot
		fdm.stash[0] = newSnapshot
		fdm.reorgFn(nil, fdm.stash)
		return
	}

	old := []*Snapshot[S, H]{oldSnapshot}
	new := []*Snapshot[S, H]{newSnapshot, newParentSnapshot}
	fdm.offset -= 1
	if fdm.offset < 0 {
		fdm.offset = len(fdm.snapshots) - 1
	}

	// handle fork, reverse until common ancestor
	for {
		oldParentOffset := fdm.offset - 1
		if oldParentOffset < 0 {
			oldParentOffset = len(fdm.snapshots) - 1
		}
		oldParent := fdm.snapshots[oldParentOffset]
		if oldParent == nil || oldParent.header.Height()+1 != old[len(old)-1].header.Height() {
			break
		}
		newParent := fdm.snapFn(new[len(new)-1].header.ParentHash())
		if newParent == nil {
			break
		}
		if oldParent.header.ParentHash() == newParent.header.ParentHash() {
			break
		}

		fdm.offset = oldParentOffset
		old = append(old, oldParent)
		new = append(new, newParent)
	}

	zlog.Info().Int("depth", len(old)).Msg("reorg")
	// rollback first
	fdm.reorgFn(old, nil)

	for i := len(new) - 1; i >= 0; i-- {
		newState := new[i]
		newState.preState = fdm.preStateFn(newState.afterState)
		fdm.stash[0] = newState
		// patch new state
		fdm.reorgFn(nil, fdm.stash)
		fdm.offset = (fdm.offset + 1) % len(fdm.snapshots)
		fdm.snapshots[fdm.offset] = newState
	}
}
