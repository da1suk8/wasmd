package types

const (
	// WasmModuleEventType is stored with any contract TX that returns non empty EventAttributes
	WasmModuleEventType = "wasm"
	// CustomContractEventPrefix contracts can create custom events. To not mix them with other system events they got the `wasm-` prefix.
	CustomContractEventPrefix = "wasm-"
	// events from callable point
	CallablePointEventType = "wasm-callablepoint"
	// prefix for custom events from callable point
	CustomCallablePointEventPrefix = "wasm-callablepoint-"

	EventTypeStoreCode         = "store_code"
	EventTypeInstantiate       = "instantiate"
	EventTypeExecute           = "execute"
	EventTypeMigrate           = "migrate"
	EventTypeUpdateAdmin       = "update_admin"
	EventTypeClearAdmin        = "clear_admin"
	EventTypePinCode           = "pin_code"
	EventTypeUnpinCode         = "unpin_code"
	EventTypeSudo              = "sudo"
	EventTypeReply             = "reply"
	EventTypeGovContractResult = "gov_contract_result"
)

// event attributes returned from contract execution
const (
	AttributeReservedPrefix = "_"

	AttributeKeyContractAddr       = "_contract_address"
	AttributeKeyCodeID             = "code_id"
	AttributeKeyChecksum           = "code_checksum"
	AttributeKeyResultDataHex      = "result"
	AttributeKeyRequiredCapability = "required_capability"
	AttributeKeyCallstack     = "_callstack"
	AttributeKeyCodeIDs       = "code_ids"
)
