package itests

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/manifest"

	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/filecoin-project/lotus/itests/kit"
)

// convert a simple byte array into input data which is a left padded 32 byte array
func inputDataFromArray(input []byte) []byte {
	inputData := make([]byte, 32)
	copy(inputData[32-len(input):], input[:])
	return inputData
}

// convert a "from" address into input data which is a left padded 32 byte array
func inputDataFromFrom(ctx context.Context, t *testing.T, client *kit.TestFullNode, from address.Address) []byte {
	fromId, err := client.StateLookupID(ctx, from, types.EmptyTSK)
	require.NoError(t, err)

	senderEthAddr, err := ethtypes.EthAddressFromFilecoinAddress(fromId)
	require.NoError(t, err)
	inputData := make([]byte, 32)
	copy(inputData[32-len(senderEthAddr):], senderEthAddr[:])
	return inputData
}

func setupFEVMTest(t *testing.T) (context.Context, context.CancelFunc, *kit.TestFullNode) {
	kit.QuietMiningLogs()
	blockTime := 5 * time.Millisecond
	client, _, ens := kit.EnsembleMinimal(t, kit.MockProofs(), kit.ThroughRPC())
	ens.InterconnectAll().BeginMiningMustPost(blockTime)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	return ctx, cancel, client
}

// TestFEVMBasic does a basic fevm contract installation and invocation
func TestFEVMBasic(t *testing.T) {

	ctx, cancel, client := setupFEVMTest(t)
	defer cancel()

	filename := "contracts/SimpleCoin.hex"
	// install contract
	fromAddr, idAddr := client.EVM().DeployContractFromFilename(ctx, filename)

	// invoke the contract with owner
	{
		inputData := inputDataFromFrom(ctx, t, client, fromAddr)
		result := client.EVM().InvokeContractByFuncName(ctx, fromAddr, idAddr, "getBalance(address)", inputData)

		expectedResult, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000002710")
		require.NoError(t, err)
		require.Equal(t, result, expectedResult)
	}

	// invoke the contract with non owner
	{
		inputData := inputDataFromFrom(ctx, t, client, fromAddr)
		inputData[31]++ // change the pub address to one that has 0 balance by incrementing the last byte of the address
		result := client.EVM().InvokeContractByFuncName(ctx, fromAddr, idAddr, "getBalance(address)", inputData)

		expectedResult, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000000")
		require.NoError(t, err)
		require.Equal(t, result, expectedResult)
	}
}

// TestFEVMETH0 tests that the ETH0 actor is in genesis
func TestFEVMETH0(t *testing.T) {
	ctx, cancel, client := setupFEVMTest(t)
	defer cancel()

	eth0id, err := address.NewIDAddress(1001)
	require.NoError(t, err)

	client.AssertActorType(ctx, eth0id, manifest.EthAccountKey)

	act, err := client.StateGetActor(ctx, eth0id, types.EmptyTSK)
	require.NoError(t, err)

	eth0Addr, err := address.NewDelegatedAddress(builtintypes.EthereumAddressManagerActorID, make([]byte, 20))
	require.NoError(t, err)
	require.Equal(t, *act.Address, eth0Addr)
}

// TestFEVMDelegateCall deploys two contracts and makes a delegate call transaction
func TestFEVMDelegateCall(t *testing.T) {

	ctx, cancel, client := setupFEVMTest(t)
	defer cancel()

	//install contract Actor
	filenameActor := "contracts/DelegatecallActor.hex"
	fromAddr, actorAddr := client.EVM().DeployContractFromFilename(ctx, filenameActor)
	//install contract Storage
	filenameStorage := "contracts/DelegatecallStorage.hex"
	fromAddrStorage, storageAddr := client.EVM().DeployContractFromFilename(ctx, filenameStorage)
	require.Equal(t, fromAddr, fromAddrStorage)

	//call Contract Storage which makes a delegatecall to contract Actor
	//this contract call sets the "counter" variable to 7, from default value 0

	inputDataContract := inputDataFromFrom(ctx, t, client, actorAddr)
	inputDataValue := inputDataFromArray([]byte{7})
	inputData := append(inputDataContract, inputDataValue...)

	//verify that the returned value of the call to setvars is 7
	result := client.EVM().InvokeContractByFuncName(ctx, fromAddr, storageAddr, "setVars(address,uint256)", inputData)
	expectedResult, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000007")
	require.NoError(t, err)
	require.Equal(t, result, expectedResult)

	//test the value is 7 via calling the getter
	result = client.EVM().InvokeContractByFuncName(ctx, fromAddr, storageAddr, "getCounter()", []byte{})
	require.Equal(t, result, expectedResult)

	//test the value is 0 via calling the getter on the Actor contract
	result = client.EVM().InvokeContractByFuncName(ctx, fromAddr, actorAddr, "getCounter()", []byte{})
	expectedResultActor, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)
	require.Equal(t, result, expectedResultActor)
}

func TestEVMRpcDisable(t *testing.T) {
	client, _, _ := kit.EnsembleMinimal(t, kit.MockProofs(), kit.ThroughRPC(), kit.DisableEthRPC())

	_, err := client.EthBlockNumber(context.Background())
	require.ErrorContains(t, err, "module disabled, enable with Fevm.EnableEthRPC")
}

// TestFEVMRecursiveFuncCall deploys a contract and makes a recursive function calls
func TestFEVMRecursiveFuncCall(t *testing.T) {
	ctx, cancel, client := setupFEVMTest(t)
	defer cancel()

	//install contract Actor
	filenameActor := "contracts/StackFunc.hex"
	fromAddr, actorAddr := client.EVM().DeployContractFromFilename(ctx, filenameActor)

	testN := func(n int, ex exitcode.ExitCode) func(t *testing.T) {
		return func(t *testing.T) {
			inputData := make([]byte, 32)
			binary.BigEndian.PutUint64(inputData[24:], uint64(n))

			client.EVM().InvokeContractByFuncNameExpectExit(ctx, fromAddr, actorAddr, "exec1(uint256)", inputData, ex)
		}
	}

	t.Run("n=0", testN(0, exitcode.Ok))
	t.Run("n=1", testN(1, exitcode.Ok))
	t.Run("n=20", testN(20, exitcode.Ok))
	t.Run("n=200", testN(200, exitcode.Ok))
	t.Run("n=507", testN(507, exitcode.Ok))
	t.Run("n=508", testN(508, exitcode.ExitCode(23))) // 23 means stack overflow
}

// TestFEVMRecursiveActorCall deploys a contract and makes a recursive actor calls
func TestFEVMRecursiveActorCall(t *testing.T) {
	ctx, cancel, client := setupFEVMTest(t)
	defer cancel()

	//install contract Actor
	filenameActor := "contracts/RecCall.hex"
	fromAddr, actorAddr := client.EVM().DeployContractFromFilename(ctx, filenameActor)

	testN := func(n, r int, ex exitcode.ExitCode) func(t *testing.T) {
		return func(t *testing.T) {
			inputData := make([]byte, 32*3)
			binary.BigEndian.PutUint64(inputData[24:], uint64(n))
			binary.BigEndian.PutUint64(inputData[32+24:], uint64(n))
			binary.BigEndian.PutUint64(inputData[32+32+24:], uint64(r))

			client.EVM().InvokeContractByFuncNameExpectExit(ctx, fromAddr, actorAddr, "exec1(uint256,uint256,uint256)", inputData, ex)
		}
	}

	t.Run("n=0,r=1", testN(0, 1, exitcode.Ok))
	t.Run("n=1,r=1", testN(1, 1, exitcode.Ok))
	t.Run("n=20,r=1", testN(20, 1, exitcode.Ok))
	t.Run("n=200,r=1", testN(200, 1, exitcode.Ok))
	t.Run("n=251,r=1", testN(251, 1, exitcode.Ok))

	t.Run("n=252,r=1-fails", testN(252, 1, exitcode.ExitCode(23))) // 23 means stack overflow

	t.Run("n=0,r=10", testN(0, 10, exitcode.Ok))
	t.Run("n=1,r=10", testN(1, 10, exitcode.Ok))
	t.Run("n=20,r=10", testN(20, 10, exitcode.Ok))
	t.Run("n=200,r=10", testN(200, 10, exitcode.Ok))
	t.Run("n=251,r=10", testN(251, 10, exitcode.Ok))

	t.Run("n=252,r=10-fails", testN(252, 10, exitcode.ExitCode(23)))

	t.Run("n=0,r=32", testN(0, 32, exitcode.Ok))
	t.Run("n=1,r=32", testN(1, 32, exitcode.Ok))
	t.Run("n=20,r=32", testN(20, 32, exitcode.Ok))
	t.Run("n=200,r=32", testN(200, 32, exitcode.Ok))
	t.Run("n=251,r=32", testN(251, 32, exitcode.Ok))

	t.Run("n=0,r=254", testN(0, 254, exitcode.Ok))
	t.Run("n=251,r=170", testN(251, 170, exitcode.Ok))

	t.Run("n=0,r=255-fails", testN(0, 255, exitcode.ExitCode(33))) // 33 means transaction reverted
	t.Run("n=251,r=171-fails", testN(251, 171, exitcode.ExitCode(33)))
}

// TestFEVMRecursiveActorCallEstimate
func TestFEVMRecursiveActorCallEstimate(t *testing.T) {
	ctx, cancel, client := setupFEVMTest(t)
	defer cancel()

	//install contract Actor
	filenameActor := "contracts/ExternalRecursiveCallSimple.hex"
	_, actorAddr := client.EVM().DeployContractFromFilename(ctx, filenameActor)

	contractAddr, err := ethtypes.EthAddressFromFilecoinAddress(actorAddr)
	require.NoError(t, err)

	// create a new Ethereum account
	key, ethAddr, ethFilAddr := client.EVM().NewAccount()
	kit.SendFunds(ctx, t, client, ethFilAddr, types.FromFil(1000))

	makeParams := func(r int) []byte {
		funcSignature := "exec1(uint256)"
		entryPoint := kit.CalcFuncSignature(funcSignature)

		inputData := make([]byte, 32)
		binary.BigEndian.PutUint64(inputData[24:], uint64(r))

		params := append(entryPoint, inputData...)

		return params
	}

	testN := func(r int) func(t *testing.T) {
		return func(t *testing.T) {
			t.Logf("running with %d recursive calls", r)

			params := makeParams(r)
			gaslimit, err := client.EthEstimateGas(ctx, ethtypes.EthCall{
				From: &ethAddr,
				To:   &contractAddr,
				Data: params,
			})
			require.NoError(t, err)
			require.LessOrEqual(t, int64(gaslimit), build.BlockGasLimit)

			t.Logf("EthEstimateGas GasLimit=%d", gaslimit)

			maxPriorityFeePerGas, err := client.EthMaxPriorityFeePerGas(ctx)
			require.NoError(t, err)

			nonce, err := client.MpoolGetNonce(ctx, ethFilAddr)
			require.NoError(t, err)

			tx := &ethtypes.EthTxArgs{
				ChainID:              build.Eip155ChainId,
				To:                   &contractAddr,
				Value:                big.Zero(),
				Nonce:                int(nonce),
				MaxFeePerGas:         types.NanoFil,
				MaxPriorityFeePerGas: big.Int(maxPriorityFeePerGas),
				GasLimit:             int(gaslimit),
				Input:                params,
				V:                    big.Zero(),
				R:                    big.Zero(),
				S:                    big.Zero(),
			}

			client.EVM().SignTransaction(tx, key.PrivateKey)
			hash := client.EVM().SubmitTransaction(ctx, tx)

			smsg, err := tx.ToSignedMessage()
			require.NoError(t, err)

			_, err = client.StateWaitMsg(ctx, smsg.Cid(), 0, 0, false)
			require.NoError(t, err)

			receipt, err := client.EthGetTransactionReceipt(ctx, hash)
			require.NoError(t, err)
			require.NotNil(t, receipt)

			t.Logf("Receipt GasUsed=%d", receipt.GasUsed)
			t.Logf("Ratio %0.2f", float64(receipt.GasUsed)/float64(gaslimit))
			t.Logf("Overestimate %0.2f", ((float64(gaslimit)/float64(receipt.GasUsed))-1)*100)

			require.EqualValues(t, ethtypes.EthUint64(1), receipt.Status)
		}
	}

	t.Run("n=1", testN(1))
	t.Run("n=2", testN(2))
	t.Run("n=3", testN(3))
	t.Run("n=4", testN(4))
	t.Run("n=5", testN(5))
	t.Run("n=10", testN(10))
	t.Run("n=20", testN(20))
	t.Run("n=30", testN(30))
	t.Run("n=40", testN(40))
	t.Run("n=50", testN(50))
	t.Run("n=100", testN(100))
}
