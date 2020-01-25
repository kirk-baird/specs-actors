package adt

import (
	"context"

	cid "github.com/ipfs/go-cid"
	hamt "github.com/ipfs/go-hamt-ipld"
	errors "github.com/pkg/errors"

	vmr "github.com/filecoin-project/specs-actors/actors/runtime"
)

// Store defines an interface required to back Map.
type Store interface {
	Context() context.Context
	hamt.CborIpldStore
}

// Keyer defines an interface required to put values in Map.
type Keyer interface {
	Key() string
}

// Map stores data in a HAMT.
type Map struct {
	root  cid.Cid
	store Store
}

// NewMap creates a new HAMT with root `r` and store `s`.
func NewMap(s Store, r cid.Cid) *Map {
	return &Map{
		root:  r,
		store: s,
	}
}

// Root return the root cid of HAMT.
func (h *Map) Root() cid.Cid {
	return h.root
}

// Put adds value `v` with key `k` to the hamt store.
func (h *Map) Put(k Keyer, v vmr.CBORMarshaler) error {
	oldRoot, err := hamt.LoadNode(h.store.Context(), h.store, h.root)
	if err != nil {
		return errors.Wrapf(err, "Map Put failed to load node %v", h.root)
	}
	if err := oldRoot.Set(h.store.Context(), k.Key(), v); err != nil {
		return errors.Wrapf(err, "Map Put failed set in node %v with key %v value %v", h.root, k.Key(), v)
	}
	if err := oldRoot.Flush(h.store.Context()); err != nil {
		return errors.Wrapf(err, "Map Put failed to flush node %v : %v", h.root, err)
	}

	// update the root
	newRoot, err := h.store.Put(h.store.Context(), oldRoot)
	if err != nil {
		return errors.Wrapf(err, "Map Put failed to persist changes to store %s", h.root)
	}
	h.root = newRoot
	return nil
}

// Get puts the value at `k` into `out`.
func (h *Map) Get(k Keyer, out vmr.CBORUnmarshaler) (bool, error) {
	oldRoot, err := hamt.LoadNode(h.store.Context(), h.store, h.root)
	if err != nil {
		return false, errors.Wrapf(err, "Map Get failed to load node %v", h.root)
	}
	if err := oldRoot.Find(h.store.Context(), k.Key(), out); err != nil {
		if err == hamt.ErrNotFound {
			return false, nil
		}
		return false, errors.Wrapf(err, "Map Get failed find in node %v with key %v", h.root, k.Key())
	}
	return true, nil
}

// Delete removes the value at `k` from the hamt store.
func (h *Map) Delete(k Keyer) error {
	oldRoot, err := hamt.LoadNode(h.store.Context(), h.store, h.root)
	if err != nil {
		return errors.Wrapf(err, "Map Delete failed to load node %v", h.root)
	}
	if err := oldRoot.Delete(h.store.Context(), k.Key()); err != nil {
		return errors.Wrapf(err, "Map Delete failed in node %v key %v", h.root, k.Key())
	}
	if err := oldRoot.Flush(h.store.Context()); err != nil {
		return errors.Wrapf(err, "Map Delete failed to flush node %v : %v", h.root, err)
	}

	// update the root
	newRoot, err := h.store.Put(h.store.Context(), oldRoot)
	if err != nil {
		return errors.Wrapf(err, "Map Delete failed to persist changes to store %s", h.root)
	}
	h.root = newRoot
	return nil
}

// ForEach applies fn to each key value in hamt.
func (h *Map) ForEach(fn func(key string, v interface{}) error) error {
	oldRoot, err := hamt.LoadNode(h.store.Context(), h.store, h.root)
	if err != nil {
		return errors.Wrapf(err, "Map ForEach failed to load node %v", h.root)
	}
	if err := oldRoot.ForEach(h.store.Context(), fn); err != nil {
		return errors.Wrapf(err, "Map ForEach failed to iterate node %v", h.root)
	}
	return nil
}

// AsStore allows Runtime to satisfy the adt.Store interface.
func AsStore(rt vmr.Runtime) Store {
	return rtStore{rt}
}

var _ Store = &rtStore{}

type rtStore struct {
	vmr.Runtime
}

func (r rtStore) Context() context.Context {
	return r.Runtime.Context()
}

func (r rtStore) Get(ctx context.Context, c cid.Cid, out interface{}) error {
	if !r.IpldGet(c, out.(vmr.CBORUnmarshaler)) {
		r.AbortStateMsg("not found")
	}
	return nil
}

func (r rtStore) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	return r.IpldPut(v.(vmr.CBORMarshaler)), nil
}