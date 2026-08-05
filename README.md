# soundprobe

`soundprobe` 是一款教育网优先的跨平台网络路径测量工具，支持 macOS、Linux
和 Windows。它不把不同
线路压成一个“网络分数”，而是按顺序回答几个清楚的问题：南大校园网或
校园 VPN 是否通、公网出口表现如何、负载下网络是否仍有响应，以及附近的
运营商侧参考服务器表现如何。

产品名、仓库名、Homebrew Formula 和可执行文件均保持小写
`soundprobe`；`SoundProbe` 只用于必要的人类可读标题。

## 测速视角

| 目标 ID | 默认 | 它回答的问题 | 方法与结果边界 |
| --- | --- | --- | --- |
| `nju-campus` | 是 | 南大校内服务、校园网或 NJU VPN 是否连通 | LibreSpeed 三流；IPv4/IPv6 可分别选择 |
| `mlab` | 是 | 当前公网/代理出口的 bulk-transport 表现 | M-Lab NDT7 单流；节点由 Locate 动态返回 |
| `apple` | macOS 默认 | macOS 内置的公网吞吐与负载下响应性 | `/usr/bin/networkQuality -c -s`；显示吞吐、基础 RTT 与 RPM；Linux/Windows 自动跳过 |
| `ookla` | 否 | 附近 Ookla 测试服务器的运营商侧参考 | 仅官方 Speedtest CLI；动态服务器、ID、赞助方和地址写入结果 |
| `tongji` | 否 | 上海及江浙沪方向的教育网参考 | Tongji LibreSpeed 三流，IPv4 |
| `qlu` | 否 | 山东方向的教育网参考 | QLU LibreSpeed 三流，IPv4；受路由和服务器负载影响 |
| `cernet` | 否 | CERNET 公共站点兼容性 | 当前服务不可达，保留显式诊断，不进入日常设置 |

NJU Edge (`http://test.nju.edu.cn`、`http://test6.nju.edu.cn`) 和中科大网页
测速是浏览器产品，不加入 CLI 测速；soundprobe 不绕过浏览器验证。教育网
三流是三个并发 HTTP 请求，不是三条物理线路，也不表示服务器带宽异常大。
NDT7 是单流 bulk-transport；两种方法的数值不应直接横向排名。

### 为什么默认是 NJU + M-Lab + Apple

macOS 首次设置和非交互 `soundprobe run` 的默认目标为：

```text
nju-campus → mlab → apple
```

NJU Campus 是产品主线；M-Lab 提供公网/境外出口参考，但不承诺一定返回
境外节点，界面和 JSON 始终显示实际返回的节点；Apple `networkQuality` 是
macOS 自带、具有明确负载响应性指标的第三视角。Linux/Windows 没有 Apple
helper 时会自动移除该可选目标，仍可正常执行 NJU Campus + M-Lab。Ookla 不自动加入：只有在
用户于 `soundprobe setup` 中主动选择并且官方 helper 可用时才运行。

所有选中的目标按显示顺序串行执行，避免互相争抢带宽。失败目标记录零速率
和稳定的失败阶段；取消时未开始的目标记录 `null` 速率和 `skipped`，不会把
失败伪装成成功。

## 安装

### soundprobe

```sh
brew tap soundadam/tap
brew install soundprobe
soundprobe doctor --json
```

Homebrew Formula 当前只覆盖 macOS；Linux/Windows 可使用对应 release 二进制，
或从仓库构建核心 CLI。构建时不需要 Apple SDK：

```sh
GOOS=linux   GOARCH=amd64 go build -o soundprobe ./cmd/soundprobe
GOOS=windows GOARCH=amd64 go build -o soundprobe.exe ./cmd/soundprobe
```

两种平台上的基础运行仍需要 soundprobe 发布的 LibreSpeed/ndt7 helper；缺少
helper 时 `doctor` 会明确报告，而不是下载未知程序。

### Apple networkQuality

macOS 自带 `/usr/bin/networkQuality`，soundprobe 只调用它，不下载、不捆绑，
也不替用户接受 Apple 或第三方条款。Apple 对 NetworkQuality 和负载下响应性
的说明见 [WWDC21 官方视频](https://developer.apple.com/videos/play/wwdc2021/10239/)。

### Ookla Speedtest CLI（可选）

只支持 Ookla 官方 CLI；soundprobe 不维护 Python `speedtest-cli`，也不把 Ookla
协议实现降级到自己的 CLI 中。安装方式和许可请以 [Ookla 官方 CLI 页面](https://www.speedtest.net/apps/cli)
为准。官方 Homebrew 命令为：

```sh
brew tap teamookla/speedtest
brew update
brew install speedtest --force
speedtest --version
```

也可以从官方页面下载 macOS universal archive。soundprobe 不会自动安装、升级
或随自身发布 Ookla helper，也不会自动传入 `--accept-license` 或
`--accept-gdpr`。第一次运行若需要接受条款，请用户直接按 Ookla 的提示操作。

Homebrew 中名为 `speedtest-cli` 的 Python 工具不是官方 Ookla CLI，且上游已
停止维护；soundprobe 会检查 `speedtest --version` 的官方标识并拒绝它。它会
检查 PATH 中所有同名候选，而不是盲目采用第一个；也可用
`SOUNDPROBE_OOKLA_PATH=/path/to/speedtest` 指定官方二进制。不自动卸载冲突程序，
请用户自行决定 PATH 中哪个可执行文件应被调用：

```sh
command -v speedtest
speedtest --version
soundprobe doctor --json
```

当用户明确运行 `soundprobe ookla`，且交互终端检测到上述冲突时，soundprobe
会显示官方 Homebrew 修复序列，并等待用户按 Enter：

```text
brew tap teamookla/speedtest
brew update
brew install speedtest --force
```

只有按 Enter 才会逐条执行这些固定参数；输入其他内容会取消。组合运行、重定向
和 `--json` 不会暂停安装。soundprobe 不会自动卸载已有 formula；如果安装报告
冲突，请确认确实要删除后再手动执行：

```sh
brew uninstall speedtest --force
brew uninstall speedtest-cli --force
brew install speedtest --force
```

没有 Homebrew 时只提供官方页面，不会尝试下载未知程序。这样 Ookla 与中科大等
网页测速一样，都是可选的外部参考，不是 soundprobe 必须维护的核心链路。

缺少 Apple（非 macOS）或 Ookla helper 不影响组合运行：soundprobe 会在测速前
移除不可用的可选目标，并继续 NJU Campus/M-Lab 基础诊断。`doctor` 会在
`optionalProviders` 中单独报告它们。显式执行 `soundprobe apple` 或
`soundprobe ookla` 时，helper 不可用会在测速前失败，不会静默换成 Python 工具。

核心二进制不依赖 macOS API；Apple 只是 macOS 可选 provider，Ookla 官方 CLI
在三个系统上都可作为可选 provider。Linux 使用 `$XDG_CONFIG_HOME`（默认
`~/.config`），Windows 使用用户的 Roaming 配置目录，macOS 使用
`~/Library/Application Support`；历史和同意记录均按同一平台目录保存。

## 教育网候选服务器验证记录

Ookla 服务器目录是动态的，不是 soundprobe 的固定目标。2026-08-05 查询到
的候选包括：

- Duke Kunshan University，Kunshan，server ID `30852`，主机
  `speedtest.dukekunshan.edu.cn`；当时 A 记录 `180.208.59.230` 属于
  AS4538 CERNET，可参考 [AS4538 信息](https://bgp.tools/as/4538)。
- Shanghai China Unicom 5G（server ID `24447`）。
- Suzhou JSQY（server ID `16204`）。

这些条目只用于说明动态目录中确实存在教育网或运营商侧候选，不固定加入
默认目标，也不建立 CERNET provider。Ookla 的 `sponsor` 表示测速服务器
运营方，不等于当前用户的接入运营商；soundprobe 不新增 ASN、地理定位或
客户端运营商识别服务。

## 首次设置与日常使用

在 TTY 中首次运行 `soundprobe` 会选择中文或 English，再选择日常测速站。
macOS 默认预选 `nju-campus`、`mlab`、`apple`；Linux/Windows 默认预选
`nju-campus`、`mlab`。以后可用：

```sh
soundprobe setup
soundprobe
```

选择器只做轻量 DNS/连接探测，不提前执行带宽测试。快捷键：

```text
↑/↓ 或 j/k   移动
Space        选择/取消
4            IPv4
6            IPv6
d            dual（仅有地址族的站点展开）
a            恢复 NJU + M-Lab + Apple 推荐
Enter        按顺序开始
q / Esc      取消
```

Ookla 只有在设置中被用户主动勾选后才会进入日常集合；它不会因 PATH 中碰巧
存在某个 `speedtest` 就自动加入。

## CLI

```sh
soundprobe
soundprobe run [--targets LIST] [--family ipv4|ipv6|dual] [--label TEXT] [--note TEXT] [--no-save]
soundprobe campus [--ipv4|--ipv6] [--label TEXT] [--note TEXT] [--no-save]
soundprobe mlab [--label TEXT] [--note TEXT] [--no-save]
soundprobe apple [--label TEXT] [--note TEXT] [--no-save]
soundprobe ookla [--label TEXT] [--note TEXT] [--no-save]
soundprobe domestic [--targets tongji,qlu] [--family ipv4] [--no-save]
soundprobe stations [--json]
soundprobe doctor [--json]
soundprobe history [--limit N]
soundprobe last [--json]
soundprobe show RUN_ID [--json]
soundprobe export --format jsonl|csv --output PATH
soundprobe consent status|accept|revoke
soundprobe version
```

脚本或重定向模式不会打开选择器；全局 `--json` 输出一个 JSON 文档，且不含
ANSI 转义。示例：

```sh
soundprobe run --targets nju-campus,mlab,apple --family ipv4 --no-save --json
soundprobe ookla --no-save --json
soundprobe doctor --json
```

`apple` provider 使用 `networkQuality` 的 `base_rtt`、`dl_throughput`、
`ul_throughput`、`responsiveness`、`dl_responsiveness`、
`ul_responsiveness` 等字段；`ookla` provider 记录 `serverId`、
`serverSponsor`、服务器 host/IP、延迟、抖动、上下行速率和 helper 版本。
自动 provider 只执行一次，不按 IPv4/IPv6 扩展两次；有 helper 返回的地址族
时记录实际 `ipFamily`。

## M-Lab 隐私

M-Lab 测试前必须明确接受当前隐私政策。M-Lab 会公开并无限期保留测试结果
和 ISP 提供的公网 IP；政策地址为
<https://www.measurementlab.net/privacy/>。交互式接受：

```sh
soundprobe consent accept
soundprobe consent status
```

没有当前同意记录时，非交互命令 fail closed，且在接触 M-Lab 前返回
`consent_required`。没有选择 M-Lab 的计划不需要这一步。

## JSON、历史和本地文件

每次结果都包含按执行顺序排列的 `targets` 和 `measurements`。新增的可选字段
包括：

```json
{
  "serverId": 30852,
  "serverSponsor": "Duke Kunshan University",
  "responsivenessRpm": 400.375,
  "uploadResponsivenessRpm": 380.25,
  "downloadResponsivenessRpm": 420.5
}
```

历史永久保存在当前用户配置目录：

```text
macOS:  ~/Library/Application Support/soundprobe/history/v1/<run-id>.json
Linux:  ${XDG_CONFIG_HOME:-~/.config}/soundprobe/history/v1/<run-id>.json
Windows: %AppData%\\soundprobe\\history\\v1\\<run-id>.json
```

目录权限 `0700`、文件权限 `0600`，使用同目录临时文件、fsync 和原子 rename，
不保存逐秒样本。旧的 schema-v1 历史仍可读取。导出：

如果系统中仍有早期 `njuprobe` Application Support 历史或 M-Lab consent，
soundprobe 会在迁移完成前继续读取它们；新写入使用 `soundprobe` 目录。

```sh
soundprobe history --limit 20
soundprobe last --json
soundprobe export --format jsonl --output /tmp/soundprobe.jsonl
soundprobe export --format csv --output /tmp/soundprobe.csv
```

CSV 一行对应一个 measurement，并包含 server ID、sponsor 和三类 RPM 字段。

## 本地开发与验证

```sh
make test-offline
make test-race
make build
./bin/soundprobe version
git diff --check
```

`make test-offline`、Formula 测试和 release automation 只使用 fixtures/mock
helper，不执行真实 Apple、Ookla、NJU 或 M-Lab 带宽测试。真实测速应由对应
平台的操作者单独验收，不应放入常规 CI。可验证跨平台编译：

```sh
GOOS=darwin  GOARCH=arm64  go build -o bin/soundprobe-darwin-arm64 ./cmd/soundprobe
GOOS=linux   GOARCH=amd64  go build -o bin/soundprobe-linux-amd64 ./cmd/soundprobe
GOOS=windows GOARCH=amd64  go build -o bin/soundprobe-windows-amd64.exe ./cmd/soundprobe
```

Makefile 目标拆分在：

```text
make/common.mk  make/build.mk  make/test.mk  make/release.mk  make/clean.mk
```

LibreSpeed CLI 和 `ndt7-client` 仍作为独立 helper 执行；LibreSpeed 保持
LGPL-3.0-only，ndt7-client 保持 Apache-2.0，soundprobe 本身为 MIT。详见
[SPEC.md](SPEC.md)、[TESTING.md](TESTING.md)、[RELEASE.md](RELEASE.md) 和
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。soundprobe 不读取或修改
soundVPN、SFM、NJUConnect 的私有配置。
