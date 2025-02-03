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
	"context"
	"strconv"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"zettelstore.de/z/box"
	"zettelstore.de/z/zettel/meta"
)

// Enrich computes additional properties and updates the given metadata.
func (mgr *Manager) Enrich(ctx context.Context, m *meta.Meta, boxNumber int) {
	// Calculate computed, but stored values.
	if _, hasCreated := m.Get(api.KeyCreated); !hasCreated {
		m.Set(api.KeyCreated, computeCreated(m.Zid))
	}

	if box.DoEnrich(ctx) {
		computePublished(m)
		if boxNumber > 0 {
			m.Set(api.KeyBoxNumber, strconv.Itoa(boxNumber))
		}
		mgr.idxStore.Enrich(ctx, m)
	}
}

func computeCreated(zid id.Zid) string {
	if zid <= 10101000000 {
		// A year 0000 is not allowed and therefore an artificial Zid.
		// In the year 0001, the month must be > 0.
		// In the month 000101, the day must be > 0.
		return "00010101000000"
	}
	seconds := zid % 100
	if seconds > 59 {
		seconds = 59
	}
	zid /= 100
	minutes := zid % 100
	if minutes > 59 {
		minutes = 59
	}
	zid /= 100
	hours := zid % 100
	if hours > 23 {
		hours = 23
	}
	zid /= 100
	day := zid % 100
	zid /= 100
	month := zid % 100
	year := zid / 100
	month, day = sanitizeMonthDay(year, month, day)
	created := ((((year*100+month)*100+day)*100+hours)*100+minutes)*100 + seconds
	return created.String()
}

func sanitizeMonthDay(year, month, day id.Zid) (id.Zid, id.Zid) {
	if day < 1 {
		day = 1
	}
	if month < 1 {
		month = 1
	}
	if month > 12 {
		month = 12
	}

	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		if day > 31 {
			day = 31
		}
	case 4, 6, 9, 11:
		if day > 30 {
			day = 30
		}
	case 2:
		if year%4 != 0 || (year%100 == 0 && year%400 != 0) {
			if day > 28 {
				day = 28
			}
		} else {
			if day > 29 {
				day = 29
			}
		}
	}
	return month, day
}

func computePublished(m *meta.Meta) {
	if _, ok := m.Get(api.KeyPublished); ok {
		return
	}
	if modified, ok := m.Get(api.KeyModified); ok {
		if _, ok = meta.TimeValue(modified); ok {
			m.Set(api.KeyPublished, modified)
			return
		}
	}
	if created, ok := m.Get(api.KeyCreated); ok {
		if _, ok = meta.TimeValue(created); ok {
			m.Set(api.KeyPublished, created)
			return
		}
	}
	zid := m.Zid.String()
	if _, ok := meta.TimeValue(zid); ok {
		m.Set(api.KeyPublished, zid)
		return
	}

	// Neither the zettel was modified nor the zettel identifer contains a valid
	// timestamp. In this case do not set the "published" property.
}
