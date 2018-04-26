// Copyright (C) 2017 go-nebulas authors
//
// This file is part of the go-nebulas library.
//
// the go-nebulas library is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// the go-nebulas library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with the go-nebulas library.  If not, see <http://www.gnu.org/licenses/>.
//

package nvm

import "C"

import (
	"unsafe"

	"encoding/json"

	"github.com/nebulasio/go-nebulas/core"
	"github.com/nebulasio/go-nebulas/core/state"
	"github.com/nebulasio/go-nebulas/util"
	"github.com/nebulasio/go-nebulas/util/byteutils"
	"github.com/nebulasio/go-nebulas/util/logging"
	"github.com/sirupsen/logrus"
)

// GetTxByHashFunc returns tx info by hash
//export GetTxByHashFunc
func GetTxByHashFunc(handler unsafe.Pointer, hash *C.char, gasCnt *C.size_t) *C.char {
	engine, _ := getEngineByStorageHandler(uint64(uintptr(handler)))
	if engine == nil || engine.ctx.block == nil {
		return nil
	}

	// calculate Gas.
	*gasCnt = C.size_t(1000)

	txHash, err := byteutils.FromHex(C.GoString(hash))
	if err != nil {
		return nil
	}
	txBytes, err := engine.ctx.state.GetTx(txHash)
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"handler": uint64(uintptr(handler)),
			"key":     C.GoString(hash),
			"err":     err,
		}).Debug("GetTxByHashFunc get tx failed.")
		return nil
	}
	sTx, err := toSerializableTransactionFromBytes(txBytes)
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"handler": uint64(uintptr(handler)),
			"key":     C.GoString(hash),
			"err":     err,
		}).Debug("GetTxByHashFunc get tx failed.")
		return nil
	}
	txJSON, err := json.Marshal(sTx)
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"handler": uint64(uintptr(handler)),
			"key":     C.GoString(hash),
			"err":     err,
		}).Debug("GetTxByHashFunc get tx failed.")
		return nil
	}

	return C.CString(string(txJSON))
}

// GetAccountStateFunc returns account info by address
//export GetAccountStateFunc
func GetAccountStateFunc(handler unsafe.Pointer, address *C.char, gasCnt *C.size_t) *C.char {
	engine, _ := getEngineByStorageHandler(uint64(uintptr(handler)))
	if engine == nil || engine.ctx.block == nil {
		return nil
	}

	// calculate Gas.
	*gasCnt = C.size_t(1000)

	addr, err := core.AddressParse(C.GoString(address))
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"handler": uint64(uintptr(handler)),
			"key":     C.GoString(address),
		}).Debug("GetAccountStateFunc parse address failed.")
		return nil
	}

	acc, err := engine.ctx.state.GetOrCreateUserAccount(addr.Bytes())
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"handler": uint64(uintptr(handler)),
			"address": addr,
			"err":     err,
		}).Debug("GetAccountStateFunc get account state failed.")
		return nil
	}
	state := toSerializableAccount(acc)
	json, err := json.Marshal(state)
	if err != nil {
		return nil
	}
	return C.CString(string(json))
}

// TransferFunc transfer vale to address
//export TransferFunc
func TransferFunc(handler unsafe.Pointer, to *C.char, v *C.char, gasCnt *C.size_t) int {
	engine, _ := getEngineByStorageHandler(uint64(uintptr(handler)))
	if engine == nil || engine.ctx.block == nil {
		logging.VLog().Error("Failed to get engine.")
		return TransferGetEngineErr
	}

	// calculate Gas.
	*gasCnt = C.size_t(2000)

	addr, err := core.AddressParse(C.GoString(to))
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"handler": uint64(uintptr(handler)),
			"key":     C.GoString(to),
		}).Debug("TransferFunc parse address failed.")
		return TransferAddressParseErr
	}

	toAcc, err := engine.ctx.state.GetOrCreateUserAccount(addr.Bytes())
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"handler": uint64(uintptr(handler)),
			"address": addr,
			"err":     err,
		}).Debug("GetAccountStateFunc get account state failed.")
		return TransferGetAccountErr
	}

	amount, err := util.NewUint128FromString(C.GoString(v))
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"handler": uint64(uintptr(handler)),
			"address": addr,
			"err":     err,
		}).Debug("GetAmountFunc get amount failed.")
		return TransferStringToBigIntErr
	}

	// update balance
	if amount.Cmp(util.NewUint128()) > 0 {
		err = engine.ctx.contract.SubBalance(amount)
		if err != nil {
			logging.VLog().WithFields(logrus.Fields{
				"handler": uint64(uintptr(handler)),
				"key":     C.GoString(to),
				"err":     err,
			}).Debug("TransferFunc SubBalance failed.")
			return TransferSubBalance
		}

		err = toAcc.AddBalance(amount)
		if err != nil {
			logging.VLog().WithFields(logrus.Fields{
				"account": toAcc,
				"amount":  amount,
				"address": addr,
				"err":     err,
			}).Debug("failed to add balance")
			return TransferAddBalance
		}
	}

	if engine.ctx.block.Height() >= TransferFromContractEventCompatibilityHeight {
		cAddr, err := core.AddressParseFromBytes(engine.ctx.contract.Address())
		if err != nil {
			logging.VLog().WithFields(logrus.Fields{
				"txhash":  engine.ctx.tx.Hash().String(),
				"address": engine.ctx.contract.Address(),
				"err":     err,
			}).Debug("failed to parse contract address")
			return TransferRecordEventFailed
		}

		event := &TransferFromContractEvent{
			Amount: amount.String(),
			From:   cAddr.String(),
			To:     addr.String(),
		}

		eData, err := json.Marshal(event)
		if err != nil {
			logging.VLog().WithFields(logrus.Fields{
				"from":   cAddr.String(),
				"to":     addr.String(),
				"amount": amount.String(),
				"err":    err,
			}).Debug("failed to marshal TransferFromContractEvent")
			return TransferRecordEventFailed
		}

		engine.ctx.state.RecordEvent(engine.ctx.tx.Hash(), &state.Event{Topic: core.TopicTransferFromContract, Data: string(eData)})
	}

	return TransferFuncSuccess
}

// VerifyAddressFunc verify address is valid
//export VerifyAddressFunc
func VerifyAddressFunc(handler unsafe.Pointer, address *C.char, gasCnt *C.size_t) int {
	// calculate Gas.
	*gasCnt = C.size_t(100)

	addr, err := core.AddressParse(C.GoString(address))
	if err != nil {
		return 0
	}
	return int(addr.Type())
}

// GetContractSourceFunc get contract code by address
//export GetContractSourceFunc
func GetContractSourceFunc(handler unsafe.Pointer, address *C.char, gasCnt *C.size_t) *C.char {
	// calculate Gas.
	engine, _ := getEngineByStorageHandler(uint64(uintptr(handler)))
	if engine == nil || engine.ctx.block == nil {
		logging.VLog().Error("Failed to get engine.")
		return nil
	}
	*gasCnt = C.size_t(100)
	ws := engine.ctx.state
	addr, err := core.AddressParse(C.GoString(address))
	if err != nil {
		return nil
	}
	contract, err := core.CheckContract(addr, ws)
	if err != nil {
		return nil
	}

	birthTx, err := core.GetTransaction(contract.BirthPlace(), ws)
	if err != nil {
		return nil
	}
	deploy, err := core.LoadDeployPayload(birthTx.Data()) // ToConfirm: move deploy payload in ctx.
	if err != nil {
		return nil
	}
	return C.CString(string(deploy.Source))
}

// RunMultilevelContractSourceFunc verify address is valid
//export RunMultilevelContractSourceFunc
func RunMultilevelContractSourceFunc(handler unsafe.Pointer, address *C.char, funcName *C.char, v *C.char, args *C.char, gasCnt *C.size_t) *C.char {
	// calculate Gas.
	logging.CLog().Errorf("begin RunMultilevelContractSourceFunc\n")
	engine, _ := getEngineByStorageHandler(uint64(uintptr(handler)))
	if engine == nil || engine.ctx.block == nil {
		logging.VLog().Error("Failed to get engine.")
		return nil
	}
	*gasCnt = C.size_t(100)
	ws := engine.ctx.state
	logging.CLog().Errorf("address:", C.GoString(address))
	addr, err := core.AddressParse(C.GoString(address))
	if err != nil {
		return nil
	}
	contract, err := core.CheckContract(addr, ws)
	if err != nil {
		return nil
	}

	birthTx, err := core.GetTransaction(contract.BirthPlace(), ws)
	if err != nil {
		return nil
	}
	deploy, err := core.LoadDeployPayload(birthTx.Data()) // ToConfirm: move deploy payload in ctx.
	if err != nil {
		return nil
	}
	//transfer
	//var transferCoseGas *C.size_t
	//TransferFunc(handler, address, v, transferCoseGas)
	logging.CLog().Errorf("begin pack payload,funcName:%s, args:%s\n", C.GoString(funcName), C.GoString(args))
	//run
	payloadType := core.TxPayloadCallType
	callpayload, err := core.NewCallPayload(C.GoString(funcName), C.GoString(args))
	if err != nil {
		logging.CLog().Errorf("core.NewCallPayload err:", err)
		return nil
	}
	payload, err := callpayload.ToBytes()
	if err != nil {
		logging.CLog().Errorf("callpayload.ToBytes err:", err)
		return nil
	}
	oldTx := engine.ctx.tx
	zeroVal := util.NewUint128()
	from := engine.ctx.contract.Address()
	fromAddr, err := core.AddressParseFromBytes(from)
	if err != nil {
		logging.CLog().Errorf("core.AddressParse err:", err)
		return nil
	}

	logging.CLog().Errorf("create NewTransaction id:%v", oldTx.ChainID())
	newTx, err := core.NewTransaction(oldTx.ChainID(), fromAddr, addr, zeroVal, oldTx.Nonce(), payloadType,
		payload, oldTx.GasPrice(), oldTx.GasLimit())
	if err != nil {
		return nil
	}
	logging.CLog().Errorf("create NewContext")

	/*addrT, err := newTx.GenerateContractAddress()
	if err != nil {
		logging.CLog().Errorf("newTx.GenerateContractAddress err:%v", err)
		return nil
	}
	contractT, err := ws.CreateContractAccount(addrT.Bytes(), newTx.Hash())
	if err != nil {
		logging.CLog().Errorf("ws.CreateContractAccount err:%v", err)
		return nil
	}*/

	newCtx, err := NewContext(engine.ctx.block, newTx, contract, engine.ctx.state)
	if err != nil {
		logging.CLog().Errorf("NewContext err:%v", err)
		return nil
	}
	logging.CLog().Errorf("begin create New V8")
	engineNew := NewV8Engine(newCtx)
	engineNew.SetExecutionLimits(10000, 10000000)
	logging.CLog().Errorf("begin Call,source:%v, sourceType:%v", deploy.Source, deploy.SourceType)
	val, err := engineNew.Call(string(deploy.Source), deploy.SourceType, C.GoString(funcName), C.GoString(args))
	engineNew.Dispose()
	if err != nil {
		return nil
	}
	logging.CLog().Errorf("end cal val:%v", val)
	return C.CString(string(val))
	//return C.CString("")
}
