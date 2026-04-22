# 操作指南：添加链

## 检查清单

1. **在 `chains.json` 中添加条目** —— 完整字段说明请参阅 [chains.json Schema](chains-schema.md)。`chains.json` 只在启动时加载一次,修改后需要重启服务生效。

2. **在 `handler/price.go` 中添加 CoinGecko 价格源映射** —— 找到 `coinGeckoIDs` map 并添加原生代币符号。

3. **在 `handler/price.go` 中添加 CoinGecko 平台 ID** —— 找到 `coinGeckoPlatformIDs`，以启用该链上的代币定价。

4. **标准 EVM 链：** 除上述步骤外无需修改代码。

5. **Solana 系列链：** 需要修改 `chain/tx_sol.go` 中的交易构建逻辑。

> **注意：** 缺少 CoinGecko 映射时，价格查询会静默失败。由于无法获取 USD 价值，所有金额的转账都将需要审批（失败关闭行为）。
