package handler

import (
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/TEENet-io/teenet-wallet/chain"
	"github.com/TEENet-io/teenet-wallet/model"
)

// CallReadRequest is the body for POST /api/wallets/:id/call-read.
type CallReadRequest struct {
	Contract string        `json:"contract" binding:"required"`
	FuncSig  string        `json:"func_sig" binding:"required"`
	Args     []interface{} `json:"args"`
}

// CallRead performs a read-only eth_call against a contract.
// No signing, no transaction, no approval needed.
// POST /api/wallets/:id/call-read
func (h *ContractCallHandler) CallRead(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}

	chainCfg, cfgOk := model.Chains[wallet.Chain]
	if !cfgOk || chainCfg.Family != "evm" {
		jsonError(c, http.StatusBadRequest, "call-read is only supported on EVM chains")
		return
	}

	var req CallReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	contractAddr, addrErr := normalizeEVMAddress(req.Contract)
	if addrErr != nil {
		jsonError(c, http.StatusBadRequest, "contract: "+addrErr.Error())
		return
	}

	calldata, encErr := chain.EncodeCall(req.FuncSig, req.Args)
	if encErr != nil {
		jsonError(c, http.StatusBadRequest, "encode calldata: "+encErr.Error())
		return
	}

	result, callErr := chain.ETHCall(chainCfg.RPCURL, wallet.Address, contractAddr, calldata)
	if callErr != nil {
		jsonError(c, http.StatusBadGateway, "eth_call: "+callErr.Error())
		return
	}

	methodName, methodErr := extractMethodName(req.FuncSig)
	if methodErr != nil {
		jsonError(c, http.StatusBadRequest, methodErr.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"result":   "0x" + hex.EncodeToString(result),
		"contract": contractAddr,
		"method":   methodName,
	})
}
