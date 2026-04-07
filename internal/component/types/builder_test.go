// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "testing"

func TestBuilderInternRecordDedup(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternRecord([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	})
	c := b.InternRecord([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	})
	if a != c {
		t.Errorf("structurally identical records produced different ValTypes: %v vs %v", a, c)
	}
}

func TestBuilderInternRecordDistinct(t *testing.T) {
	b := NewComponentTypesBuilder()
	// Different field names → distinct
	a := b.InternRecord([]RecordField{{Name: "a", Type: U32}})
	c := b.InternRecord([]RecordField{{Name: "b", Type: U32}})
	if a == c {
		t.Errorf("differently-named records collapsed: %v == %v", a, c)
	}
	// Different field order → distinct
	d := b.InternRecord([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	})
	e := b.InternRecord([]RecordField{
		{Name: "b", Type: U32},
		{Name: "a", Type: U32},
	})
	if d == e {
		t.Errorf("reordered-field records collapsed: %v == %v", d, e)
	}
}

func TestBuilderInternListVsFixedList(t *testing.T) {
	b := NewComponentTypesBuilder()
	dynList := b.InternList(U32)
	fixedList5 := b.InternFixedLengthList(U32, 5)
	fixedList7 := b.InternFixedLengthList(U32, 7)
	if dynList.Kind != TypeKindList {
		t.Errorf("dynList.Kind = %v, want TypeKindList", dynList.Kind)
	}
	if fixedList5.Kind != TypeKindFixedList {
		t.Errorf("fixedList5.Kind = %v, want TypeKindFixedList", fixedList5.Kind)
	}
	if dynList == fixedList5 {
		t.Errorf("dynamic and fixed-length list collapsed: %v == %v", dynList, fixedList5)
	}
	if fixedList5 == fixedList7 {
		t.Errorf("fixed lists with different lengths collapsed: %v == %v", fixedList5, fixedList7)
	}
	dynList2 := b.InternList(U32)
	if dynList != dynList2 {
		t.Errorf("dynamic list dedup failed: %v vs %v", dynList, dynList2)
	}
	fixedList5b := b.InternFixedLengthList(U32, 5)
	if fixedList5 != fixedList5b {
		t.Errorf("fixed-length list dedup failed: %v vs %v", fixedList5, fixedList5b)
	}
}

func TestBuilderInternResultDistinct(t *testing.T) {
	b := NewComponentTypesBuilder()
	rA := b.InternResult(U32, ValType{}, true, false)
	rB := b.InternResult(ValType{}, U32, false, true)
	rC := b.InternResult(U32, U32, true, true)
	rD := b.InternResult(ValType{}, ValType{}, false, false)
	if rA == rB || rA == rC || rA == rD || rB == rC || rB == rD || rC == rD {
		t.Errorf("results with different has-flags collapsed: %v %v %v %v", rA, rB, rC, rD)
	}
}

func TestBuilderInternTupleDedup(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternTuple([]ValType{U32, U32})
	c := b.InternTuple([]ValType{U32, U32})
	if a != c {
		t.Errorf("identical tuples not deduped: %v vs %v", a, c)
	}
}

func TestBuilderInternFlagsDedup(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternFlags([]string{"x", "y"})
	c := b.InternFlags([]string{"x", "y"})
	if a != c {
		t.Errorf("identical flags not deduped: %v vs %v", a, c)
	}
	d := b.InternFlags([]string{"y", "x"})
	if a == d {
		t.Errorf("reordered flags collapsed: %v == %v", a, d)
	}
}

func TestBuilderInternAbstractResourceDoesNotDedup(t *testing.T) {
	b := NewComponentTypesBuilder()
	a := b.InternAbstractResource()
	c := b.InternAbstractResource()
	// Each call returns a fresh ResourceTableIdx — abstract resource
	// declarations are distinct by construction.
	if a == c {
		t.Errorf("InternAbstractResource collapsed two calls: %v == %v", a, c)
	}
}

func TestBuilderFinishFreezesBuilder(t *testing.T) {
	b := NewComponentTypesBuilder()
	b.InternRecord([]RecordField{{Name: "a", Type: U32}})
	ct := b.Finish()
	if ct == nil {
		t.Fatal("Finish returned nil")
	}
	if len(ct.Records) != 1 {
		t.Errorf("len(ct.Records) = %d, want 1", len(ct.Records))
	}
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("post-Finish InternRecord did not panic")
		}
	}()
	b.InternRecord([]RecordField{{Name: "b", Type: U32}})
}

func TestBuilderDoubleFinishPanics(t *testing.T) {
	b := NewComponentTypesBuilder()
	b.Finish()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("second Finish() did not panic")
		}
	}()
	b.Finish()
}

func TestBuilderInternFunc(t *testing.T) {
	b := NewComponentTypesBuilder()
	params := b.InternTuple([]ValType{U32, S32})
	results := b.InternTuple([]ValType{Bool})
	idx := b.InternFunc(false, []string{"a", "b"}, params, results)
	idx2 := b.InternFunc(false, []string{"a", "b"}, params, results)
	if idx != idx2 {
		t.Errorf("InternFunc dedup failed: %v vs %v", idx, idx2)
	}
	// Different parameter names → distinct
	idx3 := b.InternFunc(false, []string{"x", "y"}, params, results)
	if idx == idx3 {
		t.Errorf("InternFunc collapsed differently-named params: %v == %v", idx, idx3)
	}
}

func TestBuilderRecordABIPrecomputed(t *testing.T) {
	// Verify that Intern* populates the ABI field correctly.
	b := NewComponentTypesBuilder()
	r := b.InternRecord([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	})
	ct := b.Finish()
	abi := ct.Records[r.Index].ABI
	if abi.Size32 != 8 || abi.Align32 != 4 || abi.FlattenCount != 2 {
		t.Errorf("interned record ABI = {%d/%d/%d}, want {8/4/2}",
			abi.Size32, abi.Align32, abi.FlattenCount)
	}
}
