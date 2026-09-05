package operations

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// pseudoRandomBytes zwraca deterministyczny strumień bajtów (xorshift32),
// dzięki czemu testy nie zależą od generatora liczb losowych.
func pseudoRandomBytes(n int, seed uint32) []byte {
	data := make([]byte, n)
	state := seed
	for i := range data {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		data[i] = byte(state)
	}
	return data
}

// equalBytes porównuje dwa strumienie bajtów bez użycia pakietu bytes.
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type progressCall struct {
	part  int
	bytes int64
}

// assertFileContent sprawdza rozmiar, zawartość i sumę CRC32 pliku.
func assertFileContent(t *testing.T, path string, original []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("nie można odczytać %s: %v", path, err)
	}
	if len(got) != len(original) {
		t.Fatalf("rozmiar %s: oczekiwano %d, otrzymano %d", path, len(original), len(got))
	}
	if !equalBytes(got, original) {
		t.Errorf("zawartość %s różni się od oryginału", path)
	}
	if want, gotCRC := crc32.ChecksumIEEE(original), crc32.ChecksumIEEE(got); gotCRC != want {
		t.Errorf("CRC32 %s: oczekiwano %08X, otrzymano %08X", path, want, gotCRC)
	}
}

func TestSplitFileAndJoinFiles(t *testing.T) {
	const (
		baseName  = "testdata.bin"
		fileSize  = 50 * 1024 // 50 KB
		chunkSize = 12 * 1024 // 12 KB
	)
	// 50 KB / 12 KB = 4 pełne części + 2048 bajtów => 5 części.
	const expectedParts = 5

	srcDir := t.TempDir()
	partsDir := t.TempDir()

	srcPath := filepath.Join(srcDir, baseName)
	original := pseudoRandomBytes(fileSize, 0x02F6E2B1)
	if err := os.WriteFile(srcPath, original, 0o644); err != nil {
		t.Fatalf("nie można utworzyć pliku źródłowego: %v", err)
	}

	// --- Dzielenie ---
	var splitProgress []progressCall
	parts, err := SplitFile(srcPath, partsDir, chunkSize, func(part int, bytesWritten int64) {
		splitProgress = append(splitProgress, progressCall{part, bytesWritten})
	})
	if err != nil {
		t.Fatalf("SplitFile: %v", err)
	}

	if len(parts) != expectedParts {
		t.Fatalf("oczekiwano %d części, otrzymano %d", expectedParts, len(parts))
	}
	for i, partPath := range parts {
		wantPath := filepath.Join(partsDir, fmt.Sprintf("%s.%03d", baseName, i+1))
		if partPath != wantPath {
			t.Errorf("część %d: oczekiwano ścieżki %q, otrzymano %q", i+1, wantPath, partPath)
		}
		info, err := os.Stat(partPath)
		if err != nil {
			t.Fatalf("brak części %s: %v", partPath, err)
		}
		wantSize := int64(chunkSize)
		if i == expectedParts-1 {
			wantSize = fileSize - (expectedParts-1)*chunkSize
		}
		if info.Size() != wantSize {
			t.Errorf("część %d: oczekiwano rozmiaru %d, otrzymano %d", i+1, wantSize, info.Size())
		}
	}
	if _, err := os.Stat(filepath.Join(partsDir, baseName+".006")); !os.IsNotExist(err) {
		t.Errorf("część .006 nie powinna istnieć (stat err = %v)", err)
	}

	// Postęp: jedno wywołanie na część, suma bajtów = rozmiar pliku.
	if len(splitProgress) != expectedParts {
		t.Fatalf("oczekiwano %d wywołań onProgress, otrzymano %d", expectedParts, len(splitProgress))
	}
	var splitSum int64
	for i, call := range splitProgress {
		if call.part != i+1 {
			t.Errorf("onProgress: oczekiwano numeru części %d, otrzymano %d", i+1, call.part)
		}
		splitSum += call.bytes
	}
	if splitSum != fileSize {
		t.Errorf("suma bajtów z onProgress: oczekiwano %d, otrzymano %d", fileSize, splitSum)
	}

	// --- Plik .crc ---
	crcPath := filepath.Join(partsDir, baseName+".crc")
	crcData, err := os.ReadFile(crcPath)
	if err != nil {
		t.Fatalf("brak pliku .crc: %v", err)
	}
	wantCRC := crc32.ChecksumIEEE(original)
	for _, want := range []string{
		"filename=" + baseName,
		"size=" + strconv.Itoa(fileSize),
		fmt.Sprintf("crc32=%08X", wantCRC),
	} {
		if !strings.Contains(string(crcData), want) {
			t.Errorf("plik .crc nie zawiera %q:\n%s", want, crcData)
		}
	}

	// --- Łączenie startując od .001 ---
	joinDir := t.TempDir()
	var joinProgress []progressCall
	joinedPath, err := JoinFiles(parts[0], joinDir, func(part int, bytesRead int64) {
		joinProgress = append(joinProgress, progressCall{part, bytesRead})
	})
	if err != nil {
		t.Fatalf("JoinFiles(.001): %v", err)
	}
	if want := filepath.Join(joinDir, baseName); joinedPath != want {
		t.Errorf("oczekiwano ścieżki wyniku %q, otrzymano %q", want, joinedPath)
	}
	assertFileContent(t, joinedPath, original)

	if len(joinProgress) != expectedParts {
		t.Errorf("oczekiwano %d wywołań onProgress (join), otrzymano %d", expectedParts, len(joinProgress))
	}
	var joinSum int64
	for i, call := range joinProgress {
		if call.part != i+1 {
			t.Errorf("onProgress (join): oczekiwano numeru części %d, otrzymano %d", i+1, call.part)
		}
		joinSum += call.bytes
	}
	if joinSum != fileSize {
		t.Errorf("suma bajtów z onProgress (join): oczekiwano %d, otrzymano %d", fileSize, joinSum)
	}

	// --- Łączenie startując od .crc ---
	joinedPath2, err := JoinFiles(crcPath, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("JoinFiles(.crc): %v", err)
	}
	assertFileContent(t, joinedPath2, original)
}

func TestJoinFilesDetectsCorruption(t *testing.T) {
	const (
		fileSize  = 40 * 1024
		chunkSize = 16 * 1024
	)
	srcDir := t.TempDir()
	partsDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "corruptme.bin")
	if err := os.WriteFile(srcPath, pseudoRandomBytes(fileSize, 0x00C0FFEE), 0o644); err != nil {
		t.Fatal(err)
	}
	parts, err := SplitFile(srcPath, partsDir, chunkSize, nil)
	if err != nil {
		t.Fatalf("SplitFile: %v", err)
	}

	// Uszkodzenie jednego bajtu w środkowej części.
	victim := parts[1]
	data, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	data[100] ^= 0xFF
	if err := os.WriteFile(victim, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = JoinFiles(parts[0], t.TempDir(), nil)
	if err == nil {
		t.Fatal("oczekiwano błędu po uszkodzeniu części")
	}
	if !strings.Contains(err.Error(), "CRC32") {
		t.Errorf("oczekiwano błędu CRC32, otrzymano: %v", err)
	}
}

func TestJoinFilesDetectsSizeMismatch(t *testing.T) {
	const (
		fileSize  = 40 * 1024
		chunkSize = 16 * 1024
	)
	srcDir := t.TempDir()
	partsDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "truncated.bin")
	if err := os.WriteFile(srcPath, pseudoRandomBytes(fileSize, 0x1BADB002), 0o644); err != nil {
		t.Fatal(err)
	}
	parts, err := SplitFile(srcPath, partsDir, chunkSize, nil)
	if err != nil {
		t.Fatalf("SplitFile: %v", err)
	}

	// Obcięcie ostatniej części o 10 bajtów.
	last := parts[len(parts)-1]
	data, err := os.ReadFile(last)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(last, data[:len(data)-10], 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = JoinFiles(parts[0], t.TempDir(), nil)
	if err == nil {
		t.Fatal("oczekiwano błędu po obcięciu części")
	}
	if !strings.Contains(err.Error(), "size mismatch") {
		t.Errorf("oczekiwano błędu rozmiaru, otrzymano: %v", err)
	}
}

func TestJoinFilesWithoutCRCFile(t *testing.T) {
	const (
		fileSize  = 30 * 1024
		chunkSize = 16 * 1024
	)
	srcDir := t.TempDir()
	partsDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "plain.bin")
	original := pseudoRandomBytes(fileSize, 0xDEADBEEF)
	if err := os.WriteFile(srcPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	parts, err := SplitFile(srcPath, partsDir, chunkSize, nil)
	if err != nil {
		t.Fatalf("SplitFile: %v", err)
	}

	// Bez pliku .crc łączenie nadal musi działać (bez weryfikacji).
	if err := os.Remove(filepath.Join(partsDir, "plain.bin.crc")); err != nil {
		t.Fatal(err)
	}
	joinedPath, err := JoinFiles(parts[0], t.TempDir(), nil)
	if err != nil {
		t.Fatalf("JoinFiles bez .crc: %v", err)
	}
	assertFileContent(t, joinedPath, original)
}

func TestSplitJoinEmptyFile(t *testing.T) {
	srcDir := t.TempDir()
	partsDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "empty.bin")
	if err := os.WriteFile(srcPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	parts, err := SplitFile(srcPath, partsDir, 1024, nil)
	if err != nil {
		t.Fatalf("SplitFile: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("oczekiwano 1 części dla pustego pliku, otrzymano %d", len(parts))
	}

	joinedPath, err := JoinFiles(parts[0], t.TempDir(), nil)
	if err != nil {
		t.Fatalf("JoinFiles: %v", err)
	}
	info, err := os.Stat(joinedPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Errorf("oczekiwano pustego pliku wynikowego, otrzymano rozmiar %d", info.Size())
	}
}

func TestSplitFileInvalidInput(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(srcPath, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SplitFile(srcPath, dir, 0, nil); err == nil {
		t.Error("oczekiwano błędu dla chunkSize = 0")
	}
	if _, err := SplitFile(srcPath, dir, -1, nil); err == nil {
		t.Error("oczekiwano błędu dla ujemnego chunkSize")
	}
	if _, err := SplitFile(filepath.Join(dir, "missing.bin"), dir, 1024, nil); err == nil {
		t.Error("oczekiwano błędu dla nieistniejącego pliku źródłowego")
	}
	if _, err := SplitFile(dir, dir, 1024, nil); err == nil {
		t.Error("oczekiwano błędu, gdy źródło jest katalogiem")
	}
}

func TestJoinFilesInvalidInput(t *testing.T) {
	dir := t.TempDir()

	if _, err := JoinFiles(filepath.Join(dir, "missing.001"), dir, nil); err == nil {
		t.Error("oczekiwano błędu dla brakującej części .001")
	}
	if _, err := JoinFiles(filepath.Join(dir, "notes.txt"), dir, nil); err == nil {
		t.Error("oczekiwano błędu dla nieobsługiwanego rozszerzenia")
	}
}

func TestChunkSizeConstants(t *testing.T) {
	want := map[string]int64{
		"ChunkSizeFloppy144": 1457664,
		"ChunkSizeZip100":    100431872,
		"ChunkSizeCD650":     681574400,
		"ChunkSizeCD700":     734003200,
		"ChunkSizeDVD47":     4700000000,
	}
	got := map[string]int64{
		"ChunkSizeFloppy144": ChunkSizeFloppy144,
		"ChunkSizeZip100":    ChunkSizeZip100,
		"ChunkSizeCD650":     ChunkSizeCD650,
		"ChunkSizeCD700":     ChunkSizeCD700,
		"ChunkSizeDVD47":     ChunkSizeDVD47,
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %d, oczekiwano %d", name, got[name], w)
		}
	}
}