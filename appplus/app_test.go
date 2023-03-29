package appplus

import (
	"encoding/json"
	"github.com/line/lbm-sdk/server"
	wasmapp "github.com/line/wasmd/app"
	wasmkeeper "github.com/line/wasmd/x/wasm/keeper"
	wasmtypes "github.com/line/wasmd/x/wasm/types"
	wasmplustypes "github.com/line/wasmd/x/wasmplus/types"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	db "github.com/tendermint/tm-db"

	abci "github.com/line/ostracon/abci/types"
	"github.com/line/ostracon/libs/log"
)

var emptyWasmOpts []wasmkeeper.Option = nil

func TestWasmdExport(t *testing.T) {
	db := db.NewMemDB()
	gapp := NewWasmApp(log.NewOCLogger(log.NewSyncWriter(os.Stdout)), db, nil, true, map[int64]bool{}, DefaultNodeHome, 0, MakeEncodingConfig(), wasmplustypes.EnableAllProposals, wasmapp.EmptyBaseAppOptions{}, emptyWasmOpts)
	require.Equal(t, appName, gapp.Name())

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

	// Making a new app object with the db, so that initchain hasn't been called
	newGapp := NewWasmApp(log.NewOCLogger(log.NewSyncWriter(os.Stdout)), db, nil, true, map[int64]bool{}, DefaultNodeHome, 0, MakeEncodingConfig(), wasmplustypes.EnableAllProposals, wasmapp.EmptyBaseAppOptions{}, emptyWasmOpts)
	_, err = newGapp.ExportAppStateAndValidators(false, []string{})
	require.NoError(t, err, "ExportAppStateAndValidators should not have an error")
}

// ensure that blocked addresses are properly set in bank keeper
func TestBlockedAddrs(t *testing.T) {
	db := db.NewMemDB()
	gapp := NewWasmApp(log.NewOCLogger(log.NewSyncWriter(os.Stdout)), db, nil, true, map[int64]bool{}, DefaultNodeHome, 0, MakeEncodingConfig(), wasmplustypes.EnableAllProposals, wasmapp.EmptyBaseAppOptions{}, emptyWasmOpts)
	blockedAddrs := gapp.BlockedAddrs()

	for acc := range maccPerms {
		t.Run(acc, func(t *testing.T) {
			addr := gapp.AccountKeeper.GetModuleAddress(acc)
			if blockedAddrs[addr.String()] {
				require.True(t, gapp.BankKeeper.BlockedAddr(addr),
					"ensure that blocked addresses are properly set in bank keeper",
				)
			}
		})
	}
}

// EmptyBaseAppOptions is a stub implementing AppOptions
type WrongWasmAppOptions struct{}

// Get implements AppOptions
func (ao WrongWasmAppOptions) Get(o string) interface{} {
	if o == server.FlagTrace {
		// make fail case.
		return "FALse"
	}
	return nil
}

func TestWrongWasmAppOptionsNewWasmApp(t *testing.T) {
	require.PanicsWithValue(t,
		"error while reading wasm config: strconv.ParseBool: parsing \"FALse\": invalid syntax",
		func() {
			NewWasmApp(
				log.NewOCLogger(log.NewSyncWriter(os.Stdout)),
				nil,
				nil,
				true,
				map[int64]bool{},
				DefaultNodeHome,
				0,
				MakeEncodingConfig(),
				wasmplustypes.EnableAllProposals,
				WrongWasmAppOptions{},
				emptyWasmOpts,
			)
		})
}

func TestGetMaccPerms(t *testing.T) {
	dup := GetMaccPerms()
	require.Equal(t, maccPerms, dup, "duplicated module account permissions differed from actual module account permissions")
}

func TestGetEnabledProposals(t *testing.T) {
	cases := map[string]struct {
		proposalsEnabled string
		specificEnabled  string
		expected         []wasmtypes.ProposalType
	}{
		"all disabled": {
			proposalsEnabled: "false",
			expected:         wasmtypes.DisableAllProposals,
		},
		"all enabled": {
			proposalsEnabled: "true",
			expected:         wasmplustypes.EnableAllProposals,
		},
		"some enabled": {
			proposalsEnabled: "okay",
			specificEnabled:  "StoreCode,InstantiateContract",
			expected:         []wasmtypes.ProposalType{wasmtypes.ProposalTypeStoreCode, wasmtypes.ProposalTypeInstantiateContract},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ProposalsEnabled = tc.proposalsEnabled
			EnableSpecificProposals = tc.specificEnabled
			proposals := GetEnabledProposals()
			assert.Equal(t, tc.expected, proposals)
		})
	}
}

func TestGetEnabledProposalsPanic(t *testing.T) {
	EnableSpecificProposals = "WrongMsg"
	assert.Panics(t, func() {
		GetEnabledProposals()
	})
}

func setGenesis(gapp *WasmPlusApp) error {
	genesisState := NewDefaultGenesisState()
	stateBytes, err := json.MarshalIndent(genesisState, "", " ")
	if err != nil {
		return err
	}

	// Initialize the chain
	gapp.InitChain(
		abci.RequestInitChain{
			Validators:    []abci.ValidatorUpdate{},
			AppStateBytes: stateBytes,
		},
	)

	gapp.Commit()
	return nil
}
