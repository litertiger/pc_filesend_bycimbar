package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DecoderResult is the outcome of a cimbar_recv run.
type DecoderResult struct {
	OutputFile string
	Err        error
}

// runDecoder launches cimbar_recv with the given image-sequence frame pattern
// and the specified output directory. It streams stdout/stderr to the console
// and sends the result on resultCh when done.
//
// framePattern should be an OpenCV image-sequence printf pattern, e.g.
//
//	"/tmp/cimbar_frames/frame_%05d.png"
//
// cimbar_recv treats this as an image sequence (OpenCV VideoCapture supports it).
func runDecoder(cimbarBin, framePattern, outputDir string, verbose bool, resultCh chan<- DecoderResult) {
	args := []string{
		"--output", outputDir,
		framePattern,
	}
	cmd := exec.Command(cimbarBin, args...)
	cmd.Dir = outputDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		resultCh <- DecoderResult{Err: fmt.Errorf("stdout pipe: %w", err)}
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		resultCh <- DecoderResult{Err: fmt.Errorf("stderr pipe: %w", err)}
		return
	}

	if err := cmd.Start(); err != nil {
		resultCh <- DecoderResult{Err: fmt.Errorf("failed to start %s: %w\n\nInstall libcimbar from https://github.com/sz3/libcimbar", cimbarBin, err)}
		return
	}

	outFile := ""
	doneCh := make(chan struct{})

	// Read stdout: look for the output filename.
	go func() {
		defer close(doneCh)
		scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
		for scanner.Scan() {
			line := scanner.Text()
			if verbose {
				fmt.Println("[cimbar]", line)
			}
			// cimbar_recv prints something like "decoded: filename" or just the path.
			if f := extractOutputFile(line, outputDir); f != "" {
				outFile = f
			}
		}
	}()

	err = cmd.Wait()
	<-doneCh

	if err != nil {
		resultCh <- DecoderResult{Err: fmt.Errorf("cimbar_recv: %w", err)}
		return
	}

	// If stdout didn't give us a filename, scan the outputDir for recently-created files.
	if outFile == "" {
		outFile = newestFile(outputDir, time.Now().Add(-30*time.Second))
	}

	resultCh <- DecoderResult{OutputFile: outFile}
}

// extractOutputFile attempts to parse a decoded file path from a cimbar_recv output line.
func extractOutputFile(line, outputDir string) string {
	line = strings.TrimSpace(line)
	// Common patterns: "decoded filename", "output: filename", or bare filename.
	for _, prefix := range []string{"decoded ", "output: ", "wrote ", "saved "} {
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			candidate := strings.TrimSpace(line[len(prefix):])
			return resolveOutputPath(candidate, outputDir)
		}
	}
	// If the whole line looks like a file path that exists.
	if _, err := os.Stat(line); err == nil {
		return line
	}
	candidate := filepath.Join(outputDir, line)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

func resolveOutputPath(name, dir string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(dir, name)
}

// newestFile returns the path of the newest regular file in dir modified after since.
func newestFile(dir string, since time.Time) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	newest := ""
	var newestTime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(since) && info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newest = filepath.Join(dir, e.Name())
		}
	}
	return newest
}

// checkCimbarBin returns an error if cimbarBin cannot be found.
func checkCimbarBin(bin string) error {
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf(
			"cannot find %q\n\n"+
				"Build libcimbar on Windows (requires Visual Studio 2019+ and CMake):\n"+
				"  git clone --recurse-submodules https://github.com/sz3/libcimbar\n"+
				"  cd libcimbar\n"+
				"  cmake -B build -DCMAKE_BUILD_TYPE=Release\n"+
				"  cmake --build build --config Release --target cimbar_recv\n"+
				"  copy build\\Release\\cimbar_recv.exe C:\\Windows\\System32\\\n\n"+
				"Then add the directory containing cimbar_recv.exe to PATH,\n"+
				"or use: -cimbar C:\\path\\to\\cimbar_recv.exe", bin)
	}
	return nil
}
