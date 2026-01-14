// internal/component/binary/core_instance_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeCoreInstance_Instantiate(t *testing.T) {
	// 0x00 = instantiate, module 0, 1 arg
	data := []byte{
		0x00,                   // instantiate
		0x00,                   // module index 0
		0x01,                   // 1 arg
		0x03, 'm', 'e', 'm',    // arg name "mem"
		0x12,                   // instance sort
		0x00,                   // instance index 0
	}

	ci, err := decodeCoreInstance(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.CoreInstanceExprInstantiate, ci.Kind)
	require.Equal(t, uint32(0), ci.ModuleIdx)
	require.Equal(t, 1, len(ci.Args))
	require.Equal(t, "mem", ci.Args[0].Name)
	require.Equal(t, uint32(0), ci.Args[0].InstanceIdx)
}

func TestDecodeCoreInstance_Inline(t *testing.T) {
	// 0x01 = inline exports
	data := []byte{
		0x01,                       // inline
		0x02,                       // 2 exports
		0x04, 't', 'e', 's', 't',   // name "test"
		0x00,                       // sort: func
		0x05,                       // index 5
		0x03, 'm', 'e', 'm',        // name "mem"
		0x02,                       // sort: memory
		0x01,                       // index 1
	}

	ci, err := decodeCoreInstance(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.CoreInstanceExprInline, ci.Kind)
	require.Equal(t, 2, len(ci.InlineExports))
	require.Equal(t, "test", ci.InlineExports[0].Name)
	require.Equal(t, component.CoreSortFunc, ci.InlineExports[0].Sort)
}

func TestDecodeCoreInstanceSection(t *testing.T) {
	// Section with 1 core instance
	data := []byte{
		0x01,                       // count: 1
		0x00,                       // instantiate
		0x00,                       // module 0
		0x00,                       // 0 args
	}

	c := &component.Component{}
	err := decodeCoreInstanceSection(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, 1, len(c.CoreInstances))
}
