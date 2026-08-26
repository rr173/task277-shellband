# BENZHI 评测说明

基于 Go 实现的贝壳同位素季节生长带对齐后端服务，一款后端服务，完成生长带标注、同位素位置校正、季节对齐与缺口诊断、重结晶污染裁决与不可变季节快照发布。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/shellband --addr :8080 --db shellband.db
```

## 自检（不启动长驻服务）

```bash
go run ./cmd/shellband --smoke-test
```

`--smoke-test` 会真实创建批次与生长带、导入同位素采样、做位置校正与对齐、完成污染诊断与裁决、发布季节快照，关闭并重新打开数据库验证持久化与重启恢复，最后以 0 退出码结束。

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/shellband --smoke-test
```

## HTTP API（前缀 /api）

批次：`POST /api/batches`、`GET /api/batches`、`GET /api/batches/{id}`、`PATCH /api/batches/{id}/species`、`POST /api/batches/{id}/status`、`POST /api/batches/{id}/seal`
生长带：`POST /api/batches/{id}/bands`、`GET /api/batches/{id}/bands`
采样：`POST /api/batches/{id}/samples`、`GET /api/batches/{id}/samples`
校正：`POST /api/batches/{id}/correct`、`GET /api/batches/{id}/corrections`
对齐：`POST /api/batches/{id}/align`、`GET /api/batches/{id}/alignments`
诊断：`POST /api/batches/{id}/diagnose`、`POST /api/batches/{id}/verdicts`、`GET /api/batches/{id}/verdicts`
锚点：`POST /api/batches/{id}/anchors`、`GET /api/batches/{id}/anchors`
快照：`POST /api/batches/{id}/snapshots`、`GET /api/batches/{id}/snapshots`、`GET /api/batches/{id}/snapshots/{sid}`、`POST /api/batches/{id}/publish`
自检：`GET /api/stats`、`GET /healthz`

## 持久化

SQLite（modernc.org/sqlite，CGO 无关）。建表：shell_batches、growth_bands、isotope_samples、age_anchors、position_corrections、alignments、pollution_verdicts、seasonal_snapshots。采样编号幂等；已发布快照关闭重开后仍可读取。
