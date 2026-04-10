package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
)

func TestResourceTypeEqual_SamePointer(t *testing.T) {
	rt := &runtime.ResourceType{}
	a := NewResourceType(rt)
	b := NewResourceType(rt)
	if !a.Equal(b) {
		t.Error("ResourceType.Equal returned false for the same underlying pointer")
	}
}

func TestResourceTypeEqual_DifferentPointer(t *testing.T) {
	a := NewResourceType(&runtime.ResourceType{})
	b := NewResourceType(&runtime.ResourceType{})
	if a.Equal(b) {
		t.Error("ResourceType.Equal returned true for different underlying pointers")
	}
}

func TestResourceTypeEqual_BothNil(t *testing.T) {
	a := NewResourceType(nil)
	b := NewResourceType(nil)
	if !a.Equal(b) {
		t.Error("ResourceType.Equal returned false for two nil-inner types")
	}
}

func TestResourceTypeInner(t *testing.T) {
	rt := &runtime.ResourceType{}
	pub := NewResourceType(rt)
	if pub.Inner() != rt {
		t.Error("ResourceType.Inner() did not return the original pointer")
	}
}

func TestResourceHandle_Accessors(t *testing.T) {
	rt := &runtime.ResourceType{}
	entry := &runtime.ResourceHandleEntry{
		RT:       rt,
		Rep:      42,
		Own:      true,
		NumLends: 3,
	}
	h := NewResourceHandle(entry)

	if !h.Type().Equal(NewResourceType(rt)) {
		t.Error("ResourceHandle.Type() does not match expected ResourceType")
	}
	if !h.Owned() {
		t.Error("ResourceHandle.Owned() = false, want true")
	}
	if h.Rep() != 42 {
		t.Errorf("ResourceHandle.Rep() = %d, want 42", h.Rep())
	}
	if h.NumLends() != 3 {
		t.Errorf("ResourceHandle.NumLends() = %d, want 3", h.NumLends())
	}
}

func TestResourceHandle_Borrow(t *testing.T) {
	entry := &runtime.ResourceHandleEntry{
		RT:  &runtime.ResourceType{},
		Rep: 7,
		Own: false,
	}
	h := NewResourceHandle(entry)
	if h.Owned() {
		t.Error("ResourceHandle.Owned() = true, want false for a borrow")
	}
}

func TestResourceNew(t *testing.T) {
	table := NewResourceTable()
	rt := NewResourceType(&runtime.ResourceType{})

	handle, err := ResourceNew(table, rt, 100)
	if err != nil {
		t.Fatalf("ResourceNew returned error: %v", err)
	}

	// Retrieve via ResourceGet and verify
	got, err := ResourceGet(table, handle)
	if err != nil {
		t.Fatalf("ResourceGet returned error: %v", err)
	}
	if got.Rep() != 100 {
		t.Errorf("Rep() = %d, want 100", got.Rep())
	}
	if !got.Owned() {
		t.Error("Owned() = false, want true for newly created resource")
	}
	if !got.Type().Equal(rt) {
		t.Error("Type() does not match the ResourceType used in ResourceNew")
	}
}

func TestResourceRep(t *testing.T) {
	table := NewResourceTable()
	rt := NewResourceType(&runtime.ResourceType{})

	handle, err := ResourceNew(table, rt, 55)
	if err != nil {
		t.Fatalf("ResourceNew returned error: %v", err)
	}

	rep, err := ResourceRep(table, handle)
	if err != nil {
		t.Fatalf("ResourceRep returned error: %v", err)
	}
	if rep != 55 {
		t.Errorf("ResourceRep = %d, want 55", rep)
	}
}

func TestResourceRep_InvalidHandle(t *testing.T) {
	table := NewResourceTable()
	_, err := ResourceRep(table, MakeHandle(999, 0))
	if err == nil {
		t.Error("ResourceRep with invalid handle should return error")
	}
}

func TestResourceDrop(t *testing.T) {
	table := NewResourceTable()
	rt := NewResourceType(&runtime.ResourceType{})

	handle, err := ResourceNew(table, rt, 77)
	if err != nil {
		t.Fatalf("ResourceNew returned error: %v", err)
	}

	dropped, err := ResourceDrop(table, handle)
	if err != nil {
		t.Fatalf("ResourceDrop returned error: %v", err)
	}
	if dropped.Rep() != 77 {
		t.Errorf("dropped.Rep() = %d, want 77", dropped.Rep())
	}
	if !dropped.Owned() {
		t.Error("dropped.Owned() = false, want true")
	}

	// The handle should now be invalid
	_, err = ResourceGet(table, handle)
	if err == nil {
		t.Error("ResourceGet after ResourceDrop should return error")
	}
}

func TestResourceDrop_InvalidHandle(t *testing.T) {
	table := NewResourceTable()
	_, err := ResourceDrop(table, MakeHandle(999, 0))
	if err == nil {
		t.Error("ResourceDrop with invalid handle should return error")
	}
}

func TestResourceGet_InvalidHandle(t *testing.T) {
	table := NewResourceTable()
	_, err := ResourceGet(table, MakeHandle(0, 0))
	if err == nil {
		t.Error("ResourceGet with invalid handle should return error")
	}
}

func TestResourceNew_MultipleResources(t *testing.T) {
	table := NewResourceTable()
	rt1 := NewResourceType(&runtime.ResourceType{})
	rt2 := NewResourceType(&runtime.ResourceType{})

	h1, err := ResourceNew(table, rt1, 10)
	if err != nil {
		t.Fatalf("ResourceNew(rt1) returned error: %v", err)
	}
	h2, err := ResourceNew(table, rt2, 20)
	if err != nil {
		t.Fatalf("ResourceNew(rt2) returned error: %v", err)
	}

	// Handles should be different
	if h1 == h2 {
		t.Error("two ResourceNew calls returned the same handle")
	}

	// Each handle should resolve to its own type
	got1, err := ResourceGet(table, h1)
	if err != nil {
		t.Fatalf("ResourceGet(h1) returned error: %v", err)
	}
	got2, err := ResourceGet(table, h2)
	if err != nil {
		t.Fatalf("ResourceGet(h2) returned error: %v", err)
	}

	if !got1.Type().Equal(rt1) {
		t.Error("h1 type mismatch")
	}
	if !got2.Type().Equal(rt2) {
		t.Error("h2 type mismatch")
	}
	if got1.Type().Equal(got2.Type()) {
		t.Error("h1 and h2 should have different resource types")
	}
}

func TestResourceDrop_ThenReuse(t *testing.T) {
	table := NewResourceTable()
	rt := NewResourceType(&runtime.ResourceType{})

	h1, err := ResourceNew(table, rt, 1)
	if err != nil {
		t.Fatalf("ResourceNew returned error: %v", err)
	}

	// Drop h1 to free the slot
	_, err = ResourceDrop(table, h1)
	if err != nil {
		t.Fatalf("ResourceDrop returned error: %v", err)
	}

	// Allocate another — should reuse the freed slot
	h2, err := ResourceNew(table, rt, 2)
	if err != nil {
		t.Fatalf("ResourceNew (reuse) returned error: %v", err)
	}

	// h1 and h2 should have the same index but different generations
	if h1.Index() != h2.Index() {
		t.Errorf("expected slot reuse: h1.Index()=%d, h2.Index()=%d", h1.Index(), h2.Index())
	}
	if h1.Generation() == h2.Generation() {
		t.Error("expected different generations after slot reuse")
	}

	// Old handle h1 must be invalid (generation mismatch)
	_, err = ResourceGet(table, h1)
	if err == nil {
		t.Error("old handle should be invalid after slot reuse")
	}

	// New handle h2 should be valid
	got, err := ResourceGet(table, h2)
	if err != nil {
		t.Fatalf("ResourceGet(h2) returned error: %v", err)
	}
	if got.Rep() != 2 {
		t.Errorf("got.Rep() = %d, want 2", got.Rep())
	}
}
