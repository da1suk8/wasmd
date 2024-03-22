package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	wasmtypes "github.com/Finschia/wasmd/x/wasm/types"
)

func (gs GenesisState) ValidateBasic() error {
	if err := gs.Params.ValidateBasic(); err != nil {
		return errorsmod.Wrap(err, "params")
	}
	for i := range gs.Codes {
		if err := gs.Codes[i].ValidateBasic(); err != nil {
			return errorsmod.Wrapf(err, "code: %d", i)
		}
	}
	for i := range gs.Contracts {
		if err := gs.Contracts[i].ValidateBasic(); err != nil {
			return errorsmod.Wrapf(err, "contract: %d", i)
		}
	}
	for i := range gs.Sequences {
		if err := gs.Sequences[i].ValidateBasic(); err != nil {
			return errorsmod.Wrapf(err, "sequence: %d", i)
		}
	}
	for i, addr := range gs.InactiveContractAddresses {
		if _, err := sdk.AccAddressFromBech32(addr); err != nil {
			return errorsmod.Wrapf(err, "inactive contract address: %d", i)
		}
	}
	return nil
}

// RawWasmState convert to wasm genesis state for vanilla import.
// Custom data models for privileged contracts are not included
func (gs GenesisState) RawWasmState() wasmtypes.GenesisState {
	params := wasmtypes.Params{
		CodeUploadAccess:             gs.Params.CodeUploadAccess,
		InstantiateDefaultPermission: gs.Params.InstantiateDefaultPermission,
	}
	return wasmtypes.GenesisState{
		Params:    params,
		Codes:     gs.Codes,
		Contracts: gs.Contracts,
		Sequences: gs.Sequences,
	}
}

func ValidateGenesis(data GenesisState) error {
	return data.ValidateBasic()
}
