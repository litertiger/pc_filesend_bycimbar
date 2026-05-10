# cimbar PC Receiver

A Go program that monitors your local screen for [cimbar](https://github.com/sz3/libcimbar)
(Color Icon Matrix Barcode) animations — such as those displayed by
[cimbar.org](https://cimbar.org) — and automatically decodes and saves the
transmitted file.

## How it works

1. Open **https://cimbar.org** in your browser, load a file, and let the
   barcode animation start.
2. Run this program.  It continuously captures the screen, detects the cimbar
   code by looking for its distinctive corner anchors and coloured data tiles,
   and feeds the captured frames to `cimbar_recv` for fountain-code decoding.
3. When the file is fully received it is saved to the output directory (default:
   `%USERPROFILE%\Downloads`) and a Windows 10 balloon notification is shown.
4. If no cimbar code appears within the configured timeout (default 20 s) the
   program exits with an error.

## Prerequisites

### 1. libcimbar (`cimbar_recv.exe`) — Windows 10

Requirements: **Visual Studio 2019 or later**, **CMake ≥ 3.14**, **Git**.

```bat
git clone --recurse-submodules https://github.com/sz3/libcimbar
cd libcimbar
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build --config Release --target cimbar_recv
:: Copy to somewhere on PATH, e.g.:
copy build\Release\cimbar_recv.exe C:\Windows\System32\
```

> Alternatively, check the [libcimbar Releases](https://github.com/sz3/libcimbar/releases)
> page for pre-built Windows binaries.

### 2. Go toolchain (to build this tool)

Download from https://go.dev/dl/ and install (adds `go` to PATH automatically).

## Build

```bat
git clone https://github.com/litertiger/pc_filesend_bycimbar
cd pc_filesend_bycimbar
go build -o cimbar-recv-pc.exe .
```

## Usage

```
cimbar-recv-pc.exe [flags]

Flags:
  -output  string    Output directory for received files
                     (default: %USERPROFILE%\Downloads)
  -timeout duration  Timeout if no cimbar code detected (default 20s)
  -fps     int       Screen capture frames per second (default 5)
  -display int       Display index to monitor (default 0)
  -cimbar  string    Path to cimbar_recv executable (default "cimbar_recv")
  -region  string    Capture region WxH+X+Y, or "auto" for full display (default "auto")
  -verbose           Show cimbar_recv output
  -keep              Keep captured frame images after decoding
```

### Examples

```bat
:: Basic usage — full screen, output to Downloads
cimbar-recv-pc.exe

:: Save to Desktop, wait up to 30 s for the code to appear
cimbar-recv-pc.exe -output %USERPROFILE%\Desktop -timeout 30s

:: Monitor only the right half of a 1920x1080 display (faster)
cimbar-recv-pc.exe -region 960x1080+960+0

:: Custom cimbar_recv path, verbose output
cimbar-recv-pc.exe -cimbar C:\tools\cimbar_recv.exe -verbose
```

### Running from Command Prompt or PowerShell

```bat
:: cmd.exe
cimbar-recv-pc.exe -output %USERPROFILE%\Downloads

:: PowerShell
.\cimbar-recv-pc.exe -output $env:USERPROFILE\Downloads
```

## Notifications

On Windows 10 a system-tray balloon notification is shown on success or error.
The notification is sent via PowerShell (`System.Windows.Forms.NotifyIcon`) and
does not require any additional packages.

## Detection logic

The anchor detector looks for three nested dark→light→dark square regions in
the top-left, top-right, and bottom-left corners of a candidate code area, plus
a brightly-coloured (high HSV-saturation) interior — matching the cimbar
standard layout.  It requires no native cimbar library at detection time.

## Decoding architecture

Frames are saved as `frame_NNNNN.png` in a temporary folder and passed to
`cimbar_recv` as an OpenCV image sequence (`frame_%05d.png`).  `cimbar_recv`
uses fountain (raptor-like) decoding and reconstructs the file once enough
unique frames have been captured.  Decoding is attempted every 5 seconds of
captured footage and retried until the transfer completes or a hard timeout is
reached.

## Licence

MPL-2.0 (mirrors libcimbar's licence).
