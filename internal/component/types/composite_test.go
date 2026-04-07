// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "testing"

// TestComposite_RecordRoundTrip builds a record via the builder, finishes
// the bag, and verifies field shape and ABI.
func TestComposite_RecordRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	r := b.InternRecord([]RecordField{
		{Name: "x", Type: U32},
		{Name: "y", Type: U32},
	})
	ct := b.Finish()
	if r.Kind != TypeKindRecord {
		t.Fatalf("Kind = %v, want TypeKindRecord", r.Kind)
	}
	rec := ct.Records[r.Index]
	if len(rec.Fields) != 2 {
		t.Fatalf("len(Fields) = %d, want 2", len(rec.Fields))
	}
	if rec.Fields[0].Name != "x" || rec.Fields[1].Name != "y" {
		t.Errorf("field names = [%q,%q], want [x,y]", rec.Fields[0].Name, rec.Fields[1].Name)
	}
	if rec.ABI.Size32 != 8 || rec.ABI.Align32 != 4 {
		t.Errorf("record ABI = {size32=%d,align32=%d}, want {8,4}", rec.ABI.Size32, rec.ABI.Align32)
	}
}

// TestComposite_VariantRoundTrip builds a variant with mixed payload/no-payload
// cases and verifies the discriminant info and ABI.
func TestComposite_VariantRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	v := b.InternVariant([]VariantCase{
		{Name: "none", HasPayload: false},
		{Name: "some", Payload: U32, HasPayload: true},
	})
	ct := b.Finish()
	if v.Kind != TypeKindVariant {
		t.Fatalf("Kind = %v, want TypeKindVariant", v.Kind)
	}
	variant := ct.Variants[v.Index]
	if variant.Disc.DiscSize != 1 {
		t.Errorf("Disc.DiscSize = %d, want 1", variant.Disc.DiscSize)
	}
	// align(disc=1 to payload-align=4) = 4 → payload offset = 4
	if variant.Disc.PayloadOffset != 4 {
		t.Errorf("Disc.PayloadOffset = %d, want 4", variant.Disc.PayloadOffset)
	}
}

// TestComposite_ListRoundTrip exercises the dynamic list path.
func TestComposite_ListRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	l := b.InternList(U32)
	ct := b.Finish()
	if l.Kind != TypeKindList {
		t.Fatalf("Kind = %v, want TypeKindList", l.Kind)
	}
	if ct.Lists[l.Index].Element != U32 {
		t.Errorf("Element = %v, want U32", ct.Lists[l.Index].Element)
	}
}

// TestComposite_FixedListRoundTrip exercises the fixed-length list path
// and verifies that fixed lists with different lengths are distinct.
func TestComposite_FixedListRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternFixedLengthList(U32, 3)
	c := b.InternFixedLengthList(U32, 4)
	ct := b.Finish()
	if a.Kind != TypeKindFixedList || c.Kind != TypeKindFixedList {
		t.Fatalf("kinds = (%v, %v), want both TypeKindFixedList", a.Kind, c.Kind)
	}
	if a == c {
		t.Errorf("fixed lists with different lengths collapsed: %v == %v", a, c)
	}
	if ct.FixedLists[a.Index].Length != 3 || ct.FixedLists[c.Index].Length != 4 {
		t.Errorf("lengths = (%d, %d), want (3, 4)", ct.FixedLists[a.Index].Length, ct.FixedLists[c.Index].Length)
	}
}

// TestComposite_TupleRoundTrip exercises tuples.
func TestComposite_TupleRoundTrip(t *testing.T) {
	b := NewComponentTypesBuilder()
	tup := b.InternTuple([]ValType{U32, S32, Bool})
	ct := b.Finish()
	if tup.Kind != TypeKindTuple {
		t.Fatalf("Kind = %v, want TypeKindTuple", tup.Kind)
	}
	if len(ct.Tuples[tup.Index].Types) != 3 {
		t.Errorf("len(Types) = %d, want 3", len(ct.Tuples[tup.Index].Types))
	}
}

// TestComposite_OptionResultEnumFlags exercises the remaining composites.
func TestComposite_OptionResultEnumFlags(t *testing.T) {
	b := NewComponentTypesBuilder()
	opt := b.InternOption(U32)
	res := b.InternResult(U32, U32, true, true)
	en := b.InternEnum([]string{"red", "green", "blue"})
	fl := b.InternFlags([]string{"r", "w", "x"})
	ct := b.Finish()
	if opt.Kind != TypeKindOption {
		t.Errorf("opt.Kind = %v, want TypeKindOption", opt.Kind)
	}
	if res.Kind != TypeKindResult {
		t.Errorf("res.Kind = %v, want TypeKindResult", res.Kind)
	}
	if en.Kind != TypeKindEnum {
		t.Errorf("en.Kind = %v, want TypeKindEnum", en.Kind)
	}
	if fl.Kind != TypeKindFlags {
		t.Errorf("fl.Kind = %v, want TypeKindFlags", fl.Kind)
	}
	if len(ct.Enums[en.Index].Names) != 3 {
		t.Errorf("len(enum.Names) = %d, want 3", len(ct.Enums[en.Index].Names))
	}
	if len(ct.Flags[fl.Index].Names) != 3 {
		t.Errorf("len(flags.Names) = %d, want 3", len(ct.Flags[fl.Index].Names))
	}
}

// TestComposite_AsyncTypes exercises stream and future tables.
func TestComposite_AsyncTypes(t *testing.T) {
	b := NewComponentTypesBuilder()
	s := b.InternStream(U32, true)
	f := b.InternFuture(U32, true)
	ec := b.InternErrorContextTable()
	ct := b.Finish()
	if s.Kind != TypeKindStream {
		t.Errorf("s.Kind = %v, want TypeKindStream", s.Kind)
	}
	if f.Kind != TypeKindFuture {
		t.Errorf("f.Kind = %v, want TypeKindFuture", f.Kind)
	}
	if ec.Kind != TypeKindErrorContext {
		t.Errorf("ec.Kind = %v, want TypeKindErrorContext", ec.Kind)
	}
	if !ct.Streams[s.Index].HasElement || ct.Streams[s.Index].Element != U32 {
		t.Errorf("stream payload = %v/%v, want true/U32", ct.Streams[s.Index].HasElement, ct.Streams[s.Index].Element)
	}
	if len(ct.ErrorContextTables) != 1 {
		t.Errorf("len(ErrorContextTables) = %d, want 1", len(ct.ErrorContextTables))
	}
}
