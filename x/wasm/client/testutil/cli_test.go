//go:build norace
// +build norace

package testutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	dbm "github.com/tendermint/tm-db"

	"github.com/line/lbm-sdk/crypto/hd"
	"github.com/line/lbm-sdk/crypto/keyring"
	servertypes "github.com/line/lbm-sdk/server/types"
	storetypes "github.com/line/lbm-sdk/store/types"
	"github.com/line/lbm-sdk/testutil/network"
	sdk "github.com/line/lbm-sdk/types"
	authtypes "github.com/line/lbm-sdk/x/auth/types"
	ostrand "github.com/line/ostracon/libs/rand"

	wasmapp "github.com/line/wasmd/app"
	"github.com/line/wasmd/app/params"
	"github.com/line/wasmd/x/wasm/types"
)

func TestIntegrationTestSuite(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NumValidators = 1
	suite.Run(t, NewIntegrationTestSuite(cfg))
}

func DefaultConfig() network.Config {
	encCfg := wasmapp.MakeEncodingConfig()

	return network.Config{
		Codec:             encCfg.Marshaler,
		LegacyAmino:       encCfg.Amino,
		InterfaceRegistry: encCfg.InterfaceRegistry,
		TxConfig:          encCfg.TxConfig,
		AccountRetriever:  authtypes.AccountRetriever{},
		AppConstructor:    NewAppConstructor(encCfg),
		GenesisState:      wasmapp.ModuleBasics.DefaultGenesis(encCfg.Marshaler),
		TimeoutCommit:     1 * time.Second,
		ChainID:           "chain-" + ostrand.NewRand().Str(6),
		NumValidators:     4,
		BondDenom:         sdk.DefaultBondDenom,
		MinGasPrices:      fmt.Sprintf("0.000006%s", sdk.DefaultBondDenom),
		AccountTokens:     sdk.TokensFromConsensusPower(1000, sdk.DefaultPowerReduction),
		StakingTokens:     sdk.TokensFromConsensusPower(500, sdk.DefaultPowerReduction),
		BondedTokens:      sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction),
		PruningStrategy:   storetypes.PruningOptionNothing,
		CleanupDir:        true,
		SigningAlgo:       string(hd.Secp256k1Type),
		KeyringOptions:    []keyring.Option{},
	}
}

func NewAppConstructor(encodingCfg params.EncodingConfig) network.AppConstructor {
	return func(val network.Validator) servertypes.Application {
		return wasmapp.NewWasmApp(
			val.Ctx.Logger, dbm.NewMemDB(), nil, true,
			map[int64]bool{}, val.Ctx.Config.RootDir, 0,
			encodingCfg,
			types.EnableAllProposals,
			wasmapp.EmptyBaseAppOptions{},
			nil,
		)
	}
}
