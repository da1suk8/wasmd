package keeper_test

import (
	_ "embed"
	"encoding/json"
	"testing"
	"time"

	wasmvm "github.com/Finschia/wasmvm"
	wasmvmtypes "github.com/Finschia/wasmvm/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Finschia/wasmd/app"
	"github.com/Finschia/wasmd/x/wasm/keeper"
	"github.com/Finschia/wasmd/x/wasm/types"
)

//go:embed testdata/reflect.wasm
var wasmContract []byte

//go:embed testdata/hackatom.wasm
var hackatomContract []byte

func TestStoreCode(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContext(false)
	_, _, sender := testdata.KeyTestPubAddr()

	specs := map[string]struct {
		addr       string
		permission *types.AccessConfig
		expEvents  []abci.Event
	}{
		"address can store a contract when permission is everybody": {
			addr:       sender.String(),
			permission: &types.AllowEverybody,
			expEvents: []abci.Event{
				{
					Type: "store_code",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("code_checksum"),
							Value: []byte("2843664c3b6c1de8bdeca672267c508aeb79bb947c87f75d8053f971d8658c89"),
							Index: false,
						}, {
							Key:   []byte("code_id"),
							Value: []byte("1"),
							Index: false,
						},
					},
				},
			},
		},
		"address can store a contract when permission is nobody": {
			addr:       sender.String(),
			permission: &types.AllowNobody,
			expEvents: []abci.Event{
				{
					Type: "store_code",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("code_checksum"),
							Value: []byte("2843664c3b6c1de8bdeca672267c508aeb79bb947c87f75d8053f971d8658c89"),
							Index: false,
						}, {
							Key:   []byte("code_id"),
							Value: []byte("1"),
							Index: false,
						},
					},
				},
			},
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			xCtx, _ := ctx.CacheContext()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = wasmContract
				m.Sender = sender.String()
			})

			expHash := sha256.Sum256(wasmContract)
			// when
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(xCtx, msg)
			// check event
			assert.Equal(t, spec.expEvents, rsp.Events)

			// then
			require.NoError(t, err)
			var result types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))
			assert.Equal(t, uint64(1), result.CodeID)
			assert.Equal(t, expHash[:], result.Checksum)
			// and
			info := wasmApp.WasmKeeper.GetCodeInfo(xCtx, 1)
			assert.NotNil(t, info)
			assert.Equal(t, expHash[:], info.CodeHash)
			assert.Equal(t, sender.String(), info.Creator)
			assert.Equal(t, types.DefaultParams().InstantiateDefaultPermission.With(sender), info.InstantiateConfig)
		})
	}
}

func TestInstantiateContract(t *testing.T) {
	wasmApp := app.Setup(false)
	ctx := wasmApp.BaseApp.NewContext(false, tmproto.Header{Time: time.Now()})

	var myAddress sdk.AccAddress = make([]byte, types.ContractAddrLen)

	specs := map[string]struct {
		addr       string
		permission *types.AccessConfig
		expEvents  []abci.Event
		expErr     bool
	}{
		"address can instantiate a contract when permission is everybody": {
			addr:       myAddress.String(),
			permission: &types.AllowEverybody,
			expEvents: []abci.Event{
				{
					Type: "instantiate",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("_contract_address"),
							Value: []byte("link14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sgf2vn8"),
							Index: false,
						}, {
							Key:   []byte("code_id"),
							Value: []byte("1"),
							Index: false,
						},
					},
				},
			},
			expErr: false,
		},
		"address cannot instantiate a contract when permission is nobody": {
			addr:       myAddress.String(),
			permission: &types.AllowNobody,
			expErr:     true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			xCtx, _ := ctx.CacheContext()
			// setup
			_, _, sender := testdata.KeyTestPubAddr()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = wasmContract
				m.Sender = sender.String()
				m.InstantiatePermission = spec.permission
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(xCtx, msg)
			require.NoError(t, err)
			var result types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))

			// when
			msgInstantiate := &types.MsgInstantiateContract{
				Sender: spec.addr,
				Admin:  myAddress.String(),
				CodeID: result.CodeID,
				Label:  "test",
				Msg:    []byte(`{}`),
				Funds:  sdk.Coins{},
			}
			rsp, err = wasmApp.MsgServiceRouter().Handler(msgInstantiate)(xCtx, msgInstantiate)

			// then
			if spec.expErr {
				require.Error(t, err)
				return
			}

			var instantiateResponse types.MsgInstantiateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &instantiateResponse))

			// check event
			assert.Equal(t, spec.expEvents, rsp.Events)

			require.NoError(t, err)
		})
	}
}

func TestInstantiateContract2(t *testing.T) {
	wasmApp := app.Setup(false)
	ctx := wasmApp.BaseApp.NewContext(false, tmproto.Header{Time: time.Now()})

	var myAddress sdk.AccAddress = make([]byte, types.ContractAddrLen)

	specs := map[string]struct {
		addr       string
		permission *types.AccessConfig
		salt       string
		expEvents  []abci.Event
		expErr     bool
	}{
		"address can instantiate a contract when permission is everybody": {
			addr:       myAddress.String(),
			permission: &types.AllowEverybody,
			salt:       "salt1",
			expEvents: []abci.Event{
				{
					Type: "instantiate",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("_contract_address"),
							Value: []byte("link1nf6f7s337nw8xgjejz9pdnhmpl843ec33h596msgrqa2qgh4hkpsdmlq2u"),
							Index: false,
						}, {
							Key:   []byte("code_id"),
							Value: []byte("1"),
							Index: false,
						},
					},
				},
			},
			expErr: false,
		},
		"address cannot instantiate a contract when permission is nobody": {
			addr:       myAddress.String(),
			permission: &types.AllowNobody,
			salt:       "salt2",
			expErr:     true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			xCtx, _ := ctx.CacheContext()
			// setup
			_, _, sender := testdata.KeyTestPubAddr()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = wasmContract
				m.Sender = sender.String()
				m.InstantiatePermission = spec.permission
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(xCtx, msg)
			require.NoError(t, err)
			var result types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))

			// when
			msgInstantiate := &types.MsgInstantiateContract2{
				Sender: spec.addr,
				Admin:  myAddress.String(),
				CodeID: result.CodeID,
				Label:  "test",
				Msg:    []byte(`{}`),
				Funds:  sdk.Coins{},
				Salt:   []byte(spec.salt),
				FixMsg: true,
			}
			rsp, err = wasmApp.MsgServiceRouter().Handler(msgInstantiate)(xCtx, msgInstantiate)

			// then
			if spec.expErr {
				require.Error(t, err)
				return
			}

			var instantiateResponse types.MsgInstantiateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &instantiateResponse))

			// check event
			assert.Equal(t, spec.expEvents, rsp.Events)

			require.NoError(t, err)
		})
	}
}

func TestMigrateContract(t *testing.T) {
	wasmApp := app.Setup(false)
	ctx := wasmApp.BaseApp.NewContext(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		_, _, otherAddr                = testdata.KeyTestPubAddr()
	)

	specs := map[string]struct {
		addr      string
		expEvents []abci.Event
		expErr    bool
	}{
		"admin can migrate a contract": {
			addr: myAddress.String(),
			expEvents: []abci.Event{
				{
					Type: "migrate",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("code_id"),
							Value: []byte("1"),
							Index: false,
						}, {
							Key:   []byte("_contract_address"),
							Value: []byte("link14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sgf2vn8"),
							Index: false,
						},
					},
				},
			},
			expErr: false,
		},
		"other address cannot migrate a contract": {
			addr:   otherAddr.String(),
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			xCtx, _ := ctx.CacheContext()
			// setup
			_, _, sender := testdata.KeyTestPubAddr()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = hackatomContract
				m.Sender = sender.String()
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(xCtx, msg)
			require.NoError(t, err)
			var storeCodeResponse types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeCodeResponse))

			// instantiate contract
			initMsg := keeper.HackatomExampleInitMsg{
				Verifier:    sender,
				Beneficiary: myAddress,
			}
			initMsgBz, err := json.Marshal(initMsg)
			require.NoError(t, err)

			msgInstantiate := &types.MsgInstantiateContract{
				Sender: sender.String(),
				Admin:  myAddress.String(),
				CodeID: storeCodeResponse.CodeID,
				Label:  "test",
				Msg:    initMsgBz,
				Funds:  sdk.Coins{},
			}
			rsp, err = wasmApp.MsgServiceRouter().Handler(msgInstantiate)(xCtx, msgInstantiate)
			require.NoError(t, err)
			var instantiateResponse types.MsgInstantiateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &instantiateResponse))

			// when
			migMsg := struct {
				Verifier sdk.AccAddress `json:"verifier"`
			}{Verifier: myAddress}
			migMsgBz, err := json.Marshal(migMsg)
			require.NoError(t, err)
			msgMigrateContract := &types.MsgMigrateContract{
				Sender:   spec.addr,
				Msg:      migMsgBz,
				Contract: instantiateResponse.Address,
				CodeID:   storeCodeResponse.CodeID,
			}
			rsp, err = wasmApp.MsgServiceRouter().Handler(msgMigrateContract)(xCtx, msgMigrateContract)

			// then
			if spec.expErr {
				require.Error(t, err)
				return
			}

			// check event
			assert.Equal(t, spec.expEvents, rsp.Events)

			require.NoError(t, err)
		})
	}
}

func TestExecuteContract(t *testing.T) {
	wasmApp := app.Setup(false)
	ctx := wasmApp.BaseApp.NewContext(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		_, _, otherAddr                = testdata.KeyTestPubAddr()
	)

	// setup
	_, _, sender := testdata.KeyTestPubAddr()
	msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
		m.WASMByteCode = hackatomContract
		m.Sender = sender.String()
	})

	// store code
	rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
	require.NoError(t, err)
	var storeCodeResponse types.MsgStoreCodeResponse
	require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeCodeResponse))

	// instantiate contract
	initMsg := keeper.HackatomExampleInitMsg{
		Verifier:    myAddress,
		Beneficiary: otherAddr,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)
	msgInstantiate := types.MsgInstantiateContractFixture(func(m *types.MsgInstantiateContract) {
		m.Sender = sender.String()
		m.Admin = myAddress.String()
		m.CodeID = storeCodeResponse.CodeID
		m.Label = "test"
		m.Msg = initMsgBz
		m.Funds = sdk.Coins{}
	})
	rsp, err = wasmApp.MsgServiceRouter().Handler(msgInstantiate)(ctx, msgInstantiate)
	require.NoError(t, err)
	var instantiateResponse types.MsgInstantiateContractResponse
	require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &instantiateResponse))

	specs := map[string]struct {
		addr string
		// Note: Value with destination as key cannot be tested because it is a different value for each execution
		expEvents func(destination_address []byte) []abci.Event
		expErr    bool
	}{
		"address can execute a contract": {
			addr: myAddress.String(),
			expEvents: func(destination_address []byte) []abci.Event {
				return []abci.Event{
					{
						Type: "execute",
						Attributes: []abci.EventAttribute{
							{
								Key:   []byte("_contract_address"),
								Value: []byte(instantiateResponse.Address),
								Index: false,
							},
						},
					}, { // This is the event for the hackatom contract. See here for details.
						// https://github.com/Finschia/cosmwasm/blob/v1.1.9-0.7.0/contracts/hackatom/src/contract.rs#L97
						Type: "wasm",
						Attributes: []abci.EventAttribute{
							{
								Key:   []byte("_contract_address"),
								Value: []byte(instantiateResponse.Address),
								Index: false,
							}, {
								Key:   []byte("action"),
								Value: []byte("release"),
								Index: false,
							}, {
								Key:   []byte("destination"),
								Value: destination_address,
								Index: false,
							},
						},
					}, { // This is the event for the hackatom contract. See here for details.
						// https://github.com/Finschia/cosmwasm/blob/v1.1.9-0.7.0/contracts/hackatom/src/contract.rs#L97
						Type: "wasm-hackatom",
						Attributes: []abci.EventAttribute{
							{
								Key:   []byte("_contract_address"),
								Value: []byte(instantiateResponse.Address),
								Index: false,
							}, {
								Key:   []byte("action"),
								Value: []byte("release"),
								Index: false,
							},
						},
					},
				}
			},
			expErr: false,
		},
		"other address cannot execute a contract": {
			addr:   otherAddr.String(),
			expErr: true,
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			xCtx, _ := ctx.CacheContext()

			// when
			msgExecuteContract := types.MsgExecuteContractFixture(func(m *types.MsgExecuteContract) {
				m.Sender = spec.addr
				m.Msg = []byte(`{"release":{}}`)
				m.Contract = instantiateResponse.Address
				m.Funds = sdk.Coins{}
			})
			rsp, err = wasmApp.MsgServiceRouter().Handler(msgExecuteContract)(xCtx, msgExecuteContract)

			// then
			if spec.expErr {
				require.Error(t, err)
				return
			}

			// check event
			assert.Equal(t, spec.expEvents(rsp.Events[1].Attributes[2].Value), rsp.Events)

			require.NoError(t, err)
		})
	}
}

func TestUpdateAdmin(t *testing.T) {
	wasmApp := app.Setup(false)
	ctx := wasmApp.BaseApp.NewContext(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		_, _, otherAddr                = testdata.KeyTestPubAddr()
		_, _, newAdmin                 = testdata.KeyTestPubAddr()
	)

	// setup
	storeMsg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
		m.WASMByteCode = wasmContract
		m.Sender = myAddress.String()
	})
	rsp, err := wasmApp.MsgServiceRouter().Handler(storeMsg)(ctx, storeMsg)
	require.NoError(t, err)
	var storeCodeResult types.MsgStoreCodeResponse
	require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeCodeResult))
	codeID := storeCodeResult.CodeID

	initMsg := types.MsgInstantiateContractFixture(func(m *types.MsgInstantiateContract) {
		m.Sender = myAddress.String()
		m.Admin = myAddress.String()
		m.CodeID = codeID
		m.Msg = []byte(`{}`)
		m.Funds = sdk.Coins{}
	})
	rsp, err = wasmApp.MsgServiceRouter().Handler(initMsg)(ctx, initMsg)
	require.NoError(t, err)

	var instantiateContractResult types.MsgInstantiateContractResponse
	require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &instantiateContractResult))
	contractAddress := instantiateContractResult.Address

	specs := map[string]struct {
		addr      string
		expErr    bool
		expEvents []abci.Event
	}{
		"admin can update admin": {
			addr:   myAddress.String(),
			expErr: false,
			expEvents: []abci.Event{
				{
					Type: "update_contract_admin",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("_contract_address"),
							Value: []byte(contractAddress),
						},
						{
							Key:   []byte("new_admin_address"),
							Value: []byte(newAdmin.String()),
						},
					},
				},
			},
		},
		"other address cannot update admin": {
			addr:   otherAddr.String(),
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			xCtx, _ := ctx.CacheContext()

			// when
			msgUpdateAdmin := &types.MsgUpdateAdmin{
				Sender:   spec.addr,
				NewAdmin: newAdmin.String(),
				Contract: contractAddress,
			}
			rsp, err = wasmApp.MsgServiceRouter().Handler(msgUpdateAdmin)(xCtx, msgUpdateAdmin)

			// then
			if spec.expErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, spec.expEvents, rsp.Events)
		})
	}
}

func TestClearAdmin(t *testing.T) {
	wasmApp := app.Setup(false)
	ctx := wasmApp.BaseApp.NewContext(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		_, _, otherAddr                = testdata.KeyTestPubAddr()
	)

	// setup
	_, _, sender := testdata.KeyTestPubAddr()
	storeMsg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
		m.Sender = sender.String()
		m.WASMByteCode = wasmContract
	})
	rsp, err := wasmApp.MsgServiceRouter().Handler(storeMsg)(ctx, storeMsg)
	require.NoError(t, err)
	var storeCodeResult types.MsgStoreCodeResponse
	require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeCodeResult))
	codeID := storeCodeResult.CodeID

	initMsg := types.MsgInstantiateContractFixture(func(m *types.MsgInstantiateContract) {
		m.Sender = myAddress.String()
		m.Admin = myAddress.String()
		m.CodeID = codeID
		m.Label = "test"
		m.Msg = []byte(`{}`)
		m.Funds = sdk.Coins{}
	})
	rsp, err = wasmApp.MsgServiceRouter().Handler(initMsg)(ctx, initMsg)
	require.NoError(t, err)

	var instantiateContractResult types.MsgInstantiateContractResponse
	require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &instantiateContractResult))
	contractAddress := instantiateContractResult.Address

	specs := map[string]struct {
		addr      string
		expErr    bool
		expEvents []abci.Event
	}{
		"admin can clear admin": {
			addr:   myAddress.String(),
			expErr: false,
			expEvents: []abci.Event{
				{
					Type: "update_contract_admin",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("_contract_address"),
							Value: []byte(contractAddress),
							Index: false,
						},
						{
							Key:   []byte("new_admin_address"),
							Value: []byte{},
							Index: false,
						},
					},
				},
			},
		},
		"other address cannot clear admin": {
			addr:   otherAddr.String(),
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			xCtx, _ := ctx.CacheContext()
			// when
			msgClearAdmin := &types.MsgClearAdmin{
				Sender:   spec.addr,
				Contract: contractAddress,
			}
			rsp, err = wasmApp.MsgServiceRouter().Handler(msgClearAdmin)(xCtx, msgClearAdmin)

			// then
			if spec.expErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, spec.expEvents, rsp.Events)
		})
	}
}

func TestUpdateParams(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContext(false)

	var (
		myAddress              sdk.AccAddress = make([]byte, types.ContractAddrLen)
		oneAddressAccessConfig                = types.AccessTypeAnyOfAddresses.With(myAddress)
		govAuthority                          = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		src                types.MsgUpdateParams
		expUploadConfig    types.AccessConfig
		expInstantiateType types.AccessType
	}{
		"update upload permission param": {
			src: types.MsgUpdateParams{
				Authority: govAuthority,
				Params: types.Params{
					CodeUploadAccess:             types.AllowNobody,
					InstantiateDefaultPermission: types.AccessTypeEverybody,
				},
			},
			expUploadConfig:    types.AllowNobody,
			expInstantiateType: types.AccessTypeEverybody,
		},
		"update upload permission with same as current value": {
			src: types.MsgUpdateParams{
				Authority: govAuthority,
				Params: types.Params{
					CodeUploadAccess:             types.AllowEverybody,
					InstantiateDefaultPermission: types.AccessTypeEverybody,
				},
			},
			expUploadConfig:    types.AllowEverybody,
			expInstantiateType: types.AccessTypeEverybody,
		},
		"update upload permission param with address": {
			src: types.MsgUpdateParams{
				Authority: govAuthority,
				Params: types.Params{
					CodeUploadAccess:             oneAddressAccessConfig,
					InstantiateDefaultPermission: types.AccessTypeEverybody,
				},
			},
			expUploadConfig:    oneAddressAccessConfig,
			expInstantiateType: types.AccessTypeEverybody,
		},
		"update instantiate param": {
			src: types.MsgUpdateParams{
				Authority: govAuthority,
				Params: types.Params{
					CodeUploadAccess:             types.AllowEverybody,
					InstantiateDefaultPermission: types.AccessTypeNobody,
				},
			},
			expUploadConfig:    types.AllowEverybody,
			expInstantiateType: types.AccessTypeNobody,
		},
		"update instantiate param as default": {
			src: types.MsgUpdateParams{
				Authority: govAuthority,
				Params: types.Params{
					CodeUploadAccess:             types.AllowEverybody,
					InstantiateDefaultPermission: types.AccessTypeEverybody,
				},
			},
			expUploadConfig:    types.AllowEverybody,
			expInstantiateType: types.AccessTypeEverybody,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			err := wasmApp.WasmKeeper.SetParams(ctx, types.DefaultParams())
			require.NoError(t, err)

			// when
			rsp, err := wasmApp.MsgServiceRouter().Handler(&spec.src)(ctx, &spec.src) //nolint:gosec
			require.NoError(t, err)
			var result types.MsgUpdateParamsResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))

			// then
			assert.True(t, spec.expUploadConfig.Equals(wasmApp.WasmKeeper.GetParams(ctx).CodeUploadAccess),
				"got %#v not %#v", wasmApp.WasmKeeper.GetParams(ctx).CodeUploadAccess, spec.expUploadConfig)
			assert.Equal(t, spec.expInstantiateType, wasmApp.WasmKeeper.GetParams(ctx).InstantiateDefaultPermission)
		})
	}
}

func TestAddCodeUploadParamsAddresses(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContext(false)

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		_, _, otherAddr                = testdata.KeyTestPubAddr()
		govAuthority                   = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		src             types.MsgAddCodeUploadParamsAddresses
		uploadConfig    types.AccessConfig
		expUploadConfig types.AccessConfig
		expErr          bool
	}{
		"authority can add addresses when permission is any of addresses": {
			src: types.MsgAddCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{otherAddr.String()},
			},
			uploadConfig:    types.AccessTypeAnyOfAddresses.With(myAddress),
			expUploadConfig: types.AccessTypeAnyOfAddresses.With(myAddress, otherAddr),
		},
		"add existing address": {
			src: types.MsgAddCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{myAddress.String()},
			},
			uploadConfig:    types.AccessTypeAnyOfAddresses.With(myAddress),
			expUploadConfig: types.AccessTypeAnyOfAddresses.With(myAddress),
		},
		"authority cannot add addresses when permission is allow everybody": {
			src: types.MsgAddCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{otherAddr.String()},
			},
			uploadConfig:    types.AllowEverybody,
			expUploadConfig: types.AllowEverybody,
			expErr:          true,
		},
		"authority cannot add addresses when permission is allow nobody": {
			src: types.MsgAddCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{otherAddr.String()},
			},
			uploadConfig:    types.AllowNobody,
			expUploadConfig: types.AllowNobody,
			expErr:          true,
		},
		"authority cannot add duplicate addresses when permission is any of addresses": {
			src: types.MsgAddCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{otherAddr.String(), otherAddr.String()},
			},
			uploadConfig:    types.AccessTypeAnyOfAddresses.With(myAddress),
			expUploadConfig: types.AccessTypeAnyOfAddresses.With(myAddress),
			expErr:          true,
		},
		"other address cannot add addresses when permission is any of addresses": {
			src: types.MsgAddCodeUploadParamsAddresses{
				Authority: otherAddr.String(),
				Addresses: []string{otherAddr.String()},
			},
			uploadConfig:    types.AccessTypeAnyOfAddresses.With(myAddress),
			expUploadConfig: types.AccessTypeAnyOfAddresses.With(myAddress, otherAddr),
			expErr:          true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			err := wasmApp.WasmKeeper.SetParams(ctx, types.Params{
				CodeUploadAccess:             spec.uploadConfig,
				InstantiateDefaultPermission: types.AccessTypeEverybody,
			})
			require.NoError(t, err)

			// when
			rsp, err := wasmApp.MsgServiceRouter().Handler(&spec.src)(ctx, &spec.src) //nolint:gosec
			if spec.expErr {
				require.Error(t, err)
				require.Nil(t, rsp)
				return
			}
			require.NoError(t, err)
			require.Len(t, rsp.MsgResponses, 1)
			assert.IsType(t, rsp.MsgResponses[0].GetCachedValue(), &types.MsgAddCodeUploadParamsAddressesResponse{})

			// then
			gotUploadConfig := wasmApp.WasmKeeper.GetParams(ctx).CodeUploadAccess
			assert.True(t, spec.expUploadConfig.Equals(gotUploadConfig),
				"got %#v not %#v", gotUploadConfig, spec.expUploadConfig)
		})
	}
}

func TestRemoveCodeUploadParamsAddresses(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContext(false)

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		_, _, otherAddr                = testdata.KeyTestPubAddr()
		govAuthority                   = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		src             types.MsgRemoveCodeUploadParamsAddresses
		uploadConfig    types.AccessConfig
		expUploadConfig types.AccessConfig
		expErr          bool
	}{
		"authority can remove addresses when permission is any of addresses": {
			src: types.MsgRemoveCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{otherAddr.String()},
			},
			uploadConfig:    types.AccessTypeAnyOfAddresses.With(myAddress, otherAddr),
			expUploadConfig: types.AccessTypeAnyOfAddresses.With(myAddress),
		},
		"authority cannot remove not existing addresses when permission is any of addresses": {
			src: types.MsgRemoveCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{otherAddr.String()},
			},
			uploadConfig:    types.AccessTypeAnyOfAddresses.With(myAddress),
			expUploadConfig: types.AccessTypeAnyOfAddresses.With(myAddress),
		},
		"authority cannot remove addresses when permission is allow everybody": {
			src: types.MsgRemoveCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{otherAddr.String()},
			},
			uploadConfig:    types.AllowEverybody,
			expUploadConfig: types.AllowEverybody,
			expErr:          true,
		},
		"authority cannot remove addresses when permission is allow nobody": {
			src: types.MsgRemoveCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{otherAddr.String()},
			},
			uploadConfig:    types.AllowNobody,
			expUploadConfig: types.AllowNobody,
			expErr:          true,
		},
		"authority cannot remove duplicate addresses when permission is any of addresses": {
			src: types.MsgRemoveCodeUploadParamsAddresses{
				Authority: govAuthority,
				Addresses: []string{otherAddr.String(), otherAddr.String()},
			},
			uploadConfig:    types.AccessTypeAnyOfAddresses.With(myAddress, otherAddr),
			expUploadConfig: types.AccessTypeAnyOfAddresses.With(myAddress, otherAddr),
			expErr:          true,
		},
		"other address cannot remove addresses when permission is any of addresses": {
			src: types.MsgRemoveCodeUploadParamsAddresses{
				Authority: otherAddr.String(),
				Addresses: []string{otherAddr.String()},
			},
			uploadConfig:    types.AccessTypeAnyOfAddresses.With(myAddress, otherAddr),
			expUploadConfig: types.AccessTypeAnyOfAddresses.With(myAddress, otherAddr),
			expErr:          true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			err := wasmApp.WasmKeeper.SetParams(ctx, types.Params{
				CodeUploadAccess:             spec.uploadConfig,
				InstantiateDefaultPermission: types.AccessTypeEverybody,
			})
			require.NoError(t, err)

			// when
			rsp, err := wasmApp.MsgServiceRouter().Handler(&spec.src)(ctx, &spec.src) //nolint:gosec
			if spec.expErr {
				require.Error(t, err)
				require.Nil(t, rsp)
				return
			}
			require.NoError(t, err)
			require.Len(t, rsp.MsgResponses, 1)
			assert.IsType(t, rsp.MsgResponses[0].GetCachedValue(), &types.MsgRemoveCodeUploadParamsAddressesResponse{})

			// then
			gotUploadConfig := wasmApp.WasmKeeper.GetParams(ctx).CodeUploadAccess
			assert.True(t, spec.expUploadConfig.Equals(gotUploadConfig),
				"got %#v not %#v", gotUploadConfig, spec.expUploadConfig)
		})
	}
}

func TestPinCodes(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContext(false)

	var (
		myAddress sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		addr   string
		expErr bool
	}{
		"authority can pin codes": {
			addr:   authority,
			expErr: false,
		},
		"other address cannot pin codes": {
			addr:   myAddress.String(),
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			_, _, sender := testdata.KeyTestPubAddr()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = wasmContract
				m.Sender = sender.String()
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var result types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))
			require.False(t, wasmApp.WasmKeeper.IsPinnedCode(ctx, result.CodeID))

			// when
			msgPinCodes := &types.MsgPinCodes{
				Authority: spec.addr,
				CodeIDs:   []uint64{result.CodeID},
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgPinCodes)(ctx, msgPinCodes)

			// then
			if spec.expErr {
				require.Error(t, err)
				assert.False(t, wasmApp.WasmKeeper.IsPinnedCode(ctx, result.CodeID))
			} else {
				require.NoError(t, err)
				assert.True(t, wasmApp.WasmKeeper.IsPinnedCode(ctx, result.CodeID))
			}
		})
	}
}

func TestUnpinCodes(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContext(false)

	var (
		myAddress sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		addr   string
		expErr bool
	}{
		"authority can unpin codes": {
			addr:   authority,
			expErr: false,
		},
		"other address cannot unpin codes": {
			addr:   myAddress.String(),
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			_, _, sender := testdata.KeyTestPubAddr()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = wasmContract
				m.Sender = sender.String()
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var result types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))

			// pin code
			msgPin := &types.MsgPinCodes{
				Authority: authority,
				CodeIDs:   []uint64{result.CodeID},
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgPin)(ctx, msgPin)
			require.NoError(t, err)
			assert.True(t, wasmApp.WasmKeeper.IsPinnedCode(ctx, result.CodeID))

			// when
			msgUnpinCodes := &types.MsgUnpinCodes{
				Authority: spec.addr,
				CodeIDs:   []uint64{result.CodeID},
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgUnpinCodes)(ctx, msgUnpinCodes)

			// then
			if spec.expErr {
				require.Error(t, err)
				assert.True(t, wasmApp.WasmKeeper.IsPinnedCode(ctx, result.CodeID))
			} else {
				require.NoError(t, err)
				assert.False(t, wasmApp.WasmKeeper.IsPinnedCode(ctx, result.CodeID))
			}
		})
	}
}

func TestSudoContract(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                = wasmApp.WasmKeeper.GetAuthority()
	)

	type StealMsg struct {
		Recipient string     `json:"recipient"`
		Amount    []sdk.Coin `json:"amount"`
	}

	stealMsg := struct {
		Steal StealMsg `json:"steal_funds"`
	}{Steal: StealMsg{
		Recipient: myAddress.String(),
		Amount:    []sdk.Coin{},
	}}

	stealMsgBz, err := json.Marshal(stealMsg)
	require.NoError(t, err)

	specs := map[string]struct {
		addr   string
		expErr bool
	}{
		"authority can call sudo on a contract": {
			addr:   authority,
			expErr: false,
		},
		"other address cannot call sudo on a contract": {
			addr:   myAddress.String(),
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			_, _, sender := testdata.KeyTestPubAddr()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = hackatomContract
				m.Sender = sender.String()
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var storeCodeResponse types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeCodeResponse))

			// instantiate contract
			initMsg := keeper.HackatomExampleInitMsg{
				Verifier:    sender,
				Beneficiary: myAddress,
			}
			initMsgBz, err := json.Marshal(initMsg)
			require.NoError(t, err)

			msgInstantiate := &types.MsgInstantiateContract{
				Sender: sender.String(),
				Admin:  sender.String(),
				CodeID: storeCodeResponse.CodeID,
				Label:  "test",
				Msg:    initMsgBz,
				Funds:  sdk.Coins{},
			}
			rsp, err = wasmApp.MsgServiceRouter().Handler(msgInstantiate)(ctx, msgInstantiate)
			require.NoError(t, err)
			var instantiateResponse types.MsgInstantiateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &instantiateResponse))

			// when
			msgSudoContract := &types.MsgSudoContract{
				Authority: spec.addr,
				Msg:       stealMsgBz,
				Contract:  instantiateResponse.Address,
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgSudoContract)(ctx, msgSudoContract)

			// then
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStoreAndInstantiateContract(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		addr       string
		permission *types.AccessConfig
		expErr     bool
	}{
		"authority can store and instantiate a contract when permission is nobody": {
			addr:       authority,
			permission: &types.AllowNobody,
			expErr:     false,
		},
		"other address cannot store and instantiate a contract when permission is nobody": {
			addr:       myAddress.String(),
			permission: &types.AllowNobody,
			expErr:     true,
		},
		"authority can store and instantiate a contract when permission is everybody": {
			addr:       authority,
			permission: &types.AllowEverybody,
			expErr:     false,
		},
		"other address can store and instantiate a contract when permission is everybody": {
			addr:       myAddress.String(),
			permission: &types.AllowEverybody,
			expErr:     false,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// when
			msg := &types.MsgStoreAndInstantiateContract{
				Authority:             spec.addr,
				WASMByteCode:          wasmContract,
				InstantiatePermission: spec.permission,
				Admin:                 myAddress.String(),
				UnpinCode:             false,
				Label:                 "test",
				Msg:                   []byte(`{}`),
				Funds:                 sdk.Coins{},
			}
			_, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)

			// then
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdateAdmin(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                      = wasmApp.WasmKeeper.GetAuthority()
		_, _, otherAddr                = testdata.KeyTestPubAddr()
	)

	specs := map[string]struct {
		addr   string
		expErr bool
	}{
		"authority can update admin": {
			addr:   authority,
			expErr: false,
		},
		"admin can update admin": {
			addr:   myAddress.String(),
			expErr: false,
		},
		"other address cannot update admin": {
			addr:   otherAddr.String(),
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			_, _, newAdmin := testdata.KeyTestPubAddr()

			// setup
			msg := &types.MsgStoreAndInstantiateContract{
				Authority:             spec.addr,
				WASMByteCode:          wasmContract,
				InstantiatePermission: &types.AllowEverybody,
				Admin:                 myAddress.String(),
				UnpinCode:             false,
				Label:                 "test",
				Msg:                   []byte(`{}`),
				Funds:                 sdk.Coins{},
			}
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var storeAndInstantiateResponse types.MsgStoreAndInstantiateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeAndInstantiateResponse))

			// when
			msgUpdateAdmin := &types.MsgUpdateAdmin{
				Sender:   spec.addr,
				NewAdmin: newAdmin.String(),
				Contract: storeAndInstantiateResponse.Address,
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgUpdateAdmin)(ctx, msgUpdateAdmin)

			// then
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClearAdmin(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                      = wasmApp.WasmKeeper.GetAuthority()
		_, _, otherAddr                = testdata.KeyTestPubAddr()
	)

	specs := map[string]struct {
		addr   string
		expErr bool
	}{
		"authority can clear admin": {
			addr:   authority,
			expErr: false,
		},
		"admin can clear admin": {
			addr:   myAddress.String(),
			expErr: false,
		},
		"other address cannot clear admin": {
			addr:   otherAddr.String(),
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			msg := &types.MsgStoreAndInstantiateContract{
				Authority:             spec.addr,
				WASMByteCode:          wasmContract,
				InstantiatePermission: &types.AllowEverybody,
				Admin:                 myAddress.String(),
				UnpinCode:             false,
				Label:                 "test",
				Msg:                   []byte(`{}`),
				Funds:                 sdk.Coins{},
			}
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var storeAndInstantiateResponse types.MsgStoreAndInstantiateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeAndInstantiateResponse))

			// when
			msgClearAdmin := &types.MsgClearAdmin{
				Sender:   spec.addr,
				Contract: storeAndInstantiateResponse.Address,
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgClearAdmin)(ctx, msgClearAdmin)

			// then
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMigrateContract(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                      = wasmApp.WasmKeeper.GetAuthority()
		_, _, otherAddr                = testdata.KeyTestPubAddr()
	)

	specs := map[string]struct {
		addr   string
		expErr bool
	}{
		"authority can migrate a contract": {
			addr:   authority,
			expErr: false,
		},
		"admin can migrate a contract": {
			addr:   myAddress.String(),
			expErr: false,
		},
		"other address cannot migrate a contract": {
			addr:   otherAddr.String(),
			expErr: true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			_, _, sender := testdata.KeyTestPubAddr()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = hackatomContract
				m.Sender = sender.String()
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var storeCodeResponse types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeCodeResponse))

			// instantiate contract
			initMsg := keeper.HackatomExampleInitMsg{
				Verifier:    sender,
				Beneficiary: myAddress,
			}
			initMsgBz, err := json.Marshal(initMsg)
			require.NoError(t, err)

			msgInstantiate := &types.MsgInstantiateContract{
				Sender: sender.String(),
				Admin:  myAddress.String(),
				CodeID: storeCodeResponse.CodeID,
				Label:  "test",
				Msg:    initMsgBz,
				Funds:  sdk.Coins{},
			}
			rsp, err = wasmApp.MsgServiceRouter().Handler(msgInstantiate)(ctx, msgInstantiate)
			require.NoError(t, err)
			var instantiateResponse types.MsgInstantiateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &instantiateResponse))

			// when
			migMsg := struct {
				Verifier sdk.AccAddress `json:"verifier"`
			}{Verifier: myAddress}
			migMsgBz, err := json.Marshal(migMsg)
			require.NoError(t, err)
			msgMigrateContract := &types.MsgMigrateContract{
				Sender:   spec.addr,
				Msg:      migMsgBz,
				Contract: instantiateResponse.Address,
				CodeID:   storeCodeResponse.CodeID,
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgMigrateContract)(ctx, msgMigrateContract)

			// then
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInstantiateContract(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		addr       string
		permission *types.AccessConfig
		expErr     bool
	}{
		"authority can instantiate a contract when permission is nobody": {
			addr:       authority,
			permission: &types.AllowNobody,
			expErr:     false,
		},
		"other address cannot instantiate a contract when permission is nobody": {
			addr:       myAddress.String(),
			permission: &types.AllowNobody,
			expErr:     true,
		},
		"authority can instantiate a contract when permission is everybody": {
			addr:       authority,
			permission: &types.AllowEverybody,
			expErr:     false,
		},
		"other address can  instantiate a contract when permission is everybody": {
			addr:       myAddress.String(),
			permission: &types.AllowEverybody,
			expErr:     false,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			_, _, sender := testdata.KeyTestPubAddr()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = wasmContract
				m.Sender = sender.String()
				m.InstantiatePermission = spec.permission
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var result types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))

			// when
			msgInstantiate := &types.MsgInstantiateContract{
				Sender: spec.addr,
				Admin:  myAddress.String(),
				CodeID: result.CodeID,
				Label:  "test",
				Msg:    []byte(`{}`),
				Funds:  sdk.Coins{},
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgInstantiate)(ctx, msgInstantiate)

			// then
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInstantiateContract2(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		addr       string
		salt       string
		permission *types.AccessConfig
		expErr     bool
	}{
		"authority can instantiate a contract when permission is nobody": {
			addr:       authority,
			permission: &types.AllowNobody,
			salt:       "salt1",
			expErr:     false,
		},
		"other address cannot instantiate a contract when permission is nobody": {
			addr:       myAddress.String(),
			permission: &types.AllowNobody,
			salt:       "salt2",
			expErr:     true,
		},
		"authority can instantiate a contract when permission is everybody": {
			addr:       authority,
			permission: &types.AllowEverybody,
			salt:       "salt3",
			expErr:     false,
		},
		"other address can  instantiate a contract when permission is everybody": {
			addr:       myAddress.String(),
			permission: &types.AllowEverybody,
			salt:       "salt4",
			expErr:     false,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			_, _, sender := testdata.KeyTestPubAddr()
			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = wasmContract
				m.Sender = sender.String()
				m.InstantiatePermission = spec.permission
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var result types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))

			// when
			msgInstantiate := &types.MsgInstantiateContract2{
				Sender: spec.addr,
				Admin:  myAddress.String(),
				CodeID: result.CodeID,
				Label:  "label",
				Msg:    []byte(`{}`),
				Funds:  sdk.Coins{},
				Salt:   []byte(spec.salt),
				FixMsg: true,
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgInstantiate)(ctx, msgInstantiate)

			// then
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdateInstantiateConfig(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	var (
		creator   sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		addr       string
		permission *types.AccessConfig
		expErr     bool
	}{
		"authority can update instantiate config when permission is subset": {
			addr:       authority,
			permission: &types.AllowNobody,
			expErr:     false,
		},
		"creator can update instantiate config when permission is subset": {
			addr:       creator.String(),
			permission: &types.AllowNobody,
			expErr:     false,
		},
		"authority can update instantiate config when permission is not subset": {
			addr:       authority,
			permission: &types.AllowEverybody,
			expErr:     false,
		},
		"creator cannot  update instantiate config when permission is not subset": {
			addr:       creator.String(),
			permission: &types.AllowEverybody,
			expErr:     true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			err := wasmApp.WasmKeeper.SetParams(ctx, types.Params{
				CodeUploadAccess:             types.AllowEverybody,
				InstantiateDefaultPermission: types.AccessTypeNobody,
			})
			require.NoError(t, err)

			msg := types.MsgStoreCodeFixture(func(m *types.MsgStoreCode) {
				m.WASMByteCode = wasmContract
				m.Sender = creator.String()
				m.InstantiatePermission = &types.AllowNobody
			})

			// store code
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var result types.MsgStoreCodeResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))

			// when
			msgUpdateInstantiateConfig := &types.MsgUpdateInstantiateConfig{
				Sender:                   spec.addr,
				CodeID:                   result.CodeID,
				NewInstantiatePermission: spec.permission,
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgUpdateInstantiateConfig)(ctx, msgUpdateInstantiateConfig)

			// then
			if spec.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStoreAndMigrateContract(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	checksum, err := wasmvm.CreateChecksum(hackatomContract)
	require.NoError(t, err)

	var (
		myAddress sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                = wasmApp.WasmKeeper.GetAuthority()
	)

	specs := map[string]struct {
		addr        string
		permission  *types.AccessConfig
		expChecksum []byte
		expErr      bool
	}{
		"authority can store and migrate a contract when permission is nobody": {
			addr:        authority,
			permission:  &types.AllowNobody,
			expChecksum: checksum,
		},
		"authority can store and migrate a contract when permission is everybody": {
			addr:        authority,
			permission:  &types.AllowEverybody,
			expChecksum: checksum,
		},
		"other address can store and migrate a contract when permission is everybody": {
			addr:        myAddress.String(),
			permission:  &types.AllowEverybody,
			expChecksum: checksum,
		},
		"other address cannot store and migrate a contract when permission is nobody": {
			addr:       myAddress.String(),
			permission: &types.AllowNobody,
			expErr:     true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			initMsg := keeper.HackatomExampleInitMsg{
				Verifier:    myAddress,
				Beneficiary: myAddress,
			}
			initMsgBz, err := json.Marshal(initMsg)
			require.NoError(t, err)
			storeAndInstantiateMsg := &types.MsgStoreAndInstantiateContract{
				Authority:             spec.addr,
				WASMByteCode:          hackatomContract,
				InstantiatePermission: &types.AllowEverybody,
				Admin:                 myAddress.String(),
				UnpinCode:             false,
				Label:                 "test",
				Msg:                   initMsgBz,
				Funds:                 sdk.Coins{},
			}
			rsp, err := wasmApp.MsgServiceRouter().Handler(storeAndInstantiateMsg)(ctx, storeAndInstantiateMsg)
			require.NoError(t, err)
			var storeAndInstantiateResponse types.MsgStoreAndInstantiateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeAndInstantiateResponse))

			contractAddr := storeAndInstantiateResponse.Address

			// when
			migMsg := struct {
				Verifier sdk.AccAddress `json:"verifier"`
			}{Verifier: myAddress}
			migMsgBz, err := json.Marshal(migMsg)
			require.NoError(t, err)
			msg := &types.MsgStoreAndMigrateContract{
				Authority:             spec.addr,
				WASMByteCode:          hackatomContract,
				InstantiatePermission: spec.permission,
				Msg:                   migMsgBz,
				Contract:              contractAddr,
			}
			rsp, err = wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)

			// then
			if spec.expErr {
				require.Error(t, err)
				require.Nil(t, rsp)
				return
			}

			require.NoError(t, err)
			var result types.MsgStoreAndMigrateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &result))
			assert.Equal(t, spec.expChecksum, result.Checksum)
			require.NotZero(t, result.CodeID)
		})
	}
}

func TestUpdateContractLabel(t *testing.T) {
	wasmApp := app.Setup(t)
	ctx := wasmApp.BaseApp.NewContextLegacy(false, tmproto.Header{Time: time.Now()})

	var (
		myAddress       sdk.AccAddress = make([]byte, types.ContractAddrLen)
		authority                      = wasmApp.WasmKeeper.GetAuthority()
		_, _, otherAddr                = testdata.KeyTestPubAddr()
	)

	specs := map[string]struct {
		addr     string
		newLabel string
		expErr   bool
	}{
		"authority can update contract label": {
			addr:     authority,
			newLabel: "new label",
			expErr:   false,
		},
		"admin can update contract label": {
			addr:     myAddress.String(),
			newLabel: "new label",
			expErr:   false,
		},
		"other address cannot update contract label": {
			addr:     otherAddr.String(),
			newLabel: "new label",
			expErr:   true,
		},
		"empty new label": {
			addr:   authority,
			expErr: true,
		},
		"invalid new label": {
			addr:     authority,
			newLabel: " start with space ",
			expErr:   true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup
			msg := &types.MsgStoreAndInstantiateContract{
				Authority:             spec.addr,
				WASMByteCode:          wasmContract,
				InstantiatePermission: &types.AllowEverybody,
				Admin:                 myAddress.String(),
				UnpinCode:             false,
				Label:                 "old label",
				Msg:                   []byte(`{}`),
				Funds:                 sdk.Coins{},
			}
			rsp, err := wasmApp.MsgServiceRouter().Handler(msg)(ctx, msg)
			require.NoError(t, err)
			var storeAndInstantiateResponse types.MsgStoreAndInstantiateContractResponse
			require.NoError(t, wasmApp.AppCodec().Unmarshal(rsp.Data, &storeAndInstantiateResponse))

			contract := storeAndInstantiateResponse.Address
			contractAddr, err := sdk.AccAddressFromBech32(contract)
			require.NoError(t, err)
			require.Equal(t, "old label", wasmApp.WasmKeeper.GetContractInfo(ctx, contractAddr).Label)

			// when
			msgUpdateLabel := &types.MsgUpdateContractLabel{
				Sender:   spec.addr,
				NewLabel: spec.newLabel,
				Contract: storeAndInstantiateResponse.Address,
			}
			_, err = wasmApp.MsgServiceRouter().Handler(msgUpdateLabel)(ctx, msgUpdateLabel)

			// then
			if spec.expErr {
				require.Error(t, err)
				require.Equal(t, "old label", wasmApp.WasmKeeper.GetContractInfo(ctx, contractAddr).Label)
			} else {
				require.NoError(t, err)
				require.Equal(t, spec.newLabel, wasmApp.WasmKeeper.GetContractInfo(ctx, contractAddr).Label)
			}
		})
	}
}
