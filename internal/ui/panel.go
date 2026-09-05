package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"myc/internal/vfs"
)

// Panel represents one file pane (left or right).
type Panel struct {
	ID        string
	VFS       vfs.VFS
	Entries   []*vfs.FileEntry
	Cursor    int
	TopOffset int
	Active    bool
}

// NewPanel creates a new panel rooted at the given path.
func NewPanel(id string, initialPath string) (*Panel, error) {
	localVfs, err := vfs.NewLocalVFS(initialPath)
	if err != nil {
		return nil, err
	}
	p := &Panel{
		ID:     id,
		VFS:    localVfs,
		Active: false,
	}
	if err := p.Refresh(); err != nil {
		return nil, err
	}
	return p, nil
}

// Refresh reloads the directory listing.
func (p *Panel) Refresh() error {
	entries, err := p.VFS.List()
	if err != nil {
		return err
	}
	p.Entries = entries
	if p.Cursor >= len(p.Entries) {
		p.Cursor = len(p.Entries) - 1
	}
	if p.Cursor < 0 {
		p.Cursor = 0
	}
	return nil
}

// CurrentEntry returns the file entry currently under cursor.
func (p *Panel) CurrentEntry() *vfs.FileEntry {
	if len(p.Entries) == 0 || p.Cursor < 0 || p.Cursor >= len(p.Entries) {
		return nil
	}
	return p.Entries[p.Cursor]
}

// MoveUp moves cursor up by 1.
func (p *Panel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
		if p.Cursor < p.TopOffset {
			p.TopOffset = p.Cursor
		}
	}
}

// MoveDown moves cursor down by 1.
func (p *Panel) MoveDown(visibleHeight int) {
	if p.Cursor < len(p.Entries)-1 {
		p.Cursor++
		if p.Cursor >= p.TopOffset+visibleHeight {
			p.TopOffset = p.Cursor - visibleHeight + 1
		}
	}
}

// PageUp moves cursor up by one page.
func (p *Panel) PageUp(visibleHeight int) {
	p.Cursor -= visibleHeight
	if p.Cursor < 0 {
		p.Cursor = 0
	}
	p.TopOffset -= visibleHeight
	if p.TopOffset < 0 {
		p.TopOffset = 0
	}
}

// PageDown moves cursor down by one page.
func (p *Panel) PageDown(visibleHeight int) {
	p.Cursor += visibleHeight
	if p.Cursor >= len(p.Entries) {
		p.Cursor = len(p.Entries) - 1
	}
	if p.Cursor < 0 {
		p.Cursor = 0
	}
	p.TopOffset += visibleHeight
	if p.TopOffset > len(p.Entries)-visibleHeight {
		p.TopOffset = len(p.Entries) - visibleHeight
	}
	if p.TopOffset < 0 {
		p.TopOffset = 0
	}
}

// Home moves cursor to the first item.
func (p *Panel) Home() {
	p.Cursor = 0
	p.TopOffset = 0
}

// End moves cursor to the last item.
func (p *Panel) End(visibleHeight int) {
	p.Cursor = len(p.Entries) - 1
	if p.Cursor < 0 {
		p.Cursor = 0
	}
	p.TopOffset = len(p.Entries) - visibleHeight
	if p.TopOffset < 0 {
		p.TopOffset = 0
	}
}

// ToggleSelect toggles selection on the current item and moves cursor down.
func (p *Panel) ToggleSelect(visibleHeight int) {
	entry := p.CurrentEntry()
	if entry != nil && entry.Name != ".." {
		entry.Selected = !entry.Selected
		p.MoveDown(visibleHeight)
	}
}

// SelectAll marks all regular files/dirs as selected.
func (p *Panel) SelectAll() {
	for _, e := range p.Entries {
		if e.Name != ".." {
			e.Selected = true
		}
	}
}

// UnselectAll clears all selections.
func (p *Panel) UnselectAll() {
	for _, e := range p.Entries {
		e.Selected = false
	}
}

// GetSelectedOrCurrent returns all selected entries, or if none selected, the current one.
func (p *Panel) GetSelectedOrCurrent() []*vfs.FileEntry {
	var selected []*vfs.FileEntry
	for _, e := range p.Entries {
		if e.Selected && e.Name != ".." {
			selected = append(selected, e)
		}
	}
	if len(selected) > 0 {
		return selected
	}
	curr := p.CurrentEntry()
	if curr != nil && curr.Name != ".." {
		return []*vfs.FileEntry{curr}
	}
	return nil
}

// SelectionStats returns count and total bytes of selected entries.
func (p *Panel) SelectionStats() (int, int64) {
	var count int
	var totalBytes int64
	for _, e := range p.Entries {
		if e.Selected && e.Name != ".." {
			count++
			if !e.IsDir {
				totalBytes += e.Size
			}
		}
	}
	return count, totalBytes
}

// Enter attempts to navigate into directory or archive. Returns true if navigated.
func (p *Panel) Enter() (bool, error) {
	curr := p.CurrentEntry()
	if curr == nil {
		return false, nil
	}

	if curr.Name == ".." {
		// Go up
		if p.VFS.IsArchive() && p.VFS.Path() == filepath.Dir(p.VFS.Path()) {
			// Leave archive back to local directory
			oldPath := p.VFS.Path()
			p.VFS.Close()
			localPath := filepath.Dir(strings.Split(oldPath, "::/")[0])
			localVfs, err := vfs.NewLocalVFS(localPath)
			if err != nil {
				return false, err
			}
			p.VFS = localVfs
		} else {
			if err := p.VFS.Parent(); err != nil {
				return false, err
			}
		}
		p.Cursor = 0
		p.TopOffset = 0
		return true, p.Refresh()
	}

	if curr.IsDir {
		if err := p.VFS.SetPath(filepath.Join(p.VFS.Path(), curr.Name)); err != nil {
			return false, err
		}
		p.Cursor = 0
		p.TopOffset = 0
		return true, p.Refresh()
	}

	if curr.IsArchive && strings.HasSuffix(strings.ToLower(curr.Name), ".zip") {
		// Mount zip as virtual directory
		zipVfs, err := vfs.NewZipVFS(curr.Path)
		if err != nil {
			return false, err
		}
		p.VFS = zipVfs
		p.Cursor = 0
		p.TopOffset = 0
		return true, p.Refresh()
	}

	return false, nil
}

// FormatSize returns human-readable file size.
func FormatSize(bytes int64, isDir bool) string {
	if isDir {
		return "<KATALOG>"
	}
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
}
