//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

// Package notify provides some notification services to be used by box services.
package notify

import "fmt"

// Notifier send events about their container and content.
type Notifier interface {
	// Return the channel
	Events() <-chan Event

	// Signal a refresh of the container. This will result in some events.
	Refresh()

	// Close the notifier (and eventually the channel)
	Close()
}

// EventOp describe a notification operation.
type EventOp uint8

// Valid constants for event operations.
//
// Error signals a detected error. Details are in Event.Err.
//
// Make signals that the container is detected. List events will follow.
//
// List signals a found file, if Event.Name is not empty. Otherwise it signals
// the end of files within the container.
//
// Destroy signals that the container is not there any more. It might me Make later again.
//
// Update signals that file Event.Name was created/updated.
// File name is relative to the container.
//
// Delete signals that file Event.Name was removed.
// File name is relative to the container's name.
const (
	_       EventOp = iota
	Error           // Error while operating
	Make            // Make container
	List            // List container
	Destroy         // Destroy container
	Update          // Update element
	Delete          // Delete element
)

// String representation of operation code.
func (c EventOp) String() string {
	switch c {
	case Error:
		return "ERROR"
	case Make:
		return "MAKE"
	case List:
		return "LIST"
	case Destroy:
		return "DESTROY"
	case Update:
		return "UPDATE"
	case Delete:
		return "DELETE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", c)
	}
}

// Event represents a single container / element event.
type Event struct {
	Op   EventOp
	Name string
	Err  error // Valid iff Op == Error
}
