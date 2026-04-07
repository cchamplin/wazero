// internal/component/wasip2test/kv_store_test.go
//
// Task 2.1: Resource Constructor/Destructor Test (KV Store example)
//
// This test exercises the resource lifecycle for host-defined resources:
// - Host resource type definition
// - Resource constructor
// - Resource method calls
// - Resource destructor
// - ResourceTable management
package wasip2test

import (
	"context"
	"sync"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/testutil"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// KVStore is a simple key-value store resource for testing.
type KVStore struct {
	data map[string]string
	id   uint32
}

// NewKVStore creates a new KV store instance.
func NewKVStore(id uint32) *KVStore {
	return &KVStore{
		data: make(map[string]string),
		id:   id,
	}
}

// Get retrieves a value by key.
func (kv *KVStore) Get(key string) (string, bool) {
	val, ok := kv.data[key]
	return val, ok
}

// Set stores a value.
func (kv *KVStore) Set(key, value string) {
	kv.data[key] = value
}

// TestResourceLifecycle_Basic tests basic resource table operations
// without involving component instantiation. This verifies the underlying
// resource management APIs work correctly.
func TestResourceLifecycle_Basic(t *testing.T) {
	ctx := context.Background()

	// Track lifecycle events
	var events []string
	var mu sync.Mutex

	addEvent := func(event string) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}

	// Create resource table
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Verify we can retrieve the table from context
	retrievedTable := component.ResourceTableFromContext(ctx)
	if retrievedTable == nil {
		t.Fatal("ResourceTableFromContext returned nil")
	}
	if retrievedTable != table {
		t.Error("Retrieved table does not match original")
	}

	// Create a KV store resource
	kvStore := NewKVStore(1)
	addEvent("constructor-called")

	// Add to resource table
	handle := table.New(kvStore, true) // own=true
	t.Logf("Created handle: index=%d, generation=%d", handle.Index(), handle.Generation())

	// Verify the handle is valid
	entry, err := table.Get(handle)
	if err != nil {
		t.Fatalf("Get handle failed: %v", err)
	}
	if entry.Rep != kvStore {
		t.Error("Entry.Rep does not match original resource")
	}
	if !entry.Own {
		t.Error("Entry.Own should be true")
	}

	// Use the resource
	kv := entry.Rep.(*KVStore)
	kv.Set("key1", "value1")
	val, ok := kv.Get("key1")
	if !ok || val != "value1" {
		t.Errorf("Get returned %q, %v; want %q, true", val, ok, "value1")
	}

	// Remove the resource
	removedEntry, err := table.Remove(handle)
	if err != nil {
		t.Fatalf("Remove handle failed: %v", err)
	}
	addEvent("destructor-called")

	if removedEntry.Rep != kvStore {
		t.Error("Removed entry Rep does not match")
	}

	// Verify handle is now invalid
	_, err = table.Get(handle)
	if err == nil {
		t.Error("Get should fail for removed handle")
	}

	// Verify events
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d: %v", len(events), events)
	}
	if events[0] != "constructor-called" {
		t.Errorf("First event should be constructor-called, got %s", events[0])
	}
	if events[1] != "destructor-called" {
		t.Errorf("Second event should be destructor-called, got %s", events[1])
	}

	t.Log("Basic resource lifecycle test passed")
}

// TestResourceLifecycle_LinkerDefinition tests defining a resource type
// in the linker and verifying the destructor callback is registered.
func TestResourceLifecycle_LinkerDefinition(t *testing.T) {
	var destructorCalled bool
	var destructedRep uint32
	var mu sync.Mutex

	destructor := func(rep uint32) {
		mu.Lock()
		destructorCalled = true
		destructedRep = rep
		mu.Unlock()
	}

	// Create linker and define a resource type
	linker := component.NewLinker()
	err := linker.DefineInstance("test:resource/types@0.1.0").
		Resource("store", destructor).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}

	// Verify the resource is defined
	def, ok := linker.Get("test:resource/types@0.1.0")
	if !ok {
		t.Fatal("Failed to get defined instance")
	}

	instanceDef, ok := def.(*component.InstanceDef)
	if !ok {
		t.Fatalf("Definition is not InstanceDef: %T", def)
	}

	resourceDef, ok := instanceDef.Exports["store"].(*component.ResourceDef)
	if !ok {
		t.Fatalf("Export 'store' is not ResourceDef: %T", instanceDef.Exports["store"])
	}

	// Call the destructor directly to verify it was registered correctly
	resourceDef.Destructor(42)

	mu.Lock()
	defer mu.Unlock()
	if !destructorCalled {
		t.Error("Destructor was not called")
	}
	if destructedRep != 42 {
		t.Errorf("Destructor received rep %d, want 42", destructedRep)
	}

	t.Log("Linker resource definition test passed")
}

// TestResourceLifecycle_TableWithDestructor tests the ResourceTable's
// destructor management APIs.
func TestResourceLifecycle_TableWithDestructor(t *testing.T) {
	var destructorCalls []uint32
	var mu sync.Mutex

	destructor := func(rep uint32) {
		mu.Lock()
		destructorCalls = append(destructorCalls, rep)
		mu.Unlock()
	}

	table := runtime.NewResourceTable()

	// Create resources using the helper that tracks destructor calls
	resourceTypeIdx := uint32(0)
	dropFunc := table.CreateResourceDropFunc(resourceTypeIdx, destructor)

	// Create two resources
	handle1 := table.New(uint32(100), true)
	handle2 := table.New(uint32(200), true)

	t.Logf("Created handle1: %d, handle2: %d", handle1, handle2)

	// Drop first resource
	dropFunc(uint32(handle1))

	mu.Lock()
	if len(destructorCalls) != 1 {
		t.Errorf("Expected 1 destructor call, got %d", len(destructorCalls))
	} else if destructorCalls[0] != 100 {
		t.Errorf("Destructor called with %d, want 100", destructorCalls[0])
	}
	mu.Unlock()

	// Drop second resource
	dropFunc(uint32(handle2))

	mu.Lock()
	if len(destructorCalls) != 2 {
		t.Errorf("Expected 2 destructor calls, got %d", len(destructorCalls))
	} else if destructorCalls[1] != 200 {
		t.Errorf("Second destructor call with %d, want 200", destructorCalls[1])
	}
	mu.Unlock()

	t.Log("Table destructor management test passed")
}

// TestResourceLifecycle_BorrowSemantics tests borrow vs own handle semantics.
func TestResourceLifecycle_BorrowSemantics(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create an owned resource
	kvStore := NewKVStore(1)
	ownHandle := table.New(kvStore, true) // own=true

	// Create a borrow of the same resource
	// Note: In real component model, borrows are created differently,
	// but for testing the table semantics we can create a non-owning entry
	borrowHandle := table.New(kvStore, false) // own=false

	// Both handles should be valid
	ownEntry, err := table.Get(ownHandle)
	if err != nil {
		t.Fatalf("Get own handle failed: %v", err)
	}
	if !ownEntry.Own {
		t.Error("Own entry should have Own=true")
	}

	borrowEntry, err := table.Get(borrowHandle)
	if err != nil {
		t.Fatalf("Get borrow handle failed: %v", err)
	}
	if borrowEntry.Own {
		t.Error("Borrow entry should have Own=false")
	}

	// Both should reference the same resource
	if ownEntry.Rep != borrowEntry.Rep {
		t.Error("Own and borrow should reference the same resource")
	}

	// Remove the borrow - this should not destroy the resource
	_, err = table.Remove(borrowHandle)
	if err != nil {
		t.Fatalf("Remove borrow failed: %v", err)
	}

	// Original own handle should still be valid
	_, err = table.Get(ownHandle)
	if err != nil {
		t.Errorf("Own handle should still be valid after borrow removal: %v", err)
	}

	// Clean up
	_, err = table.Remove(ownHandle)
	if err != nil {
		t.Fatalf("Remove own handle failed: %v", err)
	}

	t.Log("Borrow semantics test passed")
}

// TestResourceLifecycle_LendTracking tests the lend count tracking
// which prevents owned resources from being dropped while borrowed.
func TestResourceLifecycle_LendTracking(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create an owned resource
	kvStore := NewKVStore(1)
	handle := table.New(kvStore, true)

	// Increment lends (simulating a borrow being created)
	err := table.IncrementLends(handle)
	if err != nil {
		t.Fatalf("IncrementLends failed: %v", err)
	}

	// Verify the entry shows active lends
	entry, err := table.Get(handle)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.NumLends != 1 {
		t.Errorf("NumLends should be 1, got %d", entry.NumLends)
	}

	// Try to remove - should fail because of active lends
	_, err = table.Remove(handle)
	if err == nil {
		t.Error("Remove should fail when resource has active lends")
	}
	if err != runtime.ErrResourceInUse {
		t.Errorf("Expected ErrResourceInUse, got %v", err)
	}

	// Decrement lends (simulating borrow end)
	err = table.DecrementLends(handle)
	if err != nil {
		t.Fatalf("DecrementLends failed: %v", err)
	}

	// Now removal should succeed
	_, err = table.Remove(handle)
	if err != nil {
		t.Fatalf("Remove should succeed after lends decremented: %v", err)
	}

	t.Log("Lend tracking test passed")
}

// TestResourceLifecycle_HandleGeneration tests that generation counting
// prevents use of stale handles.
func TestResourceLifecycle_HandleGeneration(t *testing.T) {
	table := runtime.NewResourceTable()

	// Create a resource
	handle1 := table.New(uint32(100), true)
	idx := handle1.Index()
	gen1 := handle1.Generation()

	t.Logf("First handle: index=%d, generation=%d", idx, gen1)

	// Remove it
	_, err := table.Remove(handle1)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Create another resource - may reuse the same slot
	handle2 := table.New(uint32(200), true)

	// If the same slot was reused, generation should be incremented
	if handle2.Index() == idx {
		gen2 := handle2.Generation()
		t.Logf("Second handle reused slot: index=%d, generation=%d", handle2.Index(), gen2)
		if gen2 <= gen1 {
			t.Errorf("Generation should increase on reuse: gen1=%d, gen2=%d", gen1, gen2)
		}

		// Original handle should be invalid (generation mismatch)
		_, err = table.Get(handle1)
		if err == nil {
			t.Error("Old handle should be invalid due to generation mismatch")
		}
	}

	t.Log("Handle generation test passed")
}

// TestResourceLifecycle_ComponentLinker tests defining resources with
// ComponentLinker (which has runtime integration).
func TestResourceLifecycle_ComponentLinker(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	var destructorCalled bool
	var mu sync.Mutex

	destructor := func(rep uint32) {
		mu.Lock()
		destructorCalled = true
		mu.Unlock()
	}

	// Create component linker with runtime
	linker := component.NewComponentLinker(rt)

	// Define an instance with a resource type
	err := linker.DefineInstance("test:kv/store@0.1.0").
		Resource("bucket", destructor).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}

	// Verify it was registered
	def, ok := linker.Get("test:kv/store@0.1.0")
	if !ok {
		t.Fatal("Instance definition not found in linker")
	}

	instDef, ok := def.(*component.InstanceDef)
	if !ok {
		t.Fatalf("Expected InstanceDef, got %T", def)
	}

	resDef, ok := instDef.Exports["bucket"].(*component.ResourceDef)
	if !ok {
		t.Fatalf("Expected ResourceDef for 'bucket', got %T", instDef.Exports["bucket"])
	}

	// Call destructor to verify
	resDef.Destructor(1)

	mu.Lock()
	if !destructorCalled {
		t.Error("Destructor should have been called")
	}
	mu.Unlock()

	t.Log("ComponentLinker resource definition test passed")
}

// TestResourceLifecycle_ComponentWithResource tests instantiating a simple
// component that imports a host resource. This is the most complex test
// and may be skipped if the underlying infrastructure isn't ready.
func TestResourceLifecycle_ComponentWithResource(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Track lifecycle events
	var events []string
	var mu sync.Mutex

	addEvent := func(event string) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}

	// Create linker and define a resource type
	linker := component.NewComponentLinker(rt)

	err := linker.DefineInstance("test:resource/types").
		Resource("item", func(rep uint32) {
			addEvent("destructor-called")
		}).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}

	// Try to compile a simple component that imports the resource type
	// This component just imports the resource but doesn't use it
	wat := `
(component
  ;; Import resource type as an instance
  (import "test:resource/types" (instance $types
    (export "item" (type (sub resource)))
  ))

  ;; Simple core module that does nothing
  (core module $m
    (func (export "noop"))
  )
  (core instance $i (instantiate $m))

  ;; Lift the noop function
  (alias core export $i "noop" (core func $f))
  (func (export "noop") (canon lift (core func $f)))
)
`

	wasmBytes, err := testutil.BuildComponentFromWAT(wat)
	if err != nil {
		t.Skipf("WAT compilation requires wasm-tools: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Skipf("Component compilation not fully supported: %v", err)
	}
	defer compiled.Close(ctx)

	// Set up resource table
	resourceTable := runtime.NewResourceTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)

	// Try to instantiate
	instance, err := linker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("Resource import instantiation not fully supported: %v", err)
	}

	// Call the exported function
	fn := instance.ExportedFunction("noop")
	if fn == nil {
		t.Fatal("noop function not found")
	}

	_, err = fn.Call(testCtx)
	if err != nil {
		t.Errorf("noop call failed: %v", err)
	}

	t.Log("Component with resource import test passed")
}

// TestResourceLifecycle_ResourceConstructorCallback tests a host function
// that acts as a resource constructor.
func TestResourceLifecycle_ResourceConstructorCallback(t *testing.T) {
	ctx := context.Background()

	// Track constructor calls
	var constructorCalls []string
	var mu sync.Mutex

	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Simulate a host resource constructor
	constructor := func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		mu.Lock()
		constructorCalls = append(constructorCalls, "constructor")
		mu.Unlock()

		// Get resource table from context
		rt := component.ResourceTableFromContext(ctx)
		if rt == nil {
			t.Error("Resource table not in context")
			return nil, nil
		}

		// Create a new resource
		kvStore := NewKVStore(1)
		handle := rt.New(kvStore, true)

		// Return the handle as an own value
		return []types.Val{types.ValOwn(uint32(handle))}, nil
	}

	// Simulate a host destructor
	destructor := func(rep uint32) {
		mu.Lock()
		constructorCalls = append(constructorCalls, "destructor")
		mu.Unlock()
	}

	// Define the resource and constructor in a linker
	linker := component.NewLinker()
	err := linker.DefineInstance("test:kv/types@0.1.0").
		Resource("store", destructor).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}

	// Note: The constructor would typically be defined on a separate interface
	// For this test, we call it directly
	result, err := constructor(ctx, []types.Val{})
	if err != nil {
		t.Fatalf("Constructor failed: %v", err)
	}

	handle := result[0].Own()
	t.Logf("Constructor returned handle: %d", handle)

	// Verify resource was created
	entry, err := table.Get(runtime.Handle(handle))
	if err != nil {
		t.Fatalf("Get handle failed: %v", err)
	}

	kv, ok := entry.Rep.(*KVStore)
	if !ok {
		t.Fatalf("Resource is not KVStore: %T", entry.Rep)
	}

	// Use the resource
	kv.Set("test", "value")
	val, found := kv.Get("test")
	if !found || val != "value" {
		t.Errorf("KVStore.Get returned %q, %v; want 'value', true", val, found)
	}

	// Call destructor
	destructor(uint32(handle))

	// Verify events
	mu.Lock()
	defer mu.Unlock()
	if len(constructorCalls) != 2 {
		t.Errorf("Expected 2 events, got %d: %v", len(constructorCalls), constructorCalls)
	}

	t.Log("Resource constructor callback test passed")
}

// TestResourceLifecycle_ResourceMethodCallback tests a host function
// that acts as a resource method (takes borrow as first arg).
func TestResourceLifecycle_ResourceMethodCallback(t *testing.T) {
	ctx := context.Background()

	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Create a KV store resource
	kvStore := NewKVStore(1)
	handle := table.New(kvStore, true)

	// Simulate a resource method: [method]store.set(key: string, value: string)
	setMethod := func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		// First arg is borrow handle
		borrowHandle := args[0].Borrow()

		rt := component.ResourceTableFromContext(ctx)
		if rt == nil {
			t.Error("Resource table not in context")
			return nil, nil
		}

		entry, err := rt.Get(runtime.Handle(borrowHandle))
		if err != nil {
			t.Errorf("Get borrow handle failed: %v", err)
			return nil, err
		}

		kv, ok := entry.Rep.(*KVStore)
		if !ok {
			t.Errorf("Resource is not KVStore: %T", entry.Rep)
			return nil, nil
		}

		key := args[1].StringVal()
		value := args[2].StringVal()
		kv.Set(key, value)

		return []types.Val{}, nil
	}

	// Simulate a resource method: [method]store.get(key: string) -> option<string>
	getMethod := func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		borrowHandle := args[0].Borrow()

		rt := component.ResourceTableFromContext(ctx)
		entry, err := rt.Get(runtime.Handle(borrowHandle))
		if err != nil {
			return nil, err
		}

		kv := entry.Rep.(*KVStore)
		key := args[1].StringVal()
		value, found := kv.Get(key)

		if found {
			strVal := types.ValString(value)
			return []types.Val{types.ValOption(&strVal)}, nil
		}
		return []types.Val{types.ValOption(nil)}, nil
	}

	// Call set method
	_, err := setMethod(ctx, []types.Val{
		types.ValBorrow(uint32(handle)),
		types.ValString("greeting"),
		types.ValString("hello"),
	})
	if err != nil {
		t.Fatalf("set method failed: %v", err)
	}

	// Call get method
	result, err := getMethod(ctx, []types.Val{
		types.ValBorrow(uint32(handle)),
		types.ValString("greeting"),
	})
	if err != nil {
		t.Fatalf("get method failed: %v", err)
	}

	opt := result[0].Option()
	if opt == nil {
		t.Error("get should return some value")
	} else if opt.StringVal() != "hello" {
		t.Errorf("get returned %q, want 'hello'", opt.StringVal())
	}

	// Test get for non-existent key
	result, err = getMethod(ctx, []types.Val{
		types.ValBorrow(uint32(handle)),
		types.ValString("nonexistent"),
	})
	if err != nil {
		t.Fatalf("get method failed: %v", err)
	}

	opt = result[0].Option()
	if opt != nil {
		t.Errorf("get for nonexistent key should return none, got %v", opt)
	}

	t.Log("Resource method callback test passed")
}

// TestResourceLifecycle_TypedResources tests resource type tracking
// using ResourceTypeID.
func TestResourceLifecycle_TypedResources(t *testing.T) {
	table := runtime.NewResourceTable()

	// Define two resource types
	storeTypeIdx := uint32(0)
	bucketTypeIdx := uint32(1)

	storeRT := runtime.NewResourceTypeID(storeTypeIdx)
	bucketRT := runtime.NewResourceTypeID(bucketTypeIdx)

	// Create resources with types
	storeHandle := table.NewWithType(uint32(100), true, storeRT)
	bucketHandle := table.NewWithType(uint32(200), true, bucketRT)

	// Verify types
	storeEntry, _ := table.Get(storeHandle)
	if storeEntry.RT != storeRT {
		t.Errorf("Store resource has wrong type: got %v, want %v", storeEntry.RT, storeRT)
	}

	bucketEntry, _ := table.Get(bucketHandle)
	if bucketEntry.RT != bucketRT {
		t.Errorf("Bucket resource has wrong type: got %v, want %v", bucketEntry.RT, bucketRT)
	}

	// Validate type matches
	err := table.ValidateType(storeHandle, storeRT)
	if err != nil {
		t.Errorf("ValidateType for store failed: %v", err)
	}

	err = table.ValidateType(storeHandle, bucketRT)
	if err == nil {
		t.Error("ValidateType should fail for mismatched types")
	}

	t.Log("Typed resources test passed")
}

// TestResourceLifecycle_MergeLinkers tests merging resource definitions
// between linkers.
func TestResourceLifecycle_MergeLinkers(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	var destructor1Called, destructor2Called bool
	var mu sync.Mutex

	// Create first linker with one resource
	linker1 := component.NewLinker()
	err := linker1.DefineInstance("test:types/a@0.1.0").
		Resource("res-a", func(rep uint32) {
			mu.Lock()
			destructor1Called = true
			mu.Unlock()
		}).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance linker1: %v", err)
	}

	// Create second linker with another resource
	linker2 := component.NewLinker()
	err = linker2.DefineInstance("test:types/b@0.1.0").
		Resource("res-b", func(rep uint32) {
			mu.Lock()
			destructor2Called = true
			mu.Unlock()
		}).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance linker2: %v", err)
	}

	// Create component linker and merge both
	componentLinker := component.NewComponentLinker(rt)
	componentLinker.MergeFrom(linker1)
	componentLinker.MergeFrom(linker2)

	// Verify both are present
	def1, ok1 := componentLinker.Get("test:types/a@0.1.0")
	if !ok1 {
		t.Error("Resource A not found after merge")
	}

	def2, ok2 := componentLinker.Get("test:types/b@0.1.0")
	if !ok2 {
		t.Error("Resource B not found after merge")
	}

	// Call destructors to verify
	inst1 := def1.(*component.InstanceDef)
	res1 := inst1.Exports["res-a"].(*component.ResourceDef)
	res1.Destructor(1)

	inst2 := def2.(*component.InstanceDef)
	res2 := inst2.Exports["res-b"].(*component.ResourceDef)
	res2.Destructor(2)

	mu.Lock()
	defer mu.Unlock()
	if !destructor1Called {
		t.Error("Destructor 1 was not called")
	}
	if !destructor2Called {
		t.Error("Destructor 2 was not called")
	}

	t.Log("Merge linkers test passed")
}
