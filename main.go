package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type JSONRPCRequest struct {
	Jsonrpc string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []any    `json:"params"`
	ID      int      `json:"id"`
}

type JSONRPCResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func callRPC(url, method string, params []any) (json.RawMessage, error) {
	reqBody := JSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// 获取最新区块高度
func getLatestBlockNumber(url string) (uint64, error) {
	result, err := callRPC(url, "eth_blockNumber", nil)
	if err != nil {
		return 0, err
	}

	fmt.Printf("原始响应: %s\n", string(result))

	// 移除 "0x" 前缀和引号并解析
	hexStr := strings.TrimPrefix(string(result), "\"0x")
	hexStr = strings.TrimSuffix(hexStr, "\"")

	blockNumber, err := strconv.ParseUint(hexStr, 16, 64)
	return blockNumber, err
}

// 检查指定高度的区块是否存在
func checkBlockExists(url string, blockNumber uint64) (bool, error) {
	// 将区块号转换为 0x 格式
	hexStr := fmt.Sprintf("0x%x", blockNumber)

	result, err := callRPC(url, "eth_getBlockByNumber", []any{hexStr, false})
	if err != nil {
		// 如果返回错误，说明该高度不可查询
		return false, err
	}

	// 如果返回 null，说明该高度没有区块
	return !bytes.Equal(result, []byte("null")), nil
}

// 检查指定高度的状态是否可查询
func checkStateExists(url string, blockNumber uint64) (bool, error) {
	// 使用 eth_getStorageAt 检查某个已知地址的状态是否可查询
	// 我们检查一个创世区块就存在的合约或账户的存储
	// 以太坊创世区块中创世账户的存储位置

	hexBlockNum := fmt.Sprintf("0x%x", blockNumber)

	// 查询创世区块中预部署的账户地址的存储
	// 例如：使用一个简单的查询来测试状态是否存在
	// 这里我们查询某个位置的存储值
	address := "0x0000000000000000000000000000000000000000"
	slot := "0x0"

	// eth_getStorageAt 参数：address, position, blockNumber
	result, err := callRPC(url, "eth_getStorageAt", []any{address, slot, hexBlockNum})
	if err != nil {
		// 如果返回错误，说明该高度的状态不可查询
		return false, nil
	}

	// 如果能返回结果（即使是 0x0000...），说明状态可查询
	return !bytes.Equal(result, []byte("null")), nil
}

func main() {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Warning: 无法加载 .env 文件: %v\n", err)
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		fmt.Println("错误: API_KEY 未设置，请在 .env 文件中配置")
		return
	}

	url := "https://api.zan.top/node/v1/eth/mainnet/" + apiKey

	fmt.Println("正在查询节点信息...")

	// 1. 获取最新区块高度
	latestBlock, err := getLatestBlockNumber(url)
	if err != nil {
		fmt.Printf("获取最新区块高度失败: %v\n", err)
		return
	}
	fmt.Printf("最新区块高度: %d (0x%x)\n", latestBlock, latestBlock)

	// 2. 使用二分法查找最早可查询的状态区块高度
	low := uint64(0)
	high := latestBlock
	earliestStateBlock := uint64(0)

	fmt.Println("\n开始二分查找最早可查询的状态区块高度...")

	for low <= high {
		mid := low + (high-low)/2

		stateExists, err := checkStateExists(url, mid)
		if err != nil {
			// 查询出错，搜索更高区间
			fmt.Printf("区块 %d (0x%x) 状态查询出错，搜索更高区间\n", mid, mid)
			low = mid + 1
			continue
		}

		if stateExists {
			// 该高度状态可查询，记录下来并尝试更低的高度
			earliestStateBlock = mid
			fmt.Printf("区块 %d (0x%x) 状态可查询，尝试更低高度\n", mid, mid)
			if mid == 0 {
				break
			}
			high = mid - 1
		} else {
			// 该高度状态不可查询，搜索更高区间
			fmt.Printf("区块 %d (0x%x) 状态不可查询，搜索更高区间\n", mid, mid)
			low = mid + 1
		}
	}

	fmt.Printf("\n=== 结果 ===\n")
	fmt.Printf("最早可查询的状态区块高度: %d (0x%x)\n", earliestStateBlock, earliestStateBlock)

	// 获取该区块的详细信息
	hexStr := fmt.Sprintf("0x%x", earliestStateBlock)
	result, err := callRPC(url, "eth_getBlockByNumber", []any{hexStr, true})
	if err == nil {
		var block map[string]interface{}
		if err := json.Unmarshal(result, &block); err == nil {
			if timestamp, ok := block["timestamp"].(string); ok {
				fmt.Printf("该区块的时间戳: %s\n", timestamp)
			}
			if hash, ok := block["hash"].(string); ok {
				fmt.Printf("该区块的哈希: %s\n", hash)
			}
		}
	}
}
