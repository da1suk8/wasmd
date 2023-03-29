package wasmplus

import (
	"context"
	"encoding/json"
	"math/rand"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/line/lbm-sdk/client"
	"github.com/line/lbm-sdk/codec"
	cdctypes "github.com/line/lbm-sdk/codec/types"
	sdk "github.com/line/lbm-sdk/types"
	sdkerrors "github.com/line/lbm-sdk/types/errors"
	"github.com/line/lbm-sdk/types/module"
	simtypes "github.com/line/lbm-sdk/types/simulation"
	abci "github.com/line/ostracon/abci/types"

	wasmkeeper "github.com/line/wasmd/x/wasm/keeper"
	"github.com/line/wasmd/x/wasm/simulation"
	wasmtypes "github.com/line/wasmd/x/wasm/types"
	"github.com/line/wasmd/x/wasmplus/client/cli"
	"github.com/line/wasmd/x/wasmplus/keeper"
	"github.com/line/wasmd/x/wasmplus/types"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
)

// AppModuleBasic defines the basic application module used by the wasm module.
type AppModuleBasic struct{}

// Name returns the wasm module's name.
func (a AppModuleBasic) Name() string {
	return types.ModuleName
}

func (a AppModuleBasic) RegisterLegacyAminoCodec(amino *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(amino)
}

// RegisterInterfaces implements InterfaceModule
func (a AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

// DefaultGenesis returns default genesis state as raw bytes for the wasm module.
func (a AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(&types.GenesisState{
		Params: wasmtypes.DefaultParams(),
	})
}

// ValidateGenesis performs genesis state validation for the wasm module.
func (a AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, message json.RawMessage) error {
	var data types.GenesisState
	if err := cdc.UnmarshalJSON(message, &data); err != nil {
		return sdkerrors.Wrap(err, "validate genesis")
	}
	return data.ValidateBasic()
}

func (a AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, serveMux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), serveMux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
	if err := wasmtypes.RegisterQueryHandlerClient(context.Background(), serveMux, wasmtypes.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// GetTxCmd returns the root tx command for the wasm module.
func (a AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

// GetQueryCmd returns no root query command for the wasm module.
func (a AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

type AppModule struct {
	AppModuleBasic
	cdc                codec.Codec
	keeper             *keeper.Keeper
	validatorSetSource wasmkeeper.ValidatorSetSource
	accountKeeper      wasmtypes.AccountKeeper // for simulation
	bankKeeper         simulation.BankKeeper
}

func NewAppModule(
	cdc codec.Codec,
	keeper *keeper.Keeper,
	vs wasmkeeper.ValidatorSetSource,
	ak wasmtypes.AccountKeeper,
	bk simulation.BankKeeper,
) AppModule {
	return AppModule{
		AppModuleBasic:     AppModuleBasic{},
		cdc:                cdc,
		keeper:             keeper,
		validatorSetSource: vs,
		accountKeeper:      ak,
		bankKeeper:         bk,
	}
}

func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) []abci.ValidatorUpdate {
	var genesisState types.GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	validators, err := keeper.InitGenesis(ctx, am.keeper, genesisState, am.validatorSetSource, am.Route().Handler())
	if err != nil {
		panic(err)
	}
	return validators
}

// ExportGenesis returns the exported genesis state as raw bytes for the wasm module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := keeper.ExportGenesis(ctx, am.keeper)
	return cdc.MustMarshalJSON(gs)
}

// RegisterInvariants registers the wasm module invariants.
func (am AppModule) RegisterInvariants(registry sdk.InvariantRegistry) {}

// Route returns the message routing key for the wasm module.
func (am AppModule) Route() sdk.Route {
	return sdk.NewRoute(types.RouterKey,
		NewHandler(keeper.NewPermissionedKeeper(*wasmkeeper.NewDefaultPermissionKeeper(am.keeper), am.keeper)))
}

// QuerierRoute returns the wasm module's querier route name.
func (am AppModule) QuerierRoute() string {
	return wasmtypes.QuerierRoute
}

func (am AppModule) LegacyQuerierHandler(amino *codec.LegacyAmino) sdk.Querier {
	return wasmkeeper.NewLegacyQuerier(am.keeper, am.keeper.QueryGasLimit())
}

func (am AppModule) RegisterServices(cfg module.Configurator) {
	// wasmplus service
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(wasmkeeper.NewDefaultPermissionKeeper(am.keeper)))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.Querier(am.keeper))
	// wasm service
	wasmtypes.RegisterMsgServer(cfg.MsgServer(), wasmkeeper.NewMsgServerImpl(wasmkeeper.NewDefaultPermissionKeeper(am.keeper)))
	wasmtypes.RegisterQueryServer(cfg.QueryServer(), keeper.WasmQuerier(am.keeper))
}

// ConsensusVersion is a sequence number for state-breaking change of the
// module. It should be incremented on each consensus-breaking change
// introduced by the module. To avoid wrong/empty versions, the initial version
// should be set to 1.
func (am AppModule) ConsensusVersion() uint64 {
	return 1
}

// ____________________________________________________________________________

// AppModuleSimulation functions

// GenerateGenesisState creates a randomized GenState of the bank module.
func (am AppModule) GenerateGenesisState(simState *module.SimulationState) {
	simulation.RandomizedGenState(simState)
}

// ProposalContents doesn't return any content functions for governance proposals.
func (am AppModule) ProposalContents(simState module.SimulationState) []simtypes.WeightedProposalContent {
	return nil
}

// RandomizedParams creates randomized bank param changes for the simulator.
func (am AppModule) RandomizedParams(r *rand.Rand) []simtypes.ParamChange {
	return simulation.ParamChanges(r, am.cdc)
}

// RegisterStoreDecoder registers a decoder for supply module's types
func (am AppModule) RegisterStoreDecoder(registry sdk.StoreDecoderRegistry) {
}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	return simulation.WeightedOperations(&simState, am.accountKeeper, am.bankKeeper, am.keeper)
}
