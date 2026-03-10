#!/usr/bin/env python3
import json
import requests
import time
import os
from dotenv import load_dotenv

# 加载 .env 文件
load_dotenv()

API_KEY = os.getenv("API_KEY")
URL = f"https://api.zan.top/node/v1/eth/mainnet/{API_KEY}"

def call_rpc(method, params):
    """调用 JSON-RPC 接口"""
    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": 1
    }

    try:
        response = requests.post(URL, json=payload, timeout=30)
        response.raise_for_status()
        data = response.json()

        if "error" in data:
            return None, f"RPC error: {data['error']['message']}"

        return data.get("result"), None
    except requests.RequestException as e:
        return None, f"Request error: {e}"

def get_latest_block_number():
    """获取最新区块高度"""
    result, error = call_rpc("eth_blockNumber", [])
    if error:
        print(f"获取最新区块高度失败: {error}")
        return 0

    print(f"原始响应: {result}")

    # 解析十六进制区块号
    hex_str = result.lstrip("0x")
    return int(hex_str, 16)

def check_state_exists(block_number):
    """检查指定高度的状态是否可查询"""
    hex_block_num = f"0x{block_number:x}"

    # 使用 eth_getStorageAt 检查状态是否存在
    # 查询零地址的存储位置
    address = "0x0000000000000000000000000000000000000000"
    slot = "0x0"

    result, error = call_rpc("eth_getStorageAt", [address, slot, hex_block_num])
    if error:
        # 查询出错，说明状态不可查询
        return False

    # 如果能返回结果，说明状态可查询
    return result is not None

def main():
    print("正在查询节点信息...")

    # 1. 获取最新区块高度
    latest_block = get_latest_block_number()
    if latest_block == 0:
        return

    print(f"最新区块高度: {latest_block} (0x{latest_block:x})")

    # 2. 使用二分法查找最早可查询的状态区块高度
    low = 0
    high = latest_block
    earliest_state_block = 0

    print("\n开始二分查找最早可查询的状态区块高度...")

    while low <= high:
        mid = low + (high - low) // 2

        state_exists = check_state_exists(mid)

        if state_exists:
            # 该高度状态可查询，记录下来并尝试更低的高度
            earliest_state_block = mid
            print(f"区块 {mid} (0x{mid:x}) 状态可查询，尝试更低高度")
            if mid == 0:
                break
            high = mid - 1
        else:
            # 该高度状态不可查询，搜索更高区间
            print(f"区块 {mid} (0x{mid:x}) 状态不可查询，搜索更高区间")
            low = mid + 1

        # 添加小延迟避免请求过快
        time.sleep(0.1)

    print(f"\n=== 结果 ===")
    print(f"最早可查询的状态区块高度: {earliest_state_block} (0x{earliest_state_block:x})")

    # 获取该区块的详细信息
    hex_str = f"0x{earliest_state_block:x}"
    result, error = call_rpc("eth_getBlockByNumber", [hex_str, True])
    if not error and result:
        if "timestamp" in result:
            print(f"该区块的时间戳: {result['timestamp']}")
        if "hash" in result:
            print(f"该区块的哈希: {result['hash']}")

if __name__ == "__main__":
    main()