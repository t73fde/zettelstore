//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

// Package box provides a generic interface to zettel boxes.
package box

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"t73f.de/r/zsc/api"
	"zettelstore.de/z/query"
	"zettelstore.de/z/zettel"
	"zettelstore.de/z/zettel/id"
	"zettelstore.de/z/zettel/meta"
)

// BaseBox is implemented by all Zettel boxes.
type BaseBox interface {
	// Location returns some information where the box is located.
	// Format is dependent of the box.
	Location() string

	// GetZettel retrieves a specific zettel.
	GetZettel(ctx context.Context, zid id.Zid) (zettel.Zettel, error)

	// CanDeleteZettel returns true, if box could possibly delete the given zettel.
	CanDeleteZettel(ctx context.Context, zid id.Zid) bool

	// DeleteZettel removes the zettel from the box.
	DeleteZettel(ctx context.Context, zid id.Zid) error
}

// WriteBox is a box that can create / update zettel content.
type WriteBox interface {
	// CanCreateZettel returns true, if box could possibly create a new zettel.
	CanCreateZettel(ctx context.Context) bool

	// CreateZettel creates a new zettel.
	// Returns the new zettel id (and an error indication).
	CreateZettel(ctx context.Context, zettel zettel.Zettel) (id.Zid, error)

	// CanUpdateZettel returns true, if box could possibly update the given zettel.
	CanUpdateZettel(ctx context.Context, zettel zettel.Zettel) bool

	// UpdateZettel updates an existing zettel.
	UpdateZettel(ctx context.Context, zettel zettel.Zettel) error
}

// ZidFunc is a function that processes identifier of a zettel.
type ZidFunc func(id.Zid)

// MetaFunc is a function that processes metadata of a zettel.
type MetaFunc func(*meta.Meta)

// ManagedBox is the interface of managed boxes.
type ManagedBox interface {
	BaseBox

	// HasZettel returns true, if box conains zettel with given identifier.
	HasZettel(context.Context, id.Zid) bool

	// Apply identifier of every zettel to the given function, if predicate returns true.
	ApplyZid(context.Context, ZidFunc, query.RetrievePredicate) error

	// Apply metadata of every zettel to the given function, if predicate returns true.
	ApplyMeta(context.Context, MetaFunc, query.RetrievePredicate) error

	// ReadStats populates st with box statistics
	ReadStats(st *ManagedBoxStats)
}

// ManagedBoxStats records statistics about the box.
type ManagedBoxStats struct {
	// ReadOnly indicates that the content of a box cannot change.
	ReadOnly bool

	// Zettel is the number of zettel managed by the box.
	Zettel int
}

// StartState enumerates the possible states of starting and stopping a box.
//
// StartStateStopped -> StartStateStarting -> StartStateStarted -> StateStateStopping -> StartStateStopped.
//
// Other transitions are also possible.
type StartState uint8

// Constant values of StartState
const (
	StartStateStopped StartState = iota
	StartStateStarting
	StartStateStarted
	StartStateStopping
)

// StartStopper performs simple lifecycle management.
type StartStopper interface {
	// State the current status of the box.
	State() StartState

	// Start the box. Now all other functions of the box are allowed.
	// Starting a box, which is not in state StartStateStopped is not allowed.
	Start(ctx context.Context) error

	// Stop the started box. Now only the Start() function is allowed.
	Stop(ctx context.Context)
}

// Refresher allow to refresh their internal data.
type Refresher interface {
	// Refresh the box data.
	Refresh(context.Context)
}

// Box is to be used outside the box package and its descendants.
type Box interface {
	BaseBox
	WriteBox

	// FetchZids returns the set of all zettel identifer managed by the box.
	FetchZids(ctx context.Context) (*id.Set, error)

	// GetMeta returns the metadata of the zettel with the given identifier.
	GetMeta(context.Context, id.Zid) (*meta.Meta, error)

	// SelectMeta returns a list of metadata that comply to the given selection criteria.
	// If `metaSeq` is `nil`, the box assumes metadata of all available zettel.
	SelectMeta(ctx context.Context, metaSeq []*meta.Meta, q *query.Query) ([]*meta.Meta, error)

	// GetAllZettel retrieves a specific zettel from all managed boxes.
	GetAllZettel(ctx context.Context, zid id.Zid) ([]zettel.Zettel, error)

	// Refresh the data from the box and from its managed sub-boxes.
	Refresh(context.Context) error

	// ReIndex one zettel to update its index data.
	ReIndex(context.Context, id.Zid) error
}

// Stats record stattistics about a box.
type Stats struct {
	// ReadOnly indicates that boxes cannot be modified.
	ReadOnly bool

	// NumManagedBoxes is the number of boxes managed.
	NumManagedBoxes int

	// Zettel is the number of zettel managed by the box, including
	// duplicates across managed boxes.
	ZettelTotal int

	// LastReload stores the timestamp when a full re-index was done.
	LastReload time.Time

	// DurLastReload is the duration of the last full re-index run.
	DurLastReload time.Duration

	// IndexesSinceReload counts indexing a zettel since the full re-index.
	IndexesSinceReload uint64

	// ZettelIndexed is the number of zettel managed by the indexer.
	ZettelIndexed int

	// IndexUpdates count the number of metadata updates.
	IndexUpdates uint64

	// IndexedWords count the different words indexed.
	IndexedWords uint64

	// IndexedUrls count the different URLs indexed.
	IndexedUrls uint64
}

// Manager is a box-managing box.
type Manager interface {
	Box
	StartStopper
	Subject

	// ReadStats populates st with box statistics
	ReadStats(st *Stats)

	// Dump internal data to a Writer.
	Dump(w io.Writer)

	// Return zid mapper
	Mapper() Mapper
}

// Mapper is used for mapping old-style to and from new-style zettel identifier
type Mapper interface {
	LookupZidO(id.ZidN) (id.Zid, bool)
}

// UpdateReason gives an indication, why the ObserverFunc was called.
type UpdateReason uint8

// Values for Reason
const (
	_        UpdateReason = iota
	OnReady               // Box is started and fully operational
	OnReload              // Box was reloaded
	OnZettel              // Something with an existing zettel happened
	OnDelete              // A zettel was deleted
)

// UpdateInfo contains all the data about a changed zettel.
type UpdateInfo struct {
	Box    BaseBox
	Reason UpdateReason
	Zid    id.Zid
}

// UpdateFunc is a function to be called when a change is detected.
type UpdateFunc func(UpdateInfo)

// UpdateNotifier is an UpdateFunc, but with separate values.
type UpdateNotifier func(BaseBox, id.Zid, UpdateReason, bool)

// Subject is a box that notifies observers about changes.
type Subject interface {
	// RegisterObserver registers an observer that will be notified
	// if one or all zettel are found to be changed.
	RegisterObserver(UpdateFunc)
}

// Enricher is used to update metadata by adding new properties.
type Enricher interface {
	// Enrich computes additional properties and updates the given metadata.
	// It is typically called by zettel reading methods.
	Enrich(ctx context.Context, m *meta.Meta, boxNumber int)
}

// NoEnrichContext will signal an enricher that nothing has to be done.
// This is useful for an Indexer, but also for some box.Box calls, when
// just the plain metadata is needed.
func NoEnrichContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxNoEnrichKey, &ctxNoEnrichKey)
}

type ctxNoEnrichType struct{}

var ctxNoEnrichKey ctxNoEnrichType

// DoEnrich determines if the context is not marked to not enrich metadata.
func DoEnrich(ctx context.Context) bool {
	_, ok := ctx.Value(ctxNoEnrichKey).(*ctxNoEnrichType)
	return !ok
}

// NoEnrichQuery provides a context that signals not to enrich, if the query does not need this.
func NoEnrichQuery(ctx context.Context, q *query.Query) context.Context {
	if q.EnrichNeeded() {
		return ctx
	}
	return NoEnrichContext(ctx)
}

// ErrNotAllowed is returned if the caller is not allowed to perform the operation.
type ErrNotAllowed struct {
	Op   string
	User *meta.Meta
	Zid  id.Zid
}

// NewErrNotAllowed creates an new authorization error.
func NewErrNotAllowed(op string, user *meta.Meta, zid id.Zid) error {
	return &ErrNotAllowed{
		Op:   op,
		User: user,
		Zid:  zid,
	}
}

func (err *ErrNotAllowed) Error() string {
	if err.User == nil {
		if err.Zid.IsValid() {
			return fmt.Sprintf(
				"operation %q on zettel %v not allowed for not authorized user",
				err.Op, err.Zid)
		}
		return fmt.Sprintf("operation %q not allowed for not authorized user", err.Op)
	}
	if err.Zid.IsValid() {
		return fmt.Sprintf(
			"operation %q on zettel %v not allowed for user %v/%v",
			err.Op, err.Zid, err.User.GetDefault(api.KeyUserID, "?"), err.User.Zid)
	}
	return fmt.Sprintf(
		"operation %q not allowed for user %v/%v",
		err.Op, err.User.GetDefault(api.KeyUserID, "?"), err.User.Zid)
}

// Is return true, if the error is of type ErrNotAllowed.
func (*ErrNotAllowed) Is(error) bool { return true }

// ErrStarted is returned when trying to start an already started box.
var ErrStarted = errors.New("box is already started")

// ErrStopped is returned if calling methods on a box that was not started.
var ErrStopped = errors.New("box is stopped")

// ErrReadOnly is returned if there is an attepmt to write to a read-only box.
var ErrReadOnly = errors.New("read-only box")

// ErrZettelNotFound is returned if a zettel was not found in the box.
type ErrZettelNotFound struct{ Zid id.Zid }

func (eznf ErrZettelNotFound) Error() string { return "zettel not found: " + eznf.Zid.String() }

// ErrConflict is returned if a box operation detected a conflict..
// One example: if calculating a new zettel identifier takes too long.
var ErrConflict = errors.New("conflict")

// ErrCapacity is returned if a box has reached its capacity.
var ErrCapacity = errors.New("capacity exceeded")

// ErrInvalidZid is returned if the zettel id is not appropriate for the box operation.
type ErrInvalidZid struct{ Zid string }

func (err ErrInvalidZid) Error() string { return "invalid Zettel id: " + err.Zid }
