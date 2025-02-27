package ethtypes

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mathbig "math/big"
	"strconv"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/multiformats/go-varint"
	"golang.org/x/crypto/sha3"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"

	"github.com/filecoin-project/lotus/build"
)

var (
	EthTopic1 = "topic1"
	EthTopic2 = "topic2"
	EthTopic3 = "topic3"
	EthTopic4 = "topic4"
)

var ErrInvalidAddress = errors.New("invalid Filecoin Eth address")

type EthUint64 uint64

func (e EthUint64) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.Hex())
}

func (e *EthUint64) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsedInt, err := strconv.ParseUint(strings.Replace(s, "0x", "", -1), 16, 64)
	if err != nil {
		return err
	}
	eint := EthUint64(parsedInt)
	*e = eint
	return nil
}

func EthUint64FromHex(s string) (EthUint64, error) {
	parsedInt, err := strconv.ParseUint(strings.Replace(s, "0x", "", -1), 16, 64)
	if err != nil {
		return EthUint64(0), err
	}
	return EthUint64(parsedInt), nil
}

func (e EthUint64) Hex() string {
	if e == 0 {
		return "0x0"
	}
	return fmt.Sprintf("0x%x", e)
}

// EthBigInt represents a large integer whose zero value serializes to "0x0".
type EthBigInt big.Int

var EthBigIntZero = EthBigInt{Int: big.Zero().Int}

func (e EthBigInt) MarshalJSON() ([]byte, error) {
	if e.Int == nil || e.Int.BitLen() == 0 {
		return json.Marshal("0x0")
	}
	return json.Marshal(fmt.Sprintf("0x%x", e.Int))
}

func (e *EthBigInt) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	replaced := strings.Replace(s, "0x", "", -1)
	if len(replaced)%2 == 1 {
		replaced = "0" + replaced
	}

	i := new(mathbig.Int)
	i.SetString(replaced, 16)

	*e = EthBigInt(big.NewFromGo(i))
	return nil
}

// EthBytes represent arbitrary bytes. A nil or empty slice serializes to "0x".
type EthBytes []byte

func (e EthBytes) MarshalJSON() ([]byte, error) {
	if len(e) == 0 {
		return json.Marshal("0x")
	}
	s := hex.EncodeToString(e)
	return json.Marshal("0x" + s)
}

func (e *EthBytes) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	s = strings.Replace(s, "0x", "", -1)
	if len(s)%2 == 1 {
		s = "0" + s
	}

	decoded, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	*e = decoded
	return nil
}

type EthBlock struct {
	Hash             EthHash    `json:"hash"`
	ParentHash       EthHash    `json:"parentHash"`
	Sha3Uncles       EthHash    `json:"sha3Uncles"`
	Miner            EthAddress `json:"miner"`
	StateRoot        EthHash    `json:"stateRoot"`
	TransactionsRoot EthHash    `json:"transactionsRoot"`
	ReceiptsRoot     EthHash    `json:"receiptsRoot"`
	LogsBloom        EthBytes   `json:"logsBloom"`
	Difficulty       EthUint64  `json:"difficulty"`
	TotalDifficulty  EthUint64  `json:"totalDifficulty"`
	Number           EthUint64  `json:"number"`
	GasLimit         EthUint64  `json:"gasLimit"`
	GasUsed          EthUint64  `json:"gasUsed"`
	Timestamp        EthUint64  `json:"timestamp"`
	Extradata        []byte     `json:"extraData"`
	MixHash          EthHash    `json:"mixHash"`
	Nonce            EthNonce   `json:"nonce"`
	BaseFeePerGas    EthBigInt  `json:"baseFeePerGas"`
	Size             EthUint64  `json:"size"`
	// can be []EthTx or []string depending on query params
	Transactions []interface{} `json:"transactions"`
	Uncles       []EthHash     `json:"uncles"`
}

var (
	EmptyEthBloom = [256]byte{}
	EmptyEthHash  = EthHash{}
	EmptyEthInt   = EthUint64(0)
	EmptyEthNonce = [8]byte{0, 0, 0, 0, 0, 0, 0, 0}
)

func NewEthBlock() EthBlock {
	return EthBlock{
		Sha3Uncles:       EmptyEthHash,
		StateRoot:        EmptyEthHash,
		TransactionsRoot: EmptyEthHash,
		ReceiptsRoot:     EmptyEthHash,
		Difficulty:       EmptyEthInt,
		LogsBloom:        EmptyEthBloom[:],
		Extradata:        []byte{},
		MixHash:          EmptyEthHash,
		Nonce:            EmptyEthNonce,
		GasLimit:         EthUint64(build.BlockGasLimit), // TODO we map Ethereum blocks to Filecoin tipsets; this is inconsistent.
		Uncles:           []EthHash{},
		Transactions:     []interface{}{},
	}
}

type EthCall struct {
	From     *EthAddress `json:"from"`
	To       *EthAddress `json:"to"`
	Gas      EthUint64   `json:"gas"`
	GasPrice EthBigInt   `json:"gasPrice"`
	Value    EthBigInt   `json:"value"`
	Data     EthBytes    `json:"data"`
}

func (c *EthCall) UnmarshalJSON(b []byte) error {
	type TempEthCall EthCall
	var params TempEthCall

	if err := json.Unmarshal(b, &params); err != nil {
		return err
	}
	*c = EthCall(params)
	return nil
}

const (
	EthAddressLength = 20
	EthHashLength    = 32
)

type EthNonce [8]byte

func (n EthNonce) String() string {
	return "0x" + hex.EncodeToString(n[:])
}

func (n EthNonce) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.String())
}

func (n *EthNonce) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	s = strings.Replace(s, "0x", "", -1)
	if len(s)%2 == 1 {
		s = "0" + s
	}

	decoded, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	copy(n[:], decoded[:8])
	return nil
}

type EthAddress [EthAddressLength]byte

// EthAddressFromPubKey returns the Ethereum address corresponding to an
// uncompressed secp256k1 public key.
func EthAddressFromPubKey(pubk []byte) ([]byte, error) {
	// if we get an uncompressed public key (that's what we get from the library,
	// but putting this check here for defensiveness), strip the prefix
	const pubKeyLen = 65
	if len(pubk) != pubKeyLen {
		return nil, fmt.Errorf("public key should have %d in length, but got %d", pubKeyLen, len(pubk))
	}
	if pubk[0] != 0x04 {
		return nil, fmt.Errorf("expected first byte of secp256k1 to be 0x04 (uncompressed)")
	}
	pubk = pubk[1:]

	// Calculate the Ethereum address based on the keccak hash of the pubkey.
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(pubk)
	ethAddr := hasher.Sum(nil)[12:]
	return ethAddr, nil
}

func EthAddressFromFilecoinAddress(addr address.Address) (EthAddress, error) {
	switch addr.Protocol() {
	case address.ID:
		id, err := address.IDFromAddress(addr)
		if err != nil {
			return EthAddress{}, err
		}
		var ethaddr EthAddress
		ethaddr[0] = 0xff
		binary.BigEndian.PutUint64(ethaddr[12:], id)
		return ethaddr, nil
	case address.Delegated:
		payload := addr.Payload()
		namespace, n, err := varint.FromUvarint(payload)
		if err != nil {
			return EthAddress{}, xerrors.Errorf("invalid delegated address namespace in: %s", addr)
		}
		payload = payload[n:]
		if namespace == builtintypes.EthereumAddressManagerActorID {
			return CastEthAddress(payload)
		}
	}
	return EthAddress{}, ErrInvalidAddress
}

// ParseEthAddress parses an Ethereum address from a hex string.
func ParseEthAddress(s string) (EthAddress, error) {
	b, err := decodeHexString(s, EthAddressLength)
	if err != nil {
		return EthAddress{}, err
	}
	var h EthAddress
	copy(h[EthAddressLength-len(b):], b)
	return h, nil
}

// CastEthAddress interprets bytes as an EthAddress, performing some basic checks.
func CastEthAddress(b []byte) (EthAddress, error) {
	var a EthAddress
	if len(b) != EthAddressLength {
		return EthAddress{}, xerrors.Errorf("cannot parse bytes into an EthAddress: incorrect input length")
	}
	copy(a[:], b[:])
	return a, nil
}

func (ea EthAddress) String() string {
	return "0x" + hex.EncodeToString(ea[:])
}

func (ea EthAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(ea.String())
}

func (ea *EthAddress) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	addr, err := ParseEthAddress(s)
	if err != nil {
		return err
	}
	copy(ea[:], addr[:])
	return nil
}

func (ea EthAddress) IsMaskedID() bool {
	idmask := [12]byte{0xff}
	return bytes.Equal(ea[:12], idmask[:])
}

func (ea EthAddress) ToFilecoinAddress() (address.Address, error) {
	if ea.IsMaskedID() {
		// This is a masked ID address.
		id := binary.BigEndian.Uint64(ea[12:])
		return address.NewIDAddress(id)
	}

	// Otherwise, translate the address into an address controlled by the
	// Ethereum Address Manager.
	addr, err := address.NewDelegatedAddress(builtintypes.EthereumAddressManagerActorID, ea[:])
	if err != nil {
		return address.Undef, fmt.Errorf("failed to translate supplied address (%s) into a "+
			"Filecoin f4 address: %w", hex.EncodeToString(ea[:]), err)
	}
	return addr, nil
}

type EthHash [EthHashLength]byte

func (h EthHash) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.String())
}

func (h *EthHash) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	hash, err := ParseEthHash(s)
	if err != nil {
		return err
	}
	copy(h[:], hash[:])
	return nil
}

func decodeHexString(s string, expectedLen int) ([]byte, error) {
	s = handleHexStringPrefix(s)
	if len(s) != expectedLen*2 {
		return nil, xerrors.Errorf("expected hex string length sans prefix %d, got %d", expectedLen*2, len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, xerrors.Errorf("cannot parse hex value: %w", err)
	}
	return b, nil
}

func DecodeHexString(s string) ([]byte, error) {
	s = handleHexStringPrefix(s)
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, xerrors.Errorf("cannot parse hex value: %w", err)
	}
	return b, nil
}

func handleHexStringPrefix(s string) string {
	// Strip the leading 0x or 0X prefix since hex.DecodeString does not support it.
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
	}
	// Sometimes clients will omit a leading zero in a byte; pad so we can decode correctly.
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return s
}

func EthHashFromCid(c cid.Cid) (EthHash, error) {
	return ParseEthHash(c.Hash().HexString()[8:])
}

func ParseEthHash(s string) (EthHash, error) {
	b, err := decodeHexString(s, EthHashLength)
	if err != nil {
		return EthHash{}, err
	}
	var h EthHash
	copy(h[EthHashLength-len(b):], b)
	return h, nil
}

func EthHashFromTxBytes(b []byte) EthHash {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(b)
	hash := hasher.Sum(nil)

	var ethHash EthHash
	copy(ethHash[:], hash)
	return ethHash
}

func (h EthHash) String() string {
	return "0x" + hex.EncodeToString(h[:])
}

// Should ONLY be used for blocks and Filecoin messages. Eth transactions expect a different hashing scheme.
func (h EthHash) ToCid() cid.Cid {
	// err is always nil
	mh, _ := multihash.EncodeName(h[:], "blake2b-256")

	return cid.NewCidV1(cid.DagCBOR, mh)
}

type EthFeeHistory struct {
	OldestBlock   uint64         `json:"oldestBlock"`
	BaseFeePerGas []EthBigInt    `json:"baseFeePerGas"`
	GasUsedRatio  []float64      `json:"gasUsedRatio"`
	Reward        *[][]EthBigInt `json:"reward,omitempty"`
}

type EthFilterID EthHash

// An opaque identifier generated by the Lotus node to refer to an active subscription.
type EthSubscriptionID EthHash

type EthFilterSpec struct {
	// Interpreted as an epoch or one of "latest" for last mined block, "earliest" for first,
	// "pending" for not yet committed messages.
	// Optional, default: "latest".
	FromBlock *string `json:"fromBlock,omitempty"`

	// Interpreted as an epoch or one of "latest" for last mined block, "earliest" for first,
	// "pending" for not yet committed messages.
	// Optional, default: "latest".
	ToBlock *string `json:"toBlock,omitempty"`

	// Actor address or a list of addresses from which event logs should originate.
	// Optional, default nil.
	// The JSON decoding must treat a string as equivalent to an array with one value, for example
	// "0x8888f1f195afa192cfee86069858" must be decoded as [ "0x8888f1f195afa192cfee86069858" ]
	Address EthAddressList `json:"address"`

	// List of topics to be matched.
	// Optional, default: empty list
	Topics EthTopicSpec `json:"topics"`

	// Restricts event logs returned to those emitted from messages contained in this tipset.
	// If BlockHash is present in in the filter criteria, then neither FromBlock nor ToBlock are allowed.
	// Added in EIP-234
	BlockHash *EthHash `json:"blockHash,omitempty"`
}

// EthAddressSpec represents a list of addresses.
// The JSON decoding must treat a string as equivalent to an array with one value, for example
// "0x8888f1f195afa192cfee86069858" must be decoded as [ "0x8888f1f195afa192cfee86069858" ]
type EthAddressList []EthAddress

func (e *EthAddressList) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte{'n', 'u', 'l', 'l'}) {
		return nil
	}
	if len(b) > 0 && b[0] == '[' {
		var addrs []EthAddress
		err := json.Unmarshal(b, &addrs)
		if err != nil {
			return err
		}
		*e = addrs
		return nil
	}
	var addr EthAddress
	err := json.Unmarshal(b, &addr)
	if err != nil {
		return err
	}
	*e = []EthAddress{addr}
	return nil
}

// TopicSpec represents a specification for matching by topic. An empty spec means all topics
// will be matched. Otherwise topics are matched conjunctively in the first dimension of the
// slice and disjunctively in the second dimension. Topics are matched in order.
// An event log with topics [A, B] will be matched by the following topic specs:
// [] "all"
// [[A]] "A in first position (and anything after)"
// [nil, [B] ] "anything in first position AND B in second position (and anything after)"
// [[A], [B]] "A in first position AND B in second position (and anything after)"
// [[A, B], [A, B]] "(A OR B) in first position AND (A OR B) in second position (and anything after)"
//
// The JSON decoding must treat string values as equivalent to arrays with one value, for example
// { "A", [ "B", "C" ] } must be decoded as [ [ A ], [ B, C ] ]
type EthTopicSpec []EthHashList

// EthHashList represents a list of EthHashes.
// The JSON decoding treats string values as equivalent to arrays with one value.
type EthHashList []EthHash

func (e *EthHashList) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte{'n', 'u', 'l', 'l'}) {
		return nil
	}
	if len(b) > 0 && b[0] == '[' {
		var hashes []EthHash
		err := json.Unmarshal(b, &hashes)
		if err != nil {
			return err
		}
		*e = hashes
		return nil
	}
	var hash EthHash
	err := json.Unmarshal(b, &hash)
	if err != nil {
		return err
	}
	*e = []EthHash{hash}
	return nil
}

// FilterResult represents the response from executing a filter: a list of block hashes, a list of transaction hashes
// or a list of logs
// This is a union type. Only one field will be populated.
// The JSON encoding must produce an array of the populated field.
type EthFilterResult struct {
	Results []interface{}
}

func (h EthFilterResult) MarshalJSON() ([]byte, error) {
	if h.Results != nil {
		return json.Marshal(h.Results)
	}
	return []byte{'[', ']'}, nil
}

func (h *EthFilterResult) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte{'n', 'u', 'l', 'l'}) {
		return nil
	}
	err := json.Unmarshal(b, &h.Results)
	return err
}

// EthLog represents the results of an event filter execution.
type EthLog struct {
	// Address is the address of the actor that produced the event log.
	Address EthAddress `json:"address"`

	// Data is the value of the event log, excluding topics
	Data EthBytes `json:"data"`

	// List of topics associated with the event log.
	Topics []EthBytes `json:"topics"`

	// Following fields are derived from the transaction containing the log

	// Indicates whether the log was removed due to a chain reorganization.
	Removed bool `json:"removed"`

	// LogIndex is the index of the event log in the sequence of events produced by the message execution.
	// (this is the index in the events AMT on the message receipt)
	LogIndex EthUint64 `json:"logIndex"`

	// TransactionIndex is the index in the tipset of the transaction that produced the event log.
	// The index corresponds to the sequence of messages produced by ChainGetParentMessages
	TransactionIndex EthUint64 `json:"transactionIndex"`

	// TransactionHash is the hash of the RLP message that produced the event log.
	TransactionHash EthHash `json:"transactionHash"`

	// BlockHash is the hash of the tipset containing the message that produced the log.
	BlockHash EthHash `json:"blockHash"`

	// BlockNumber is the epoch of the tipset containing the message.
	BlockNumber EthUint64 `json:"blockNumber"`
}

type EthSubscriptionParams struct {
	// List of topics to be matched.
	// Optional, default: empty list
	Topics EthTopicSpec `json:"topics,omitempty"`
}

type EthSubscriptionResponse struct {
	// The persistent identifier for the subscription which can be used to unsubscribe.
	SubscriptionID EthSubscriptionID `json:"subscription"`

	// The object matching the subscription. This may be a Block (tipset), a Transaction (message) or an EthLog
	Result interface{} `json:"result"`
}

func GetContractEthAddressFromCode(sender EthAddress, salt [32]byte, initcode []byte) (EthAddress, error) {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(initcode)
	inithash := hasher.Sum(nil)

	hasher.Reset()
	hasher.Write([]byte{0xff})
	hasher.Write(sender[:])
	hasher.Write(salt[:])
	hasher.Write(inithash)

	ethAddr, err := CastEthAddress(hasher.Sum(nil)[12:])
	if err != nil {
		return [20]byte{}, err
	}

	return ethAddr, nil
}
