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
3. When the file is fully received it is saved to the output directory and a
   desktop notification is shown.
4. If no cimbar code appears within the configured timeout (default 20 s) the
   program exits with an error.

## Prerequisites

### 1. libcimbar (`cimbar_recv`)

Build from source:

```bash
git clone --recurse-submodules https://github.com/sz3/libcimbar
cd libcimbar
mkdir build && cd build
cmake ..
make -j$(nproc) cimbar_recv
sudo cp src/exe/cimbar_recv /usr/local/bin/
```

### 2. System dependencies (Linux)

```bash
# X11 screen-capture libraries (usually already present on a desktop system)
sudo apt install libx11-dev libxrandr-dev

# Desktop notifications
sudo apt install libnotify-bin
```

## Build

```bash
git clone https://github.com/litertiger/pc_filesend_bycimbar
cd pc_filesend_bycimbar
go build -o cimbar-recv-pc .
```

## Usage

```
cimbar-recv-pc [flags]

Flags:
  -output  string    Output directory for received files (default ".")
  -timeout duration  Timeout if no cimbar code detected (default 20s)
  -fps     int       Screen capture frames per second (default 5)
  -display int       Display index to monitor (default 0)
  -cimbar  string    Path to cimbar_recv binary (default "cimbar_recv")
  -region  string    Capture region WxH+X+Y, or "auto" for full display (default "auto")
  -verbose           Show cimbar_recv output
  -keep              Keep captured frame images after decoding
```

### Examples

```bash
# Basic usage — full screen, output to current directory
./cimbar-recv-pc

# Save to Downloads, wait up to 30 s for code to appear
./cimbar-recv-pc -output ~/Downloads -timeout 30s

# Monitor only the right half of a 1920×1080 display (faster detection)
./cimbar-recv-pc -region 960x1080+960+0

# Use a custom cimbar_recv path and show its output
./cimbar-recv-pc -cimbar /opt/libcimbar/build/cimbar_recv -verbose
```

## Detection logic

The anchor detector looks for three nested dark→light→dark square regions in
the top-left, top-right, and bottom-left corners of a candidate code area, plus
a brightly-coloured (high-HSV-saturation) interior — matching the cimbar
standard layout.  It requires no native cimbar library at detection time.

## Decoding architecture

Frames are saved as `frame_NNNNN.png` and passed to `cimbar_recv` as an OpenCV
image sequence (`frame_%05d.png`).  `cimbar_recv` uses fountain (raptor-like)
decoding, so it reconstructs the file once a sufficient number of *unique*
frames have been captured.  Decoding is attempted every 5 seconds of captured
footage and retried until the transfer completes or a hard timeout is reached.

## Licence

MPL-2.0 (mirrors libcimbar's licence).
