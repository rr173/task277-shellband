# shellband — 贝壳同位素季节生长带对齐服务

面向古气候研究者的贝壳同位素科研后端服务。导入贝壳切片样本与生长带标注、按生长轴导入同位素采样，执行位置校正与生长带周期对齐（含缺口诊断），诊断重结晶污染并裁决取舍，最终发布不可变的季节生长序列快照。

## 业务闭环

1. 录入贝壳切片批次与生长带（band_index / 季节 / 起止位置）。
2. 录入年代锚点（position → age_year），供对齐参考。
3. 按生长轴导入同位素采样（raw_pos / isotope_value / unit / recrystall_score）。
4. 位置校正：得到单调非降的 `corrected_pos`，破坏单调性则拒绝。
5. 生长带对齐：将采样点对齐到所属生长带，标记缺少采样的生长带为「缺口」。
6. 污染诊断：依重结晶分数裁决采样点保留 / 排除。
7. 发布季节生长序列快照（不可变），供季节气候重建复用。

## 状态机

- 切片批次：`接收中` → `待对齐` → `需复核` → `已发布` → `封存`
- 生长带 / 采样 / 对齐：随批次状态机推进
- 季节快照：`草稿` → `已发布`

## 标准命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/shellband --smoke-test
go run ./cmd/shellband --addr :8080 --db shellband.db
```

## 目录结构

仓库根目录就是源码根（本地 `env/` 原样推送，远程不再套一层 `env/`）：

```
cmd/shellband/            # 入口（--smoke-test 契约）
internal/model/           # 实体、错误、状态机
internal/store/           # SQLite 持久化与迁移
internal/slice/           # 切片与生长带
internal/sample/          # 同位素采样与位置校正
internal/align/           # 生长带周期对齐与缺口诊断
internal/diagnose/        # 重结晶污染诊断与裁决
internal/snapshot/        # 季节快照构建
internal/service/         # 编排层
internal/httpapi/         # /api HTTP 层
go.mod / go.sum
component-versions.json
Dockerfile / benzhi.Dockerfile / build_benzhi_docker.sh
BENZHI_README.md
```

详见 `BENZHI_README.md`。
