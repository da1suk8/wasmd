package appplus

import (
	"encoding/json"
	abci "github.com/line/ostracon/abci/types"
	"github.com/stretchr/testify/require"
	"os"
	"testing"

	db "github.com/tendermint/tm-db"

	"github.com/line/ostracon/libs/log"

	wasmapp "github.com/line/wasmd/app"
	wasmplustypes "github.com/line/wasmd/x/wasmplus/types"
)

func TestZeroHeightGenesis(t *testing.T) {
	db := db.NewMemDB()
	gapp := NewWasmApp(log.NewOCLogger(log.NewSyncWriter(os.Stdout)), db, nil, true, map[int64]bool{}, DefaultNodeHome, 0, MakeEncodingConfig(), wasmplustypes.EnableAllProposals, wasmapp.EmptyBaseAppOptions{}, emptyWasmOpts)

	genesisState := NewDefaultGenesisState()
	stateBytes, err := json.MarshalIndent(genesisState, "", "  ")
	require.NoError(t, err)

	// Initialize the chain
	gapp.InitChain(
		abci.RequestInitChain{
			Validators:    []abci.ValidatorUpdate{},
			AppStateBytes: stateBytes,
		},
	)
	gapp.Commit()

	jailAllowedAddress := []string{"linkvaloper12kr02kew9fl73rqekalavuu0xaxcgwr6pz5vt8"}
	_, err = gapp.ExportAppStateAndValidators(true, jailAllowedAddress)
	require.NoError(t, err)
}
