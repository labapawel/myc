package operations

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// CompareResult describes the outcome of comparing two files.
type CompareResult struct {
	Identical       bool
	SizeA           int64
	SizeB           int64
	DiffOffset      int64
	DiffLine        int64
	Message         string
}

// CompareFiles compares two files by content.
func CompareFiles(pathA, pathB string) (*CompareResult, error) {
	statA, err := os.Stat(pathA)
	if err != nil {
		return nil, fmt.Errorf("błąd odczytu pliku A: %w", err)
	}
	statB, err := os.Stat(pathB)
	if err != nil {
		return nil, fmt.Errorf("błąd odczytu pliku B: %w", err)
	}

	result := &CompareResult{
		SizeA: statA.Size(),
		SizeB: statB.Size(),
	}

	if statA.Size() != statB.Size() {
		result.Identical = false
		result.Message = fmt.Sprintf("Pliki różnią się rozmiarem (%d B vs %d B)", statA.Size(), statB.Size())
		return result, nil
	}

	fileA, err := os.Open(pathA)
	if err != nil {
		return nil, err
	}
	defer fileA.Close()

	fileB, err := os.Open(pathB)
	if err != nil {
		return nil, err
	}
	defer fileB.Close()

	bufA := make([]byte, 32*1024)
	bufB := make([]byte, 32*1024)
	var offset int64
	var line int64 = 1

	for {
		nA, errA := fileA.Read(bufA)
		nB, errB := fileB.Read(bufB)

		if nA != nB || !bytes.Equal(bufA[:nA], bufB[:nB]) {
			// Find exact differing byte and line
			for i := 0; i < nA && i < nB; i++ {
				if bufA[i] == '\n' {
					line++
				}
				if bufA[i] != bufB[i] {
					result.Identical = false
					result.DiffOffset = offset + int64(i)
					result.DiffLine = line
					result.Message = fmt.Sprintf("Pierwsza różnica w bajcie %d (linia %d)", result.DiffOffset, result.DiffLine)
					return result, nil
				}
			}
			result.Identical = false
			result.DiffOffset = offset + int64(nA)
			result.Message = fmt.Sprintf("Różnica w długości bloku na pozycji %d", result.DiffOffset)
			return result, nil
		}

		// Count newlines in matching block
		for i := 0; i < nA; i++ {
			if bufA[i] == '\n' {
				line++
			}
		}

		offset += int64(nA)

		if errA == io.EOF && errB == io.EOF {
			break
		}
		if errA != nil && errA != io.EOF {
			return nil, errA
		}
		if errB != nil && errB != io.EOF {
			return nil, errB
		}
	}

	result.Identical = true
	result.Message = "Zawartość obu plików jest identyczna"
	return result, nil
}
