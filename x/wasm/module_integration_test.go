package wasm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/line/lbm-sdk/types"
	"github.com/line/lbm-sdk/types/module"
	upgradetypes "github.com/line/lbm-sdk/x/upgrade/types"
	ocproto "github.com/line/ostracon/proto/ostracon/types"

	"github.com/line/wasmd/app"
	"github.com/line/wasmd/x/wasm"
)

func TestModuleMigrations(t *testing.T) {
	wasmApp := app.Setup(false)
	ctx := wasmApp.BaseApp.NewContext(false, ocproto.Header{})
	upgradeHandler := func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return wasmApp.ModuleManager().RunMigrations(ctx, wasmApp.ModuleConfigurator(), fromVM)
	}
	fromVM := wasmApp.UpgradeKeeper.GetModuleVersionMap(ctx)
	fromVM[wasm.ModuleName] = 1 // start with initial version
	upgradeHandler(ctx, upgradetypes.Plan{Name: "testing"}, fromVM)
	// when
	gotVM, err := wasmApp.ModuleManager().RunMigrations(ctx, wasmApp.ModuleConfigurator(), fromVM)
	// then
	require.NoError(t, err)
	assert.Equal(t, uint64(1), gotVM[wasm.ModuleName])
}
