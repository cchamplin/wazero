// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeStreamType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x66,       // stream opcode
		0x01, 0x7d, // has element type: u8
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.Streams))
	s := c.Types.Streams[0]
	require.True(t, s.HasElement)
	require.Equal(t, types.U8, s.Element)
}

func TestDecodeStreamType_NoElement(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x66, // stream opcode
		0x00, // no element type
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.Streams))
	require.False(t, c.Types.Streams[0].HasElement)
}

func TestDecodeFutureType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x65,       // future opcode
		0x01, 0x73, // has payload: string
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.Futures))
	f := c.Types.Futures[0]
	require.True(t, f.HasElement)
	require.Equal(t, types.String_, f.Element)
}

func TestDecodeFutureType_NoPayload(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x65, // future opcode
		0x00, // no payload
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.Futures))
	require.False(t, c.Types.Futures[0].HasElement)
}

func TestDecodeFixedSizeListType(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x67, // fixed-size list opcode
		0x7d, // element type: u8
		0x10, // size: 16
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.FixedLists))
	fl := c.Types.FixedLists[0]
	require.Equal(t, uint32(16), fl.Length)
	require.Equal(t, types.U8, fl.Element)
}

func TestDecodeFixedSizeListType_LargeSize(t *testing.T) {
	data := buildComponentWithTypeSection([]byte{
		0x67,             // fixed-size list opcode
		0x7a,             // element type: s32
		0x80, 0x80, 0x04, // size: 65536 (LEB128)
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.FixedLists))
	fl := c.Types.FixedLists[0]
	require.Equal(t, uint32(65536), fl.Length)
	require.Equal(t, types.S32, fl.Element)
}
