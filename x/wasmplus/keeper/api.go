package keeper

import (
	"fmt"

	sdk "github.com/line/lbm-sdk/types"
	wasmkeeper "github.com/line/wasmd/x/wasm/keeper"

	// wasmpluskeeper "github.com/line/wasmd/x/wasmplus/keeper"
	"github.com/line/wasmd/x/wasm/types"
	wasmplustypes "github.com/line/wasmd/x/wasmplus/types"
	wasmvm "github.com/line/wasmvm"
	wasmvmtypes "github.com/line/wasmvm/types"
)

// This section was added by dynamic link and differs from the original
type cosmwasmAPIImpl struct {
	keeper *Keeper
	ctx    *sdk.Context
}

// InactiveContract

const (
	// DefaultGasCostHumanAddress is how moch SDK gas we charge to convert to a human address format
	DefaultGasCostHumanAddress = 5
	// DefaultGasCostCanonicalAddress is how moch SDK gas we charge to convert to a canonical address format
	DefaultGasCostCanonicalAddress = 4

	// DefaultDeserializationCostPerByte The formular should be `len(data) * deserializationCostPerByte`
	DefaultDeserializationCostPerByte = 1
)

var (
	costHumanize            = DefaultGasCostHumanAddress * wasmkeeper.DefaultGasMultiplier
	costCanonical           = DefaultGasCostCanonicalAddress * wasmkeeper.DefaultGasMultiplier
	costJSONDeserialization = wasmvmtypes.UFraction{
		Numerator:   DefaultDeserializationCostPerByte * wasmkeeper.DefaultGasMultiplier,
		Denominator: 1,
	}
)

func humanAddress(canon []byte) (string, uint64, error) {
	if err := sdk.VerifyAddressFormat(canon); err != nil {
		return "", costHumanize, err
	}
	return sdk.AccAddress(canon).String(), costHumanize, nil
}

func canonicalAddress(human string) ([]byte, uint64, error) {
	bz, err := sdk.AccAddressFromBech32(human)
	return bz, costCanonical, err
}

// returns result, gas used, error
func (a cosmwasmAPIImpl) callCallablePoint(contractAddrStr string, name []byte, args []byte, isReadonly bool, callstack []byte, gasLimit uint64) ([]byte, uint64, error) {
	contractAddr := sdk.MustAccAddressFromBech32(contractAddrStr)
	contractInfo, codeInfo, prefixStore, err := a.keeper.ContractInstance(*a.ctx, contractAddr)
	if err != nil {
		return nil, 0, err
	}

	if a.keeper.IsInactiveContract(*a.ctx, contractAddr) {
		return nil, 0, fmt.Errorf("called contract cannot be executed")
	}

	env := types.NewEnv(*a.ctx, contractAddr)
	wasmStore := wasmplustypes.NewWasmStore(prefixStore)
	
	gasRegister := a.keeper.GasRegister
	querier := wasmkeeper.NewQueryHandler(*a.ctx, a.keeper.WasmVMQueryHandler, contractAddr, gasRegister)
	gasMeter := a.keeper.GasMeter(*a.ctx)
	api := a.keeper.CosmwasmAPI(*a.ctx)

	instantiateCost := gasRegister.ToWasmVMGas(gasRegister.InstantiateContractCosts(a.keeper.IsPinnedCode(*a.ctx, contractInfo.CodeID), len(args)))
	if gasLimit < instantiateCost {
		return nil, 0, fmt.Errorf("Lack of gas for calling callable point")
	}
	wasmGasLimit := gasLimit - instantiateCost

	result, events, attrs, gas, err := a.keeper.WasmVM.CallCallablePoint(name, codeInfo.CodeHash, isReadonly, callstack, env, args, wasmStore, api, querier, gasMeter, wasmGasLimit, costJSONDeserialization)
	gas = gas + instantiateCost

	if err != nil {
		return nil, gas, err
	}

	if !isReadonly {
		// issue events and attrs
		if len(attrs) != 0 {
			eventsByAttr, err := newCallablePointEvent(attrs, contractAddr, callstack)
			if err != nil {
				return nil, gas, err
			}
			a.ctx.EventManager().EmitEvents(eventsByAttr)
		}

		if len(events) != 0 {
			customEvents, err := newCustomCallablePointEvents(events, contractAddr, callstack)
			if err != nil {
				return nil, gas, err
			}
			a.ctx.EventManager().EmitEvents(customEvents)
		}
	}

	return result, gas, err
}

// returns result, gas used, error
func (a cosmwasmAPIImpl) validateInterface(contractAddrStr string, expectedInterface []byte) ([]byte, uint64, error) {
	contractAddr := sdk.MustAccAddressFromBech32(contractAddrStr)
	_, codeInfo, _, err := a.keeper.ContractInstance(*a.ctx, contractAddr)
	if err != nil {
		return nil, 0, err
	}

	if a.keeper.IsInactiveContract(*a.ctx, contractAddr) {
		return nil, 0, fmt.Errorf("try to validate a contract cannot be executed")
	}


	result, err := a.keeper.WasmVM.ValidateDynamicLinkInterface(codeInfo.CodeHash, expectedInterface)

	return result, 0, err
}


// This section was added by dynamic link and differs from the original
func (k *Keeper) CosmwasmAPI(ctx sdk.Context) wasmvm.GoAPI {
	x := cosmwasmAPIImpl{
		keeper: k,
		ctx:    &ctx,
	}
	return wasmvm.GoAPI{
		HumanAddress:     humanAddress,
		CanonicalAddress: canonicalAddress,
		CallCallablePoint: x.callCallablePoint,
		ValidateInterface: x.validateInterface,
	}
}
