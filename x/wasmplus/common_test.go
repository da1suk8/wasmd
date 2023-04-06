package wasmplus

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/line/lbm-sdk/types"
)

// ensure store code returns the expected response
func assertStoreCodeResponse(t *testing.T, data []byte, expected uint64) {
	var pStoreResp MsgStoreCodeResponse
	require.NoError(t, pStoreResp.Unmarshal(data))
	require.Equal(t, pStoreResp.CodeID, expected)
}

// ensures this returns a valid bech32 address and returns it
func parseInitResponse(t *testing.T, data []byte) string {
	var pInstResp MsgInstantiateContractResponse
	require.NoError(t, pInstResp.Unmarshal(data))
	require.NotEmpty(t, pInstResp.Address)
	addr := pInstResp.Address
	// ensure this is a valid sdk address
	_, err := sdk.AccAddressFromBech32(addr)
	require.NoError(t, err)
	return addr
}
