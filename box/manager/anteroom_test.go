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

package manager

import (
	"testing"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/id/idset"
)

func TestSimple(t *testing.T) {
	t.Parallel()
	ar := newAnteroomQueue(2)
	ar.EnqueueZettel(id.Zid(1))
	action, zid, lastReload := ar.Dequeue()
	if zid != id.Zid(1) || action != arZettel || lastReload {
		t.Errorf("Expected arZettel/1/false, but got %v/%v/%v", action, zid, lastReload)
	}
	_, zid, _ = ar.Dequeue()
	if zid != id.Invalid {
		t.Errorf("Expected invalid Zid, but got %v", zid)
	}
	ar.EnqueueZettel(id.Zid(1))
	ar.EnqueueZettel(id.Zid(2))
	if ar.first != ar.last {
		t.Errorf("Expected one room, but got more")
	}
	ar.EnqueueZettel(id.Zid(3))
	if ar.first == ar.last {
		t.Errorf("Expected more than one room, but got only one")
	}

	count := 0
	for ; count < 1000; count++ {
		action, _, _ = ar.Dequeue()
		if action == arNothing {
			break
		}
	}
	if count != 3 {
		t.Errorf("Expected 3 dequeues, but got %v", count)
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	ar := newAnteroomQueue(1)
	ar.EnqueueZettel(id.Zid(1))
	ar.Reset()
	action, zid, _ := ar.Dequeue()
	if action != arReload || zid != id.Invalid {
		t.Errorf("Expected reload & invalid Zid, but got %v/%v", action, zid)
	}
	ar.Reload(idset.NewSet(3, 4))
	ar.EnqueueZettel(id.Zid(5))
	ar.EnqueueZettel(id.Zid(5))
	if ar.first == ar.last || ar.first.next != ar.last /*|| ar.first.next.next != ar.last*/ {
		t.Errorf("Expected 2 rooms")
	}
	action, zid1, _ := ar.Dequeue()
	if action != arZettel {
		t.Errorf("Expected arZettel, but got %v", action)
	}
	action, zid2, _ := ar.Dequeue()
	if action != arZettel {
		t.Errorf("Expected arZettel, but got %v", action)
	}
	if !(zid1 == id.Zid(3) && zid2 == id.Zid(4) || zid1 == id.Zid(4) && zid2 == id.Zid(3)) {
		t.Errorf("Zids must be 3 or 4, but got %v/%v", zid1, zid2)
	}
	action, zid, _ = ar.Dequeue()
	if zid != id.Zid(5) || action != arZettel {
		t.Errorf("Expected 5/arZettel, but got %v/%v", zid, action)
	}
	action, zid, _ = ar.Dequeue()
	if action != arNothing || zid != id.Invalid {
		t.Errorf("Expected nothing & invalid Zid, but got %v/%v", action, zid)
	}

	ar = newAnteroomQueue(1)
	ar.Reload(idset.NewSet(id.Zid(6)))
	action, zid, _ = ar.Dequeue()
	if zid != id.Zid(6) || action != arZettel {
		t.Errorf("Expected 6/arZettel, but got %v/%v", zid, action)
	}
	action, zid, _ = ar.Dequeue()
	if action != arNothing || zid != id.Invalid {
		t.Errorf("Expected nothing & invalid Zid, but got %v/%v", action, zid)
	}

	ar = newAnteroomQueue(1)
	ar.EnqueueZettel(id.Zid(8))
	ar.Reload(nil)
	action, zid, _ = ar.Dequeue()
	if action != arNothing || zid != id.Invalid {
		t.Errorf("Expected nothing & invalid Zid, but got %v/%v", action, zid)
	}
}
