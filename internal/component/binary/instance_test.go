// internal/component/binary/instance_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeComponentInstance_Instantiate(t *testing.T) {
	// 0x00 = instantiate, component 0, 1 arg
	data := []byte{
		0x00,                               // instantiate
		0x00,                               // component index 0
		0x01,                               // 1 arg
		0x07, 's', 't', 'r', 'e', 'a', 'm', 's', // arg name "streams"
		0x05,                               // sort: instance
		0x01,                               // index 1
	}

	ci, err := decodeComponentInstance(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ComponentInstanceExprInstantiate, ci.Kind)
	require.Equal(t, uint32(0), ci.ComponentIdx)
	require.Equal(t, 1, len(ci.Args))
	require.Equal(t, "streams", ci.Args[0].Name)
	require.Equal(t, component.SortInstance, ci.Args[0].Sort)
	require.Equal(t, uint32(1), ci.Args[0].Idx)
}

func TestDecodeComponentInstance_Inline(t *testing.T) {
	// 0x01 = inline exports
	data := []byte{
		0x01,                     // inline
		0x01,                     // 1 export
		0x04, 't', 'e', 's', 't', // name "test"
		0x01,                     // sort: func
		0x05,                     // index 5
	}

	ci, err := decodeComponentInstance(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ComponentInstanceExprInline, ci.Kind)
	require.Equal(t, 1, len(ci.InlineExports))
	require.Equal(t, "test", ci.InlineExports[0].Name)
	require.Equal(t, component.SortFunc, ci.InlineExports[0].Sort)
}

func TestDecodeInstanceSection(t *testing.T) {
	// Section with 1 component instance
	data := []byte{
		0x01, // count: 1
		0x00, // instantiate
		0x00, // component 0
		0x00, // 0 args
	}

	c := &component.Component{}
	err := decodeInstanceSection(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, 1, len(c.ComponentInstances))
}
