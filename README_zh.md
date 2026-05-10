# cimbar PC 接收工具

用 Go 编写的 PC 端 cimbar 接收程序。它监控本地屏幕上显示的
[cimbar](https://github.com/sz3/libcimbar)（彩色图标矩阵条码）动画
（例如 [cimbar.org](https://cimbar.org) 在浏览器中播放的动态码），
自动解码并保存传输的文件。

## 工作原理

1. 在浏览器中打开 **https://cimbar.org**，加载要传输的文件，等待动态条码开始播放。
2. 运行本程序。程序持续截取屏幕，通过检测 cimbar 特有的角标和彩色数据格，
   判断条码是否出现，并将截帧送入 `cimbar_recv` 进行喷泉码解码。
3. 文件接收完成后，自动保存到输出目录（默认 `%USERPROFILE%\Downloads`），
   并弹出 Windows 10 系统托盘气泡通知。
4. 若在超时时间内（默认 20 秒）未检测到 cimbar 条码，程序报错退出。

---

## 依赖项

### 1. libcimbar（`cimbar_recv.exe`）— Windows 10

**前置要求**：Visual Studio 2019 或更高版本、CMake ≥ 3.14、Git。

```bat
git clone --recurse-submodules https://github.com/sz3/libcimbar
cd libcimbar
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build --config Release --target cimbar_recv
:: 将生成的 exe 复制到 PATH 中的某个目录，例如：
copy build\Release\cimbar_recv.exe C:\Windows\System32\
```

> 也可在 [libcimbar Releases](https://github.com/sz3/libcimbar/releases) 页面
> 查找是否有预编译的 Windows 二进制包。

### 2. Go 工具链（用于编译本工具）

从 https://go.dev/dl/ 下载并安装，安装程序会自动将 `go` 加入 PATH。

---

## 编译

```bat
git clone https://github.com/litertiger/pc_filesend_bycimbar
cd pc_filesend_bycimbar
go build -o cimbar-recv-pc.exe .
```

---

## 使用方法

```
cimbar-recv-pc.exe [参数]

参数说明：
  -output   string    文件保存目录（默认：%USERPROFILE%\Downloads）
  -timeout  duration  等待条码出现的超时时间（默认 20s）
  -min-fps  float     等待阶段最低截屏帧率（默认 2，省 CPU）
  -max-fps  float     捕获阶段最高截屏帧率（默认 30）
  -display  int       要监控的显示器序号（默认 0，即主屏）
  -cimbar   string    cimbar_recv 可执行文件路径（默认从 PATH 查找）
  -region   string    截取区域，格式 宽x高+左+上，或 "auto"（默认 auto）
  -verbose            显示 cimbar_recv 的详细输出
  -keep               解码完成后保留截帧图片
  -log      string    日志文件路径，同时写入文件和终端（默认仅终端）
  -loglevel string    日志级别：debug | info | warn | error（默认 info）
```

### 常用示例

```bat
:: 最简用法：全屏监控，文件保存到 Downloads
cimbar-recv-pc.exe

:: 保存到桌面，超时时间改为 30 秒，同时写日志文件
cimbar-recv-pc.exe -output %USERPROFILE%\Desktop -timeout 30s -log cimbar.log

:: 只监控 1920×1080 屏幕的右半部分（减少截图开销）
cimbar-recv-pc.exe -region 960x1080+960+0

:: 限制帧率范围并开启 debug 日志
cimbar-recv-pc.exe -min-fps 3 -max-fps 20 -loglevel debug

:: 指定 cimbar_recv 路径并显示解码过程
cimbar-recv-pc.exe -cimbar C:\tools\cimbar_recv.exe -verbose
```

### 在命令提示符 / PowerShell 中运行

```bat
:: cmd.exe
cimbar-recv-pc.exe -output %USERPROFILE%\Downloads

:: PowerShell
.\cimbar-recv-pc.exe -output $env:USERPROFILE\Downloads
```

---

## 自适应截屏帧率

程序会自动追踪 cimbar 动画的播放速度，动态调整截屏间隔：

| 阶段 | 截屏帧率 |
|---|---|
| 等待条码出现 | `min-fps`（默认 2 fps，节省 CPU） |
| 刚检测到条码 | 立即跳至 5 fps |
| 稳定捕获阶段 | 动态 = 动画实测帧率 × 1.5，上限为 `max-fps` |

**算法说明**：
- 对每帧截图计算 **8×8 感知哈希**，与上一帧对比：哈希不同即判定为"新帧"。
- 保留最近 40 个新帧的时间戳，通过时间跨度反推动画实际帧率。
- 每 20 次截图重新评估一次（指数移动平均，平滑过渡）。
- **只保存新帧**到临时目录，重复帧直接丢弃，避免向 `cimbar_recv` 输入冗余数据。

---

## 日志说明

所有事件默认输出到终端（stderr）。通过 `-log` 参数可同时写入文件，
适合排查问题。`-loglevel debug` 会记录每次截图和帧率调整的详细信息。

日志格式示例：

```
2025-05-10 12:34:56.789 [INFO ] === cimbar PC Receiver ===
2025-05-10 12:34:57.012 [INFO ] [OK] cimbar code detected on screen
2025-05-10 12:35:01.100 [INFO ] adaptive: FPS 5.0 → 14.2  (new 18/20 = 90%, anim ≈ 9.5 fps)
2025-05-10 12:35:30.000 [INFO ] starting decode attempt (unique frames: 25)
2025-05-10 12:36:05.500 [INFO ] [OK] file received: C:\Users\...\Downloads\data.zip
```

---

## 条码检测原理

检测器在截图的左上、右上、左下三个角寻找嵌套的"深色→浅色→深色"方块
（即 cimbar 标准锚点图案），同时验证中央区域存在高饱和度彩色像素
（即数据格）。检测过程不依赖 cimbar 原生库。

---

## 解码架构

截帧以 `frame_NNNNN.png` 格式保存在系统临时目录，
以 OpenCV 图像序列模式（`frame_%05d.png`）传给 `cimbar_recv`。
`cimbar_recv` 使用喷泉码（类 Raptor 码）解码，
积累足够多的唯一帧后即可还原完整文件。
程序在采集到 25 帧唯一帧后发起首次解码尝试，失败则继续采集并重试，
直至成功或达到硬超时（60 秒无进展）。

---

## 许可证

MPL-2.0（与 libcimbar 保持一致）。
